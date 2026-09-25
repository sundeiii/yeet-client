package tray

import (
	"io"
	"log"
	"net/url"
	"path/filepath"

	"fyne.io/fyne/v2"
	"github.com/sundeiii/yeet-client/pkg/puush"
)

func (m *TrayManager) PerformUpload(reader io.Reader, filename string, preserveClipboard bool) {
	if !m.api.Account.Credentials.HasApiKey() {
		return
	}
	log.Println("Starting upload:", filename)

	urlResponse, err := m.api.Upload(reader, filename)
	if err != nil {
		m.OnUploadError(err)
		return
	}
	m.OnUploadComplete(urlResponse, preserveClipboard)
	// TODO: Implement upload retries
}

func (m *TrayManager) PerformFileUpload(path string) {
	pr, err := puush.NewProgressReaderFromFile(path, m.OnTrayProgressUpdate)
	if err != nil {
		m.OnUploadError(err)
		return
	}
	defer pr.Close()

	filename := filepath.Base(path)
	m.PerformUpload(pr, filename, false)
}

func (m *TrayManager) PerformScreenshotUpload(reader io.ReadSeekCloser, filename string) {
	// Preserve the clipboard if the raw image was already saved to it
	preserveClipboard := m.config.Capture.SaveImagesToClipboard

	// For screenshots we only have a reader available by default
	// We want to try our best to still show a progress bar for the upload

	total, err := seekableReaderSize(reader)
	if err == nil {
		// If we can determine the size of the reader, we can
		// show the progress bar during the upload
		pr := puush.NewProgressReader(reader, total, m.OnTrayProgressUpdate)
		defer pr.Close()
		m.PerformUpload(pr, filename, preserveClipboard)
		return
	}

	// If we can't determine the size of the reader, we can still perform
	// the upload, but we won't be able to show the progress bar
	log.Printf("Unable to determine upload size for %s: %v", filename, err)

	if seekErr := seekToStart(reader); seekErr != nil {
		reader.Close()
		m.OnUploadError(seekErr)
		return
	}
	defer reader.Close()
	m.PerformUpload(reader, filename, preserveClipboard)
}

func (m *TrayManager) OnUploadComplete(urlResponse string, preserveClipboard bool) {
	log.Println("Upload complete:", urlResponse)

	// Set updated disk usage to config
	m.config.Account.Usage = m.api.Account.DiskUsage

	// Update the tray icon to the "complete" state
	m.OnTrayProgressComplete()
	m.ShowUploadNotification(urlResponse)

	if m.config.General.CopyToClipboard && !preserveClipboard {
		fyne.CurrentApp().Clipboard().SetContent(urlResponse)
	}
	if m.config.General.OpenBrowser {
		if u, err := url.Parse(urlResponse); err == nil {
			fyne.CurrentApp().OpenURL(u)
		}
	}

	// Refresh the history to reflect the new upload
	go m.RefreshHistory()
}

func (m *TrayManager) OnUploadError(err error) {
	log.Println("Upload error:", err)

	// Update the tray icon to the "failed" state
	m.OnTrayProgressFail()
	m.ShowErrorNotification(puush.FormatError(err))
}

func seekableReaderSize(reader io.Seeker) (int64, error) {
	total, err := seekToEnd(reader)
	if err != nil {
		return 0, err
	}
	if err = seekToStart(reader); err != nil {
		return 0, err
	}
	return total, nil
}

func seekToStart(reader io.Seeker) error {
	_, err := reader.Seek(0, io.SeekStart)
	return err
}

func seekToEnd(reader io.Seeker) (int64, error) {
	total, err := reader.Seek(0, io.SeekEnd)
	if err != nil {
		return 0, err
	}
	return total, nil
}
