package desktop

import (
	"net/url"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/sundeiii/yeet-client/internal/i18n"
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
	infoText := i18n.T("You need to login before you can make full use of puush. If you don't already have an account, you can register for free via the link below.")
	infoLabel := widget.NewLabel(infoText)
	infoLabel.Wrapping = fyne.TextWrapWord

	serverUrl := ui.config.Misc.ParseServerURL()
	resetUrl := serverUrl.String() + "/reset_password"
	registerUrl := serverUrl.String() + "/register"

	emailEntry := widget.NewEntry()
	passwordEntry := widget.NewPasswordEntry()
	emailLabel := trailingLabel(i18n.T("Email:"))
	passwordLabel := trailingLabel(i18n.T("Password:"))

	form := container.NewGridWithColumns(2,
		emailLabel, emailEntry,
		passwordLabel, passwordEntry,
	)

	forgotURL, _ := url.Parse(resetUrl)
	registerURL, _ := url.Parse(registerUrl)
	forgotLink := NewUnderlinedLink(i18n.T("Forgotten Password?"), forgotURL)
	registerLink := NewUnderlinedLink(i18n.T("Sign up for free account..."), registerURL)
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
		if err := ui.authenticateWithTwoFactor(); err != nil {
			if err != errLoginCancelled {
				showError(err)
			}
			return
		}

		defer ui.UpdateAccountConfiguration()
		defer fyne.Do(updateView)
	}
	loginButton = NewBorderedButton(i18n.T("Login"), func() { go performLogin() })

	sizedForm := container.NewGridWrap(fyne.NewSize(350, 55), form)
	sizedLoginButton := container.NewGridWrap(fyne.NewSize(140, 53), loginButton)

	loginContainer := container.NewHBox(
		layout.NewSpacer(),
		sizedForm, widget.NewLabel(" "), sizedLoginButton,
		layout.NewSpacer(),
	)

	websiteButton := NewBorderedButton(i18n.T("Log in with the website"), func() {
		ui.loginWithWebsite(func() {
			ui.UpdateAccountConfiguration()
			updateView()
		})
	})
	websiteHint := widget.NewLabel(i18n.T("No password needed here, and two-factor login works too."))
	websiteHint.Importance = widget.LowImportance
	websiteHint.Alignment = fyne.TextAlignCenter
	websiteRow := container.NewVBox(
		container.NewCenter(container.NewGridWrap(fyne.NewSize(max(220, websiteButton.MinSize().Width+20), 30), websiteButton)),
		websiteHint,
	)
	content := container.NewVBox(
		infoLabel,
		widget.NewLabel(""),
		loginContainer,
		websiteRow,
		widget.NewLabel(""),
		linksContainer,
	)

	return createGroup(i18n.T("Account Setup"), content)
}

func (ui *UI) buildAccountDetails(updateView func()) fyne.CanvasObject {
	accountTypeString := i18n.T(ui.config.Account.Type.String() + " account") // e.g. "Pro account"
	diskUsageString := ui.config.Account.DiskUsageHumanReadable()             // e.g. 1.5 GB

	expiryTime := ui.config.Account.SubscriptionExpiry()
	expiryString := i18n.T("Never")
	if expiryTime != nil {
		expiryString = i18n.Date(expiryTime.Local())
	}

	// Labels as wide as the longest one, values right next to them
	detailsGrid := container.New(layout.NewFormLayout(),
		trailingLabel(i18n.T("Logged in as:")), widget.NewLabel(ui.config.Account.Username),
		trailingLabel(i18n.T("API Key:")), widget.NewLabel(ui.config.Account.Key),
		trailingLabel(i18n.T("Account Type:")), widget.NewLabel(accountTypeString),
		trailingLabel(i18n.T("Expiry Date:")), widget.NewLabel(expiryString),
		trailingLabel(i18n.T("Storage:")), ui.storageBar(diskUsageString),
	)

	myAccountButton := NewBorderedButton(i18n.T("My Account"), func() {
		// Asks the server for a one-time login link, so don't block the window
		go OpenBrowser(ui.api.AccountLink())
	})
	logoutButton := NewBorderedButton(i18n.T("Logout"), func() {
		ui.Logout()
		updateView()
	})
	buttons := container.NewGridWrap(fyne.NewSize(160, 34), myAccountButton, logoutButton)

	content := container.NewVBox(
		ui.profileHeader(),
		detailsGrid,
		container.NewPadded(buttons),
	)
	return createGroup(i18n.T("Account Details"), content)
}

// profileHeader shows the avatar and a link to the profile page.
func (ui *UI) profileHeader() fyne.CanvasObject {
	avatar := canvas.NewImageFromResource(theme.AccountIcon())
	if ui.avatar != nil {
		avatar.Resource = ui.avatar
	}
	avatar.FillMode = canvas.ImageFillContain
	avatar.SetMinSize(fyne.NewSquareSize(56))

	name := widget.NewLabel(ui.config.Account.Username)
	var link fyne.CanvasObject
	if ui.profile != nil && ui.profile.Profile != "" {
		name.SetText(ui.profile.Name)
		profileUrl, _ := url.Parse(ui.profile.Profile)
		link = widget.NewHyperlink(i18n.T("View my profile"), profileUrl)
	} else {
		hint := widget.NewLabel(i18n.T("Pick a username on the website to get a profile page."))
		hint.Importance = widget.LowImportance
		link = hint
	}
	text := container.New(layout.NewCustomPaddedVBoxLayout(-10), name, link)
	return container.NewHBox(container.NewPadded(avatar), container.NewCenter(text))
}

// storageBar shows how much of the account's storage is used, as a bar
// when there's a limit.
func (ui *UI) storageBar(usage string) fyne.CanvasObject {
	if ui.profile == nil || ui.profile.Limit <= 0 {
		text := usage
		if ui.profile != nil {
			text = i18n.T("%s (unlimited)", puush.FormatBytes(ui.profile.Usage))
		}
		return widget.NewLabel(text)
	}
	used, limit := ui.profile.Usage, ui.profile.Limit
	bar := widget.NewProgressBar()
	bar.Max = float64(limit)
	bar.SetValue(float64(min(used, limit)))
	bar.TextFormatter = func() string {
		return i18n.T("%s of %s used", puush.FormatBytes(used), puush.FormatBytes(limit))
	}
	return container.NewPadded(bar)
}

// refreshProfile asks the server for the profile: the avatar, the profile
// page and the storage. Not for the main thread.
func (ui *UI) refreshProfile() {
	profile, err := ui.api.Profile()
	if err != nil {
		return
	}
	var avatar fyne.Resource
	if profile.Avatar != "" {
		if data, err := ui.api.Picture(profile.Avatar); err == nil {
			avatar = fyne.NewStaticResource("avatar.png", data)
		}
	}
	fyne.Do(func() {
		if !ui.api.Account.Credentials.HasApiKey() {
			return // logged out meanwhile
		}
		ui.profile = profile
		ui.avatar = avatar
		ui.tray.ChatRead(profile.Unread)
		if ui.refreshHome != nil {
			ui.refreshHome()
		}
	})
}
