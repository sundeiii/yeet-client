package tray

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"github.com/sundeiii/yeet-client/internal/i18n"
	"github.com/sundeiii/yeet-client/internal/screenshots"
)

// Screen recordings: the same shortcut (or the tray menu) starts one after
// picking an area, and stops it. The file is uploaded like a screenshot.

type recordingState struct {
	mu        sync.Mutex
	current   *screenshots.Recording
	selecting bool // the area is being picked
}

// Recording reports whether the screen is being recorded.
func (m *TrayManager) Recording() bool {
	m.recording.mu.Lock()
	defer m.recording.mu.Unlock()
	return m.recording.current != nil
}

// RecordingSupported reports whether this computer can record the screen.
func (m *TrayManager) RecordingSupported() bool {
	return screenshots.RecordingSupported()
}

// ToggleRecording starts recording an area of the screen, or stops the
// recording that's running.
func (m *TrayManager) ToggleRecording() {
	m.recording.mu.Lock()
	if current := m.recording.current; current != nil {
		m.recording.mu.Unlock()
		// The goroutine waiting for it uploads the file
		go current.Stop()
		return
	}
	if m.recording.selecting {
		m.recording.mu.Unlock()
		return
	}
	m.recording.selecting = true
	m.recording.mu.Unlock()
	defer func() {
		m.recording.mu.Lock()
		m.recording.selecting = false
		m.recording.mu.Unlock()
	}()

	if !screenshots.RecordingSupported() {
		m.ShowErrorNotification(i18n.T("Recording the screen only works on Windows for now."))
		return
	}

	window := screenshots.ActiveWindowTitle()
	area, err := screenshots.SelectRecordingArea()
	if err != nil {
		if !isCancelledError(err) {
			log.Printf("Error picking the area to record: %v", err)
		}
		return
	}

	format := screenshots.RecordingFormat(m.config.Capture.RecordingFormat)
	if format != screenshots.RecordingGIF {
		format = screenshots.RecordingMP4
	}
	path := filepath.Join(os.TempDir(), fmt.Sprintf("yeet-recording-%d.%s", time.Now().UnixNano(), format))
	recording, err := screenshots.StartRecording(area, format, path)
	if err != nil {
		log.Printf("Error starting a recording: %v", err)
		m.ShowErrorNotification(i18n.T("The recording couldn't start: %s", err.Error()))
		return
	}
	name := ScreenshotName(m.config.Capture.NamePattern, window, time.Now()) + "." + string(format)

	m.recording.mu.Lock()
	m.recording.current = recording
	m.recording.mu.Unlock()
	m.RebuildMenu()
	go m.showRecordingTime(recording)
	go m.finishRecording(recording, name)
}

// showRecordingTime keeps the tray's tooltip up to date while recording.
func (m *TrayManager) showRecordingTime(recording *screenshots.Recording) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		elapsed := recording.Elapsed().Round(time.Second)
		text := i18n.T("puush: recording %s", fmt.Sprintf("%d:%02d", int(elapsed.Minutes()), int(elapsed.Seconds())%60))
		if shortcut := m.config.Hotkeys.Record; shortcut != "" {
			text += "\n" + i18n.T("%s stops it", shortcut)
		}
		fyne.Do(func() { setTrayTooltip(text) })
		select {
		case <-recording.Done():
			return
		case <-ticker.C:
		}
	}
}

// finishRecording waits for the recording to end and uploads it.
func (m *TrayManager) finishRecording(recording *screenshots.Recording, name string) {
	<-recording.Done()
	m.recording.mu.Lock()
	m.recording.current = nil
	m.recording.mu.Unlock()
	m.RebuildMenu()
	fyne.Do(m.ResetTrayIcon)
	defer os.Remove(recording.Path)

	if err := recording.Err(); err != nil {
		log.Printf("Recording failed: %v", err)
		m.ShowErrorNotification(i18n.T("The recording failed: %s", err.Error()))
		return
	}
	data, err := os.ReadFile(recording.Path)
	if err != nil || len(data) == 0 {
		if err == nil {
			err = errors.New("the file is empty")
		}
		m.ShowErrorNotification(i18n.T("The recording failed: %s", err.Error()))
		return
	}

	localCopy := ""
	if m.config.Capture.SaveImages {
		localCopy = m.SaveScreenshotToDisk(data, name, m.config.Capture.ImageSavePath())
	}
	m.enqueueJob(&uploadJob{Name: name, Data: data, LocalCopy: localCopy})
}
