package desktop

import (
	"fmt"
	"net/url"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/sundeiii/yeet-client/pkg/puush"
)

func (ui *UI) buildAccountTab() (fyne.CanvasObject, func()) {
	var updateView func()

	accountContainer := container.NewStack()
	updateView = func() {
		defer accountContainer.Refresh()
		accountContainer.Objects = nil

		if ui.config.Account.HasCredentials() {
			accountContainer.Add(ui.buildAccountDetails(updateView))
		} else {
			accountContainer.Add(ui.buildAccountSetup(updateView))
		}
	}
	updateView()

	view := container.NewVBox(
		widget.NewSeparator(),
		accountContainer,
		widget.NewSeparator(),
	)
	return view, updateView
}

func (ui *UI) buildAccountSetup(updateView func()) fyne.CanvasObject {
	infoText := "You need to login before you can make full use of puush. "
	infoText += "If you don't already have an account, you can register for free via the link below."
	infoLabel := widget.NewLabel(infoText)
	infoLabel.Wrapping = fyne.TextWrapWord

	serverUrl := ui.config.Misc.ParseServerURL()
	resetUrl := serverUrl.String() + "/reset_password"
	registerUrl := serverUrl.String() + "/register"

	emailEntry := widget.NewEntry()
	passwordEntry := widget.NewPasswordEntry()
	emailLabel := trailingLabel("Email:")
	passwordLabel := trailingLabel("Password:")

	form := container.NewGridWithColumns(2,
		emailLabel, emailEntry,
		passwordLabel, passwordEntry,
	)

	forgotURL, _ := url.Parse(resetUrl)
	registerURL, _ := url.Parse(registerUrl)
	forgotLink := NewUnderlinedLink("Forgotten Password?", forgotURL)
	registerLink := NewUnderlinedLink("Sign up for free account...", registerURL)
	linksContainer := container.NewHBox(forgotLink, layout.NewSpacer(), registerLink)

	var loginButton *BorderedButton

	disableLogin := func() {
		loginButton.Instance.Disable()
		emailEntry.Disable()
		passwordEntry.Disable()
	}
	enableLogin := func() {
		loginButton.Instance.Enable()
		emailEntry.Enable()
		passwordEntry.Enable()
	}
	performLogin := func() {
		fyne.Do(disableLogin)
		defer fyne.Do(enableLogin)

		ui.api.Account.Credentials = &puush.Credentials{
			Identifier: &emailEntry.Text,
			Password:   &passwordEntry.Text,
		}
		ui.api.SetBaseURL(serverUrl.String())

		// Attempt authentication with new credentials
		if err := ui.api.Authenticate(); err != nil {
			showError(err)
			return
		}

		defer ui.UpdateAccountConfiguration()
		defer fyne.Do(updateView)
	}
	loginButton = NewBorderedButton("Login", func() { go performLogin() })

	sizedForm := container.NewGridWrap(fyne.NewSize(350, 55), form)
	sizedLoginButton := container.NewGridWrap(fyne.NewSize(140, 53), loginButton)

	loginContainer := container.NewHBox(
		layout.NewSpacer(),
		sizedForm, widget.NewLabel(" "), sizedLoginButton,
		layout.NewSpacer(),
	)

	content := container.NewVBox(
		infoLabel,
		widget.NewLabel(""),
		loginContainer,
		widget.NewLabel(""),
		linksContainer,
	)

	return createGroup("Account Setup", content)
}

func (ui *UI) buildAccountDetails(updateView func()) fyne.CanvasObject {
	accountTypeString := ui.config.Account.Type.String() + " Account" // e.g. "Pro Account"
	diskUsageString := ui.config.Account.DiskUsageHumanReadable()     // e.g. 1.5 GB

	expiryTime := ui.config.Account.SubscriptionExpiry()
	expiryString := "Never"
	if expiryTime != nil {
		// TODO: Check if this is the right date time format
		expiryString = expiryTime.Format(time.DateTime)
	}

	detailsGrid := container.NewGridWithColumns(2,
		trailingLabel("Logged in as:"), widget.NewLabel(ui.config.Account.Username),
		trailingLabel("API Key:"), widget.NewLabel(ui.config.Account.Key),
		trailingLabel("Account Type:"), widget.NewLabel(accountTypeString),
		trailingLabel("Expiry Date:"), widget.NewLabel(expiryString),
		trailingLabel("Disk Usage:"), widget.NewLabel(diskUsageString),
	)

	myAccountButton := NewBorderedButton("My Account", func() {
		path := fmt.Sprintf("/login/go/?k=%s", ui.config.Account.Key)
		OpenBrowser(ui.api.FormatURL(path))
	})
	logoutButton := NewBorderedButton("Logout", func() {
		ui.config.Account.Reset()
		ui.api.Account.Reset()
		updateView()
	})
	buttons := container.NewGridWithColumns(
		2, myAccountButton, logoutButton,
	)

	content := container.NewVBox(
		container.NewPadded(detailsGrid),
		widget.NewLabel(""),
		container.NewPadded(buttons),
	)
	return createGroup("Account Details", content)
}
