package tray

import (
	"github.com/sqweek/dialog"
	"github.com/sundeiii/yeet-client/internal/i18n"
)

func (m *TrayManager) UploadFileFromDialog() {
	filename, err := dialog.File().Title(i18n.T("Select a file to upload")).Load()
	if err != nil {
		return
	}
	if err := m.EnqueueFile(filename); err != nil {
		m.OnUploadError(err)
	}
}
