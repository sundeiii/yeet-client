package desktop

import (
	"errors"
	"github.com/sundeiii/yeet-client/internal/i18n"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/sundeiii/yeet-client/pkg/puush"
)

// errLoginCancelled means the user closed the code prompt; nothing to report.
var errLoginCancelled = errors.New("login cancelled")

// authenticateWithTwoFactor logs in, asking for the authenticator app's code
// when the account has two-factor login. Not for the main thread.
func (ui *UI) authenticateWithTwoFactor() error {
	err := ui.api.Authenticate()
	message := i18n.T("This account has two-factor login. Enter the 6-digit code from your authenticator app, or a recovery code.")
	for tries := 0; tries < 5; tries++ {
		switch err {
		case puush.PuushErrorTwoFactorRequired:
		case puush.PuushErrorTwoFactorWrong:
			message = i18n.T("That code didn't work. Codes change every 30 seconds, so try the one showing now.")
		default:
			return err
		}
		code := ui.askTwoFactorCode(message)
		if code == "" {
			return errLoginCancelled
		}
		ui.api.Account.Credentials.TwoFactorCode = &code
		err = ui.api.Authenticate()
	}
	return err
}

// askTwoFactorCode shows a small window for the code and waits for it.
// It returns "" when the user cancels.
func (ui *UI) askTwoFactorCode(message string) string {
	result := make(chan string, 1)
	fyne.Do(func() {
		w := ui.app.NewWindow(i18n.T("puush login"))
		w.SetIcon(puushIcon)
		w.SetFixedSize(true)

		answered := false
		answer := func(code string) {
			if answered {
				return
			}
			answered = true
			result <- strings.TrimSpace(code)
			w.Close()
		}
		w.SetOnClosed(func() {
			if !answered {
				answered = true
				result <- ""
			}
		})

		label := widget.NewLabel(message)
		label.Wrapping = fyne.TextWrapWord
		entry := widget.NewEntry()
		entry.SetPlaceHolder("123456")
		entry.OnSubmitted = answer
		login := NewBorderedButton(i18n.T("Log in"), func() { answer(entry.Text) })
		cancel := NewBorderedButton(i18n.T("Cancel"), func() { answer("") })

		w.SetContent(container.NewPadded(container.NewVBox(
			label,
			entry,
			container.NewGridWithColumns(2, cancel, login),
		)))
		w.Resize(fyne.NewSize(360, 160))
		w.CenterOnScreen()
		w.Show()
		w.Canvas().Focus(entry)
	})
	return <-result
}
