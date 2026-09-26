package desktop

import (
	"context"
	"errors"
	"os"
	"runtime"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/skip2/go-qrcode"

	"github.com/sundeiii/yeet-client/internal/i18n"
	"github.com/sundeiii/yeet-client/pkg/puush"
)

// Logging in through the website: the app shows a link (and a QR code for
// phones), the user logs in there as usual, two-factor login included, and
// approves the app, which then gets its API key. No password is typed into
// the app.

// loginWithWebsite opens the login window. onDone runs on the main thread
// once the app is logged in.
func (ui *UI) loginWithWebsite(onDone func()) {
	w := ui.app.NewWindow(i18n.T("Log in with the website"))
	w.SetIcon(puushIcon)
	w.SetFixedSize(true)

	ctx, cancel := context.WithCancel(context.Background())
	w.SetOnClosed(cancel)

	status := widget.NewLabel(i18n.T("Getting a login link…"))
	status.Wrapping = fyne.TextWrapWord
	status.Alignment = fyne.TextAlignCenter

	code := canvas.NewImageFromResource(theme.QuestionIcon())
	code.FillMode = canvas.ImageFillContain
	code.SetMinSize(fyne.NewSquareSize(190))
	code.Hide()

	var link string
	open := widget.NewButtonWithIcon(i18n.T("Open in browser"), theme.ComputerIcon(), func() { OpenBrowser(link) })
	copyLink := widget.NewButtonWithIcon(i18n.T("Copy link"), theme.ContentCopyIcon(), func() {
		fyne.CurrentApp().Clipboard().SetContent(link)
	})
	open.Disable()
	copyLink.Disable()
	again := widget.NewButtonWithIcon(i18n.T("Try again"), theme.ViewRefreshIcon(), func() {
		w.Close()
		ui.loginWithWebsite(onDone)
	})
	again.Hide()

	w.SetContent(container.NewPadded(container.NewBorder(
		container.NewCenter(code),
		container.NewVBox(container.NewGridWithColumns(2, open, copyLink), again),
		nil, nil,
		status,
	)))
	w.Resize(fyne.NewSize(400, 370))
	w.CenterOnScreen()
	w.Show()

	fail := func(message string) {
		fyne.Do(func() {
			if ctx.Err() != nil {
				return
			}
			status.SetText(message)
			code.Hide()
			open.Disable()
			copyLink.Disable()
			again.Show()
			w.Content().Refresh()
		})
	}

	go func() {
		ui.api.SetBaseURL(ui.config.Misc.ParseServerURL().String())
		login, err := ui.api.StartAppLogin(deviceName())
		switch {
		case errors.Is(err, puush.ErrNotSupported):
			fail(i18n.T("This server can't log in the app through the website yet. Log in with your email and password instead."))
			return
		case err != nil:
			fail(puush.FormatError(err))
			return
		}

		picture, _ := qrcode.Encode(login.Url, qrcode.Medium, 380)
		fyne.Do(func() {
			if ctx.Err() != nil {
				return
			}
			link = login.Url
			if len(picture) > 0 {
				code.Resource = fyne.NewStaticResource("login-qr.png", picture)
				code.Show()
				code.Refresh()
			}
			status.SetText(i18n.T("Log in on the website in your browser, or scan the code with your phone. Two-factor login works as usual. This window closes by itself once you approve the app."))
			open.Enable()
			copyLink.Enable()
			w.Content().Refresh() // room for the code
		})
		OpenBrowser(login.Url)

		expires := time.Now().Add(time.Duration(max(login.Expires, 60)) * time.Second)
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
			err := ui.api.PollAppLogin(login.Code)
			switch {
			case err == nil:
				fyne.Do(func() {
					w.Close()
					onDone()
				})
				return
			case errors.Is(err, puush.ErrLoginWaiting):
			case errors.Is(err, puush.ErrLoginExpired), time.Now().After(expires):
				fail(i18n.T("The login link expired, or the app wasn't approved."))
				return
			default:
				// The connection dropped for a moment; keep asking
			}
		}
	}()
}

// deviceName describes this computer on the website, like "DESKTOP-1 (Windows)".
func deviceName() string {
	system := map[string]string{"windows": "Windows", "darwin": "macOS", "linux": "Linux"}[runtime.GOOS]
	if system == "" {
		system = runtime.GOOS
	}
	if host, err := os.Hostname(); err == nil && host != "" {
		return host + " (" + system + ")"
	}
	return system
}
