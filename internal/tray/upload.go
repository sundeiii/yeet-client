package tray

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/sundeiii/yeet-client/internal/i18n"
	"io"
	"io/fs"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"github.com/sundeiii/yeet-client/pkg/puush"
)

// uploadJob is one file to upload. Files from disk keep their path;
// screenshots and clipboard contents are held in memory, so a failed upload
// can be tried again without asking for the file again.
type uploadJob struct {
	Name string
	Path string
	Data []byte

	// LocalCopy is the file on this computer the upload came from, if any,
	// for "Show in Folder" in the recent uploads menu
	LocalCopy string

	// Pending is this job's copy in the pending folder after a failed upload
	Pending string

	// PreserveClipboard: the screenshot itself was put on the clipboard, so
	// the link shouldn't replace it
	PreserveClipboard bool

	Attempts int
}

func newFileJob(path string) *uploadJob {
	return &uploadJob{Name: filepath.Base(path), Path: path, LocalCopy: path}
}

func (job *uploadJob) open() (io.ReadCloser, int64, error) {
	if job.Data != nil {
		return io.NopCloser(bytes.NewReader(job.Data)), int64(len(job.Data)), nil
	}
	file, err := os.Open(job.Path)
	if err != nil {
		return nil, 0, err
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, 0, err
	}
	return file, info.Size(), nil
}

// Automatic retries for connection problems, before the upload is kept for
// retrying by hand
var retryDelays = []time.Duration{5 * time.Second, 20 * time.Second}

// runUpload uploads one job, showing its progress in the tray. The upload
// can be cancelled from the tray menu while it runs.
func (m *TrayManager) runUpload(job *uploadJob) (string, error) {
	if !m.api.Account.Credentials.HasApiKey() {
		return "", puush.PuushErrorInvalidCredentials
	}

	reader, size, err := job.open()
	if err != nil {
		return "", err
	}
	defer reader.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.setActiveUpload(job, cancel)
	defer m.setActiveUpload(nil, nil)

	job.Attempts++
	log.Printf("Starting upload: %s (attempt %d)", job.Name, job.Attempts)
	m.track(job, func(entry *QueueEntry) {
		entry.Status = QueueUploading
		entry.Size = size
		entry.Progress = 0
	})

	progress := puush.NewProgressReader(reader, size, m.progressReporter(job, size))
	options := puush.UploadOptions{PoolId: m.config.General.UploadPoolId, Size: size}
	return m.api.UploadWithOptions(ctx, progress, job.Name, options)
}

// progressReporter shows the upload's progress in the tray icon and tooltip,
// only redrawing when the whole percentage changes.
func (m *TrayManager) progressReporter(job *uploadJob, size int64) func(float64) {
	last := -1
	return func(percentage float64) {
		if int(percentage) == last {
			return
		}
		last = int(percentage)
		m.OnTrayProgressUpdate(percentage, i18n.T("puush: uploading %s (%d%% of %s)", job.Name, last, humanSize(size)))
		m.track(job, func(entry *QueueEntry) { entry.Progress = percentage })
	}
}

// ActiveUpload returns the name of the file being uploaded right now, if any.
func (m *TrayManager) ActiveUpload() (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.activeUpload == nil {
		return "", false
	}
	return m.activeUpload.Name, true
}

