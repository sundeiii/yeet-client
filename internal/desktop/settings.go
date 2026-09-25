package desktop

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// Tabs of the app window, in order
const (
	tabHome = iota
	tabUploads
	tabQueue
	tabGeneral
	tabKeyBindings
	tabAccount
)

// ShowAppWindow opens the app window on its home page.
func (ui *UI) ShowAppWindow() {
	ui.showWindow(tabHome)
}

// ShowQueueWindow opens the app window on the upload queue.
func (ui *UI) ShowQueueWindow() {
	ui.showWindow(tabQueue)
}

// ShowSettingsWindow opens the app window on the settings.
func (ui *UI) ShowSettingsWindow() {
	ui.showWindow(tabGeneral)
}

func (ui *UI) showWindow(tab int) {
	if ui.settingsWindow != nil {
		ui.windowTabs.SelectIndex(tab)
		ui.settingsWindow.Show()
		ui.settingsWindow.RequestFocus()
		return
	}

	w := ui.app.NewWindow("puush")
	w.SetOnClosed(func() {
		ui.settingsWindow = nil
		ui.windowTabs = nil
		ui.refreshHome = nil
		ui.uploads = nil
		ui.queue = nil
		ui.SetUpdateFinishedCallback(nil)
	})
	w.Resize(fyne.NewSize(740, 500))
	w.SetIcon(puushIcon)
	ui.settingsWindow = w

	var tabs *container.AppTabs
	goToAccount := func() { tabs.SelectIndex(tabAccount) }

	accountView, accountViewUpdate := ui.buildAccountTab()
	homeView, homeRefresh := ui.buildHomeTab(w, goToAccount)
	uploadsView := ui.buildUploadsTab(w)
	queueView := ui.buildQueueTab()
	generalView := ui.buildGeneralTab()
	keyBindingsView := ui.buildKeyBindingsTab()
	advancedView := ui.buildAdvancedTab(accountViewUpdate)
	updateView := ui.buildUpdateTab()
	aboutView := ui.buildAboutTab()

	tabs = container.NewAppTabs(
		container.NewTabItem("Home", homeView),
		container.NewTabItem("Uploads", uploadsView.content),
		container.NewTabItem("Queue", queueView.content),
		container.NewTabItem("General", generalView),
		container.NewTabItem("Key Bindings", keyBindingsView),
		container.NewTabItem("Account", accountView),
		container.NewTabItem("Update", updateView),
		container.NewTabItem("Advanced", advancedView),
		container.NewTabItem("About", aboutView),
	)
	tabs.OnSelected = func(item *container.TabItem) {
		if item.Content == uploadsView.content {
			uploadsView.loadIfEmpty()
		}
	}
	ui.windowTabs = tabs
	ui.refreshHome = func() {
		homeRefresh()
		// Rebuilding the login form would throw away what's being typed in it
		if ui.config.Account.HasCredentials() {
			accountViewUpdate()
		}
	}
	ui.uploads = uploadsView
	ui.queue = queueView

	tabs.SelectIndex(tab)
	if tab == tabUploads {
		uploadsView.loadIfEmpty()
	}
	w.SetContent(container.NewPadded(tabs))
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
