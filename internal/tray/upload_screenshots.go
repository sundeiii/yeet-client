package tray

import (
	"bytes"
	"fmt"
	"image"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"github.com/sundeiii/yeet-client/internal/screenshots"
)

type captureKind int

const (
	captureArea captureKind = iota
	captureDesktop
	captureWindow
	captureLastArea
)

func (m *TrayManager) UploadAreaScreenshot()    { m.captureAndUpload(captureArea) }
func (m *TrayManager) UploadDesktopScreenshot() { m.captureAndUpload(captureDesktop) }
func (m *TrayManager) UploadWindowScreenshot()  { m.captureAndUpload(captureWindow) }

// UploadLastAreaScreenshot captures the area that was picked last time again.
func (m *TrayManager) UploadLastAreaScreenshot() { m.captureAndUpload(captureLastArea) }

// Delayed captures count down first, e.g. to open a menu before the screenshot
func (m *TrayManager) DelayedAreaScreenshot()    { m.delayedCapture(captureArea) }
func (m *TrayManager) DelayedDesktopScreenshot() { m.delayedCapture(captureDesktop) }
func (m *TrayManager) DelayedWindowScreenshot()  { m.delayedCapture(captureWindow) }

// HasLastArea reports whether "Capture Last Area" has an area to capture.
func (m *TrayManager) HasLastArea() bool {
	_, ok := m.lastArea()
	return ok
}

func (m *TrayManager) delayedCapture(kind captureKind) {
	if !m.countingDown.CompareAndSwap(false, true) {
		// One countdown at a time
		return
	}
	defer m.countingDown.Store(false)

	// The tray icon counts down; a notification would end up in the screenshot
	delay := m.config.Capture.Delay()
	for remaining := delay; remaining > 0; remaining -= time.Second {
		m.OnTrayProgressUpdate(100*remaining.Seconds()/delay.Seconds(), fmt.Sprintf("puush: capturing in %d...", int(remaining.Seconds())))
		time.Sleep(time.Second)
	}
	fyne.Do(m.ResetTrayIcon)

	m.captureAndUpload(kind)
}

func (m *TrayManager) captureAndUpload(kind captureKind) {
	provider := m.GetScreenshotProvider()
	if provider == nil {
		m.ShowErrorNotification("No screenshot provider available. Please install a compatible screenshot tool to use this feature!")
		return
	}

	var reader io.ReadSeekCloser
	var err error

	switch kind {
	case captureArea:
		reader, err = provider.CaptureArea()
		if err == nil {
			m.rememberLastArea(provider)
		}
	case captureDesktop:
		reader, err = provider.CaptureScreen()
	case captureWindow:
		reader, err = provider.CaptureWindow()
	case captureLastArea:
		regions, supported := provider.(screenshots.RegionCapturer)
		area, known := m.lastArea()
		switch {
		case !supported:
			m.ShowErrorNotification(fmt.Sprintf("Capturing the last area again doesn't work with the %s screenshot provider.", provider.Name()))
			return
		case !known:
			m.ShowNotification("No area yet", "Capture an area first. After that, this captures the same area again.")
			return
		}
		reader, err = regions.CaptureRegion(area)
	}

	if err != nil {
		if !isCancelledError(err) {
			m.ShowErrorNotification("An error occurred while capturing the screenshot. Please try again.")
		}
		log.Printf("Error capturing screenshot: %v", err)
		return
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		m.ShowErrorNotification("An error occurred while capturing the screenshot. Please try again.")
		return
	}

	filename := getImageFilename(data)
	localCopy := m.OnScreenshotCaptured(data, filename)
	m.enqueueJob(&uploadJob{
		Name:              filename,
		Data:              data,
		LocalCopy:         localCopy,
		PreserveClipboard: m.config.Capture.SaveImagesToClipboard,
	})
}

func (m *TrayManager) rememberLastArea(provider screenshots.ScreenshotProvider) {
	regions, ok := provider.(screenshots.RegionCapturer)
	if !ok {
		return
	}
	if area, ok := regions.LastArea(); ok {
		m.config.Capture.LastArea = []int{area.Min.X, area.Min.Y, area.Dx(), area.Dy()}
	}
}

func (m *TrayManager) lastArea() (image.Rectangle, bool) {
	saved := m.config.Capture.LastArea
	if len(saved) != 4 || saved[2] <= 0 || saved[3] <= 0 {
		return image.Rectangle{}, false
	}
	return image.Rect(saved[0], saved[1], saved[0]+saved[2], saved[1]+saved[3]), true
}

// OnScreenshotCaptured puts the screenshot on the clipboard and saves a local
// copy, depending on the settings. It returns where the copy was saved.
func (m *TrayManager) OnScreenshotCaptured(data []byte, filename string) string {
	if m.config.Capture.SaveImagesToClipboard {
		if err := SetClipboard(bytes.NewReader(data)); err != nil {
			log.Printf("Error setting clipboard: %v", err)
		}
	}
	if m.config.Capture.SaveImages {
		return m.SaveScreenshotToDisk(data, filename, m.config.Capture.ImageSavePath())
	}
	return ""
}

func (m *TrayManager) SaveScreenshotToDisk(data []byte, filename string, path string) string {
	if path == "" {
		log.Printf("No folder to save screenshots to")
		return ""
	}
	if err := os.MkdirAll(path, 0755); err != nil {
		log.Printf("Error creating screenshot folder %s: %v", path, err)
		return ""
	}

	outputPath := filepath.Join(path, filename)
	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		log.Printf("Error saving screenshot to file: %v", err)
		return ""
	}

	log.Printf("Screenshot saved to: %s", outputPath)
	return outputPath
}

func getImageFilename(data []byte) string {
	return fmt.Sprintf("ss (%s)%s", time.Now().Format("2006-01-02 at 15.04.05"), getImageExtension(data))
}

func getImageExtension(data []byte) string {
	switch http.DetectContentType(data) {
	case "image/jpeg":
		return ".jpg"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	default:
		return ".png"
	}
}

func isCancelledError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "cancelled")
}
