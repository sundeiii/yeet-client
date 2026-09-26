package desktop

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/sundeiii/yeet-client/internal/i18n"
)

// Tabs of the app window, in order. Settings holds its own tabs.
const (
	tabHome = iota
	tabUploads
	tabQueue
	tabMessages
	tabSettings
)

// Tabs inside Settings, in order
const (
	settingsGeneral = iota
	settingsKeyBindings
	settingsAccount
	settingsUpdate
	settingsAdvanced
	settingsAbout
)

// ShowAppWindow opens the app window on its home page.
func (ui *UI) ShowAppWindow() {
	ui.showWindow(tabHome)
}

// ShowQueueWindow opens the app window on the upload queue.
func (ui *UI) ShowQueueWindow() {
	ui.showWindow(tabQueue)
}

// ShowMessagesWindow opens the app window on the chats.
func (ui *UI) ShowMessagesWindow() {
	ui.showWindow(tabMessages)
}

// ShowSettingsWindow opens the app window on the settings.
func (ui *UI) ShowSettingsWindow() {
	ui.showWindow(tabSettings)
}

func (ui *UI) showWindow(tab int) {
	// Logged out, there's only logging in, like the very first start
	if !ui.config.Account.HasCredentials() {
		if ui.settingsWindow != nil {
			ui.settingsWindow.Close()
		}
		ui.ShowStartupWindow()
		return
	}
	if ui.startupWindow != nil {
		ui.startupWindow.Close()
	}

	if ui.settingsWindow != nil {
		ui.windowTabs.SelectIndex(tab)
		if tab == tabSettings {
			ui.settingsTabs.SelectIndex(settingsGeneral)
		}
		ui.settingsWindow.Show()
		ui.settingsWindow.RequestFocus()
		return
	}

	w := ui.app.NewWindow("puush")
	w.SetOnClosed(func() {
		ui.settingsWindow = nil
		ui.windowTabs = nil
		ui.settingsTabs = nil
		ui.refreshHome = nil
		ui.uploads = nil
		ui.queue = nil
		if ui.messages != nil {
			ui.messages.close()
			ui.messages = nil
		}
		ui.SetUpdateFinishedCallback(nil)
	})
	// Files dropped on the window go into the open chat, or are uploaded
	w.SetOnDropped(func(_ fyne.Position, uris []fyne.URI) {
		if ui.messages != nil && ui.messages.dropped(uris) {
			return
		}
		var paths []string
		for _, uri := range uris {
			if uri.Scheme() == "file" {
				paths = append(paths, uri.Path())
			}
		}
		if len(paths) > 0 {
			if err := ui.tray.EnqueueFiles(paths); err != nil {
				ui.tray.OnUploadError(err)
			}
		}
	})
	w.Resize(fyne.NewSize(720, 520))
	w.SetIcon(puushIcon)
	ui.settingsWindow = w

	ui.fillWindow(w, tab)
	w.Show()

	// Fresh account details, e.g. the disk usage after uploading on the website
	go ui.refreshAccount()
}

// onTrayChange runs when uploads, failed uploads or pools change.
func (ui *UI) onTrayChange() {
	if ui.refreshHome != nil {
		ui.refreshHome()
	}
	if ui.uploads != nil {
		ui.uploads.reloadSoon()
	}
}

// refreshAccount asks the server for the account's details again.
func (ui *UI) refreshAccount() {
	if !ui.api.Account.Credentials.HasApiKey() {
		return
	}
	if err := ui.api.Authenticate(); err != nil {
		return
	}
	ui.UpdateAccountConfiguration()
	fyne.Do(func() {
		if ui.refreshHome != nil {
			ui.refreshHome()
		}
	})
}

func createGroup(title string, content fyne.CanvasObject) fyne.CanvasObject {
	indentedContent := container.NewBorder(nil, nil, widget.NewLabel("    "), widget.NewLabel("    "), content)
	return widget.NewCard("", title, indentedContent)
}

func createGroupNoIndent(title string, content fyne.CanvasObject) fyne.CanvasObject {
	return widget.NewCard("", title, content)
}

func trailingLabel(text string) *widget.Label {
	label := widget.NewLabel(text)
	label.Alignment = fyne.TextAlignTrailing
	return label
}

// fillWindow builds the app window's tabs, in the current language, and
// shows the given one.
func (ui *UI) fillWindow(w fyne.Window, tab int) {
	var tabs, settingsTabs *container.AppTabs
	goToAccount := func() {
		tabs.SelectIndex(tabSettings)
		settingsTabs.SelectIndex(settingsAccount)
	}

	accountView, accountViewUpdate := ui.buildAccountTab()
	homeView, homeRefresh := ui.buildHomeTab(w, goToAccount)
	uploadsView := ui.buildUploadsTab(w)
	queueView := ui.buildQueueTab()
	messagesView := ui.buildMessagesTab()
	generalView := ui.buildGeneralTab()
	keyBindingsView := ui.buildKeyBindingsTab()
	advancedView := ui.buildAdvancedTab(accountViewUpdate)
	updateView := ui.buildUpdateTab()
	aboutView := ui.buildAboutTab()

	// The settings pages sit on the left inside one Settings tab, so the
	// top row stays short
	settingsTabs = container.NewAppTabs(
		container.NewTabItemWithIcon(i18n.T("General"), theme.SettingsIcon(), generalView),
		container.NewTabItemWithIcon(i18n.T("Key Bindings"), theme.ComputerIcon(), keyBindingsView),
		container.NewTabItemWithIcon(i18n.T("Account"), theme.AccountIcon(), accountView),
		container.NewTabItemWithIcon(i18n.T("Update"), theme.DownloadIcon(), updateView),
		container.NewTabItemWithIcon(i18n.T("Advanced"), theme.MoreHorizontalIcon(), advancedView),
		container.NewTabItemWithIcon(i18n.T("About"), theme.InfoIcon(), aboutView),
	)
	settingsTabs.SetTabLocation(container.TabLocationLeading)

	tabs = container.NewAppTabs(
		container.NewTabItemWithIcon(i18n.T("Home"), theme.HomeIcon(), homeView),
		container.NewTabItemWithIcon(i18n.T("Uploads"), theme.StorageIcon(), uploadsView.content),
		container.NewTabItemWithIcon(i18n.T("Queue"), theme.UploadIcon(), queueView.content),
		container.NewTabItemWithIcon(i18n.T("Messages"), theme.MailComposeIcon(), messagesView.content),
		container.NewTabItemWithIcon(i18n.T("Settings"), theme.SettingsIcon(), settingsTabs),
	)
	tabs.OnSelected = func(item *container.TabItem) {
		if item.Content == uploadsView.content {
			uploadsView.loadIfEmpty()
		}
		if item.Content == messagesView.content {
			messagesView.show()
		} else {
			messagesView.hide()
		}
	}
	ui.windowTabs = tabs
	ui.settingsTabs = settingsTabs
	ui.refreshHome = func() {
		homeRefresh()
		// Rebuilding the login form would throw away what's being typed in it
		if ui.config.Account.HasCredentials() {
			accountViewUpdate()
		}
	}
	ui.uploads = uploadsView
	ui.queue = queueView
	ui.messages = messagesView

	tabs.SelectIndex(tab)
	if tab == tabUploads {
		uploadsView.loadIfEmpty()
	}
	if tab == tabMessages {
		messagesView.show()
	}
	w.SetContent(container.NewPadded(tabs))
}
