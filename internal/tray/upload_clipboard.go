package tray

import (
	"fmt"
	"strings"
	"time"
)

func (m *TrayManager) UploadFromClipboard() {
	content := GetClipboard()
	if content == "" {
		m.ShowErrorNotification("Your clipboard is empty or does not contain any text.")
		return
	}

	reader := strings.NewReader(content)
	filename := fmt.Sprintf("clipboard (%s).txt", time.Now().Format("2006-01-02 at 15.04.05"))

	m.PerformUpload(reader, filename, false)
}
