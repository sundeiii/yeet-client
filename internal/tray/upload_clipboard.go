package tray

import (
	"context"
	"errors"
	"fmt"
	"github.com/sundeiii/yeet-client/internal/i18n"
	"log"
	"os"
	"time"

	"golang.design/x/clipboard"
)

// UploadFromClipboard uploads whatever was copied last: files (e.g. copied in
// Explorer) are uploaded as themselves, images (e.g. copied from a browser or
// Discord) as a PNG, and text as a .txt file.
func (m *TrayManager) UploadFromClipboard() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	timestamp := time.Now().Format("2006-01-02 at 15.04.05")

	if paths := clipboardFiles(ctx); len(paths) > 0 {
		if err := m.EnqueueFiles(paths); err != nil {
			m.OnUploadError(err)
		}
		return
	}

	if image, err := clipboard.Read(ctx, clipboard.FmtImage); err == nil && len(image) > 0 {
		m.enqueueJob(&uploadJob{Name: fmt.Sprintf("clipboard (%s).png", timestamp), Data: image})
		return
	}

	content := GetClipboard()
	if content == "" {
		m.ShowErrorNotification(i18n.T("Your clipboard is empty, or holds something puush can't upload."))
		return
	}
	m.enqueueJob(&uploadJob{Name: fmt.Sprintf("clipboard (%s).txt", timestamp), Data: []byte(content)})
}

// clipboardFiles returns the copied files that exist on this computer.
// Folders are skipped; puush uploads files.
func clipboardFiles(ctx context.Context) []string {
	paths, err := clipboard.ReadFiles(ctx)
	if err != nil {
		if !errors.Is(err, clipboard.ErrNoData) {
			log.Printf("Could not read files from the clipboard: %v", err)
		}
		return nil
	}

	var files []string
	for _, path := range paths {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			files = append(files, path)
		}
	}
	return files
}
