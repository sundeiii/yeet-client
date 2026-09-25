package desktop

import (
	"github.com/sundeiii/yeet-client/internal/i18n"
	"image/color"
	"net/url"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/sqweek/dialog"

	"github.com/sundeiii/yeet-client/assets"
	"github.com/sundeiii/yeet-client/pkg/puush"
)

// puush uses a pre-made background asset for the quick start window.
// Elements like the login prompt are just added on top of it.

func (ui *UI) ShowStartupWindow() {
	if ui.startupWindow != nil {
		ui.startupWindow.Show()
		ui.startupWindow.RequestFocus()
		return
	}

	w := ui.app.NewWindow(i18n.T("puush quick start"))
	w.SetOnClosed(func() { ui.startupWindow = nil })
	w.SetFixedSize(true)
	w.SetIcon(puushIcon)
	ui.startupWindow = w

	serverUrl := ui.config.Misc.ParseServerURL()
	registerUrl := serverUrl.String() + "/register"
	resetUrl := serverUrl.String() + "/reset_password"

	// Create the background image from our embedded asset
	bgResource := fyne.NewStaticResource("quickstart_bg", assets.QuickstartData)
	bgImage := canvas.NewImageFromResource(bgResource)
	bgImage.FillMode = canvas.ImageFillCover
	bgImage.SetMinSize(fyne.NewSize(640, 540))

	// Create button to link to account page
	registerBtn := NewBorderedButton(i18n.T("Take me to the account creation page!"), func() {
		OpenBrowser(registerUrl)
	})
	registerBtn.Move(fyne.NewPos(200, 138))
	registerBtn.Resize(fyne.NewSize(250, 28))

	// Right-aligned next to the boxes, since the words differ in length per language
	emailLabel := canvas.NewText(i18n.T("Email:"), color.Black)
	emailLabel.Move(fyne.NewPos(192-emailLabel.MinSize().Width, 207))

	passwordLabel := canvas.NewText(i18n.T("Password:"), color.Black)
	passwordLabel.Move(fyne.NewPos(192-passwordLabel.MinSize().Width, 237))

	// Create the inputs that will be placed over the background
	emailEntry := widget.NewEntry()
	emailEntry.Move(fyne.NewPos(200, 200))
	emailEntry.Resize(fyne.NewSize(150, 25))

	passwordEntry := widget.NewPasswordEntry()
	passwordEntry.Move(fyne.NewPos(200, 230))
	passwordEntry.Resize(fyne.NewSize(150, 25))

	// Forgot password hyperlink
	forgotURL, _ := url.Parse(resetUrl)
	forgotLink := NewUnderlinedLink(i18n.T("Forgotten Password?"), forgotURL)
	forgotLink.Move(fyne.NewPos(201, 256))
	forgotLink.Resize(forgotLink.MinSize())

	// Create a rectangle to cover the "Login successful" text
	coverRectangle := canvas.NewRectangle(color.White)
	coverRectangle.Move(fyne.NewPos(125, 200))
	coverRectangle.Resize(fyne.NewSize(400, 75))

	var loginBtn *BorderedButton
	var okayBtn *BorderedButton

	disableLoginElements := func() {
		emailEntry.Disable()
		passwordEntry.Disable()
		loginBtn.Instance.Disable()
	}
	enableLoginElements := func() {
		emailEntry.Enable()
		passwordEntry.Enable()
		loginBtn.Instance.Enable()
	}
	onLoginSuccess := func() {
		// Hide login elements
		loginBtn.Hide()
		emailEntry.Hide()
		passwordEntry.Hide()
		coverRectangle.Hide()
		emailLabel.Hide()
		passwordLabel.Hide()
		forgotLink.Hide()

		// Enable "okay" button
		okayBtn.Instance.Enable()

		// Save values in config
		ui.UpdateAccountConfiguration()
	}

	performLogin := func() {
		// Disable input & enable it back afterwards
		fyne.Do(disableLoginElements)
		defer fyne.Do(enableLoginElements)

		ui.api.Account.Credentials = &puush.Credentials{
			Identifier: &emailEntry.Text,
			Password:   &passwordEntry.Text,
		}
		ui.api.SetBaseURL(serverUrl.String())

		// Attempt authentication with new credentials
		if err := ui.authenticateWithTwoFactor(); err != nil {
			if err != errLoginCancelled {
				showError(err)
			}
			return
		}

		fyne.Do(onLoginSuccess)
	}

	loginBtn = NewBorderedButton(i18n.T("Login"), func() { go performLogin() })
	loginBtn.Move(fyne.NewPos(370, 200))
	loginBtn.Resize(fyne.NewSize(160, 55))
	loginBtn.Instance.Disable()

	// Login button should be enabled once password & email have been filled in
	checkLoginStatus := func(s string) {
		if emailEntry.Text != "" && passwordEntry.Text != "" {
			loginBtn.Instance.Enable()
		} else {
			loginBtn.Instance.Disable()
		}
	}
	emailEntry.OnChanged = checkLoginStatus
	passwordEntry.OnChanged = checkLoginStatus

	// Container for the background and absolutely positioned overlays
	overlayContainer := container.NewWithoutLayout(
		bgImage,
		coverRectangle,
		registerBtn,
		emailLabel,
		passwordLabel,
		emailEntry,
		passwordEntry,
		forgotLink,
		loginBtn,
	)
	bgContainer := container.NewStack(bgImage, overlayContainer)

	startupCheckbox := widget.NewCheck(i18n.T("Start puush on startup"), ui.UpdateAutostartConfiguration)
	startupCheckbox.SetChecked(ui.config.General.Startup)

	okayBtn = NewBorderedButton(i18n.T("Okay, I've got it!"), func() {
		w.Close()
	})
	okayBtn.Instance.Disable()

	// Force-resize this button inside a new container, since the HBox will not allow that
	sizedOkayBtn := container.NewGridWrap(fyne.NewSize(325, 32), okayBtn)

	// Layout the bottom bar
	bottomBar := container.NewHBox(
		layout.NewSpacer(),
		startupCheckbox,
		layout.NewSpacer(),
		sizedOkayBtn,
		layout.NewSpacer(),
	)

	// Combine the background area and the bottom bar
	mainContent := container.NewVBox(
		bgContainer,
		widget.NewSeparator(),
		container.NewPadded(bottomBar),
	)
	w.SetContent(mainContent)
	w.Resize(fyne.NewSize(640, 540))
	w.Show()
}

func showError(err error) {
	errorMessage := formatStartupError(err)
	dialog.Message("%s", errorMessage).Title(i18n.T("puush error")).Error()
}

func formatStartupError(err error) string {
	if err == puush.PuushErrorInvalidCredentials {
		return i18n.T("The username or password you entered is incorrect.")
	}
	return puush.FormatError(err)
}
