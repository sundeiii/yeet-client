package tray

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

func (m *TrayManager) UploadAreaScreenshot() {
	provider := m.GetScreenshotProvider()
	if provider == nil {
		m.ShowErrorNotification("No screenshot provider available. Please install a compatible screenshot tool to use this feature!")
		return
	}

	reader, err := provider.CaptureArea()
	if err != nil {
		if !isCancelledError(err) {
			m.ShowErrorNotification("An error occurred while capturing the screenshot. Please try again.")
		}
		log.Printf("Error capturing area screenshot: %v", err)
		return
	}

	filename := getImageFilename(reader)
	m.OnScreenshotCaptured(reader, filename)
	m.PerformScreenshotUpload(reader, filename)
}

func (m *TrayManager) UploadDesktopScreenshot() {
	provider := m.GetScreenshotProvider()
	if provider == nil {
		m.ShowErrorNotification("No screenshot provider available. Please install a compatible screenshot tool to use this feature!")
		return
	}

	reader, err := provider.CaptureScreen()
	if err != nil {
		if !isCancelledError(err) {
			m.ShowErrorNotification("An error occurred while capturing the screenshot. Please try again.")
		}
		log.Printf("Error capturing area screenshot: %v", err)
		return
	}

	filename := getImageFilename(reader)
	m.OnScreenshotCaptured(reader, filename)
	m.PerformScreenshotUpload(reader, filename)
}

func (m *TrayManager) UploadWindowScreenshot() {
	provider := m.GetScreenshotProvider()
	if provider == nil {
		m.ShowErrorNotification("No screenshot provider available. Please install a compatible screenshot tool to use this feature!")
		return
	}

	reader, err := provider.CaptureWindow()
	if err != nil {
		if !isCancelledError(err) {
			m.ShowErrorNotification("An error occurred while capturing the screenshot. Please try again.")
		}
		log.Printf("Error capturing area screenshot: %v", err)
		return
	}

	filename := getImageFilename(reader)
	m.OnScreenshotCaptured(reader, filename)
	m.PerformScreenshotUpload(reader, filename)
}

func (m *TrayManager) OnScreenshotCaptured(reader io.ReadSeeker, filename string) {
	if m.config.Capture.SaveImagesToClipboard {
		reader.Seek(0, io.SeekStart)
		m.CopyScreenshotToClipboard(reader)
	}
	if m.config.Capture.SaveImages && m.config.Capture.SaveImagePath != "" {
		reader.Seek(0, io.SeekStart)
		m.SaveScreenshotToDisk(reader, filename, m.config.Capture.SaveImagePath)
	}
	reader.Seek(0, io.SeekStart)
}

func (m *TrayManager) CopyScreenshotToClipboard(reader io.ReadSeeker) {
	err := SetClipboard(reader)
	if err != nil {
		log.Printf("Error setting clipboard: %v", err)
	} else {
		log.Printf("Screenshot copied to clipboard")
	}
}

func (m *TrayManager) SaveScreenshotToDisk(reader io.ReadSeeker, filename string, path string) string {
	if !strings.HasSuffix(path, "/") {
		path += "/"
	}

	outputPath := path + filename
	outFile, err := os.Create(outputPath)
	if err != nil {
		log.Printf("Error creating file for saving screenshot: %v", err)
		return ""
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, reader)
	if err != nil {
		log.Printf("Error saving screenshot to file: %v", err)
		return ""
	}

	log.Printf("Screenshot saved to: %s", outputPath)
	return outputPath
}

func getImageFilename(reader io.ReadSeeker) string {
	ext := getImageExtension(reader)
	return fmt.Sprintf("ss (%s)%s", time.Now().Format("2006-01-02 at 15.04.05"), ext)
}

func getImageExtension(reader io.ReadSeeker) string {
	buffer := make([]byte, 512)
	n, _ := reader.Read(buffer)
	reader.Seek(0, io.SeekStart)

	contentType := http.DetectContentType(buffer[:n])
	switch contentType {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
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
