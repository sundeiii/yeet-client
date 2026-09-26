package tray

import (
	"errors"
	"io"
	"log"
	"strings"

	"fyne.io/fyne/v2"

	"github.com/sundeiii/yeet-client/internal/i18n"
	"github.com/sundeiii/yeet-client/pkg/puush"
)

// CopyScreenText captures an area of the screen and copies the text in it.
// The server reads the text and doesn't keep the picture; nothing is
// uploaded.
func (m *TrayManager) CopyScreenText() {
	if !m.api.Account.Credentials.HasApiKey() {
		m.OnUploadError(puush.PuushErrorInvalidCredentials)
		return
	}
	provider := m.GetScreenshotProvider()
	if provider == nil {
		m.ShowErrorNotification(i18n.T("No screenshot provider available. Please install a compatible screenshot tool to use this feature!"))
		return
	}

	reader, err := provider.CaptureArea()
	if err != nil {
		if !isCancelledError(err) {
			m.ShowErrorNotification(i18n.T("An error occurred while capturing the screenshot. Please try again."))
		}
		log.Printf("Error capturing the area to read: %v", err)
		return
	}
	data, err := io.ReadAll(reader)
	reader.Close()
	if err != nil {
		m.ShowErrorNotification(i18n.T("An error occurred while capturing the screenshot. Please try again."))
		return
	}

	m.OnTrayProgressUpdate(50, i18n.T("puush: reading the text..."))
	text, err := m.api.ReadText(data)
	fyne.Do(m.ResetTrayIcon)
	switch {
	case errors.Is(err, puush.ErrNotSupported):
		m.ShowErrorNotification(i18n.T("This server can't read text in pictures yet."))
	case err != nil:
		m.ShowErrorNotification(puush.FormatError(err))
	case strings.TrimSpace(text) == "":
		m.ShowNotification(i18n.T("No text found"), i18n.T("puush couldn't find any text in that area."))
	default:
		fyne.Do(func() { fyne.CurrentApp().Clipboard().SetContent(text) })
		m.ShowNotification(i18n.T("Text copied"), textPreview(text, 150))
	}
}

// textPreview is the start of a text, on one line.
func textPreview(text string, length int) string {
	text = strings.Join(strings.Fields(text), " ")
	if runes := []rune(text); len(runes) > length {
		return string(runes[:length-1]) + "…"
	}
	return text
}