// CancelUpload stops the upload that's running right now.
func (m *TrayManager) CancelUpload() {
	m.mu.Lock()
	cancel := m.cancelUpload
	m.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (m *TrayManager) setActiveUpload(job *uploadJob, cancel context.CancelFunc) {
	m.mu.Lock()
	m.activeUpload = job
	m.cancelUpload = cancel
	m.mu.Unlock()
	// Shows or hides "Cancel Upload" in the menu
	m.stateChanged()
}

// processBatch uploads jobs one after another. Links of a batch with several
// files are copied together at the end.
func (m *TrayManager) processBatch(jobs []*uploadJob) {
	var done []*uploadJob
	var links []string

	for _, job := range jobs {
		link, err := m.runUpload(job)
		switch {
		case err == nil:
			log.Println("Upload complete:", link)
			m.OnTrayProgressComplete()
			m.forgetFailed(job)
			m.config.Capture.RememberLocalCopy(link, job.LocalCopy)
			m.track(job, func(entry *QueueEntry) {
				entry.Status = QueueDone
				entry.Link = link
				entry.Progress = 100
			})
			done = append(done, job)
			links = append(links, link)
		case errors.Is(err, context.Canceled):
			log.Println("Upload cancelled:", job.Name)
			fyne.Do(m.ResetTrayIcon)
			m.track(job, func(entry *QueueEntry) { entry.Status = QueueCancelled })
			m.ShowNotification(i18n.T("Upload cancelled"), job.Name)
		default:
			m.onUploadFailed(job, err)
		}
	}

	if len(links) == 0 {
		return
	}
	m.config.Account.Usage = m.api.Account.DiskUsage

	if len(links) == 1 {
		m.onLinkReady(links[0], done[0].PreserveClipboard, previewIcon(done[0]))
	} else {
		m.onLinksReady(links)
	}

	// Refresh the history to reflect the new uploads
	go m.RefreshHistory()
}

func (m *TrayManager) onLinkReady(link string, preserveClipboard bool, preview []byte) {
	m.ShowUploadNotification(link, preview)

	if m.config.General.CopyToClipboard && !preserveClipboard {
		fyne.Do(func() { fyne.CurrentApp().Clipboard().SetContent(link) })
	}
	if m.config.General.OpenBrowser {
		if u, err := url.Parse(link); err == nil {
			fyne.Do(func() { fyne.CurrentApp().OpenURL(u) })
		}
	}
}

// onLinksReady finishes a batch of several files: all links are copied at
// once, one per line. (Opening them all in the browser would be a lot of tabs.)
func (m *TrayManager) onLinksReady(links []string) {
	// One album link is easier to share than a pile of links
	if m.config.General.Albums {
		album, err := m.api.CreateAlbum(links, "")
		if err == nil {
			m.onLinkReady(album, false, nil)
			return
		}
		log.Printf("Could not make an album: %v", err)
	}

	all := strings.Join(links, "\n")
	message := i18n.T("%d files puushed!", len(links))
	if m.config.General.CopyToClipboard {
		fyne.Do(func() { fyne.CurrentApp().Clipboard().SetContent(all) })
		message = i18n.T("%d files puushed! All links were copied.", len(links))
	}
	m.ShowNotification(message, all)
}

// onUploadFailed retries uploads that failed because of the connection, and
// keeps the rest (or the ones that keep failing) for retrying by hand.
func (m *TrayManager) onUploadFailed(job *uploadJob, err error) {
	log.Printf("Upload of %s failed: %v", job.Name, err)
	m.OnTrayProgressFail()
	m.track(job, func(entry *QueueEntry) {
		entry.Status = QueueFailed
		entry.Error = puush.FormatError(err)
	})

	if errors.Is(err, fs.ErrNotExist) {
		m.ShowErrorNotification(i18n.T("%s could not be uploaded because it no longer exists.", job.Name))
		m.forgetFailed(job)
		return
	}
	if errors.Is(err, puush.PuushErrorUploadTooLarge) {
		// Trying again won't make it smaller
		m.ShowErrorNotification(puush.FormatError(err))
		m.forgetFailed(job)
		return
	}

	if puush.ShouldRetryError(err) && job.Attempts <= len(retryDelays) {
		delay := retryDelays[job.Attempts-1]
		if job.Attempts == 1 {
			m.ShowErrorNotification(puush.FormatError(err) + " " + i18n.T("puush will try again in a moment."))
		}
		time.AfterFunc(delay, func() {
			if err := m.enqueue(uploadBatch{jobs: []*uploadJob{job}}); err != nil {
				m.keepFailed(job)
			}
		})
		return
	}

	m.keepFailed(job)
	m.ShowErrorNotification(puush.FormatError(err) + " " + i18n.T("The upload was kept, so you can retry it from the tray menu."))
}

// OnUploadError reports an error that happened before an upload could start.
func (m *TrayManager) OnUploadError(err error) {
	log.Println("Upload error:", err)

	// Update the tray icon to the "failed" state
	m.OnTrayProgressFail()
	m.ShowErrorNotification(puush.FormatError(err))
}

func humanSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}
