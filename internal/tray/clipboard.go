package tray

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os/exec"
	"strings"

	"fyne.io/fyne/v2"
	"golang.design/x/clipboard"
)

func GetClipboard() string {
	content := fyne.CurrentApp().Clipboard().Content()
	if content != "" {
		return content
	}

	// Fallback to wl-paste for wayland systems
	cmd := exec.Command("wl-paste")
	out, err := cmd.Output()
	if err == nil && len(out) > 0 {
		return string(out)
	}

	// No clipboard for u :(
	return ""
}

func SetClipboard(reader io.Reader) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	mimeType := http.DetectContentType(data)

	// Try wl-copy first for wayland systems
	cmd := exec.Command("wl-copy", "-t", mimeType)
	cmd.Stdin = bytes.NewReader(data)
	err = cmd.Run()
	if err == nil {
		// Successfully copied using wl-copy
		return nil
	}

	// Fallback to golang-design/clipboard
	err = clipboard.Init()
	if err != nil {
		return err
	}

	format := clipboard.FmtText
	if strings.HasPrefix(mimeType, "image/") {
		format = clipboard.FmtImage
	}

	_, err = clipboard.Write(context.Background(), format, data)
	return err
}
