package tray

import (
	"github.com/sundeiii/yeet-client/internal/i18n"
	"time"

	"github.com/sundeiii/yeet-client/assets"
	"github.com/sundeiii/yeet-client/internal/notifications"
	"github.com/sundeiii/yeet-client/pkg/puush"
)

// PerformBackgroundAuthentication re-authenticates the user's api session in the background
// and shows an error notification in the tray once authentication has failed
func (m *TrayManager) PerformBackgroundAuthentication() {
	if !m.api.Account.Credentials.HasApiKey() {
		return
	}

	if err := m.api.Authenticate(); err != nil {
		m.OnTrayProgressFail()

		errorMessage := puush.FormatError(err)
		shouldRetry := puush.ShouldRetryError(err)

		if shouldRetry {
			time.AfterFunc(time.Second*15, m.PerformBackgroundAuthentication)
			errorMessage += " Retrying..."
		}

		go notifications.NewNotification("puush", i18n.T("puush error"), errorMessage).
			WithIconData(assets.PuushIconData).
			Push()

		// TODO: Clear account credentials?
	}
}
