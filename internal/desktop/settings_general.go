package desktop

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/sqweek/dialog"
)

func (ui *UI) buildGeneralTab() fyne.CanvasObject {
	startupCheckbox := widget.NewCheck("Start puush on startup", ui.UpdateAutostartConfiguration)
	startupCheckbox.Checked = ui.config.General.Startup

	soundCheckbox := widget.NewCheck("Play a notification sound", func(b bool) { ui.config.General.NotificationSound = b })
	soundCheckbox.Checked = ui.config.General.NotificationSound

	copyLinkCheckbox := widget.NewCheck("Copy link to clipboard", func(b bool) { ui.config.General.CopyToClipboard = b })
	copyLinkCheckbox.Checked = ui.config.General.CopyToClipboard

	openBrowserCheckbox := widget.NewCheck("Open link in browser", func(b bool) { ui.config.General.OpenBrowser = b })
	openBrowserCheckbox.Checked = ui.config.General.OpenBrowser

	saveClipboardCheckbox := widget.NewCheck("Save image to the clipboard", func(b bool) { ui.config.Capture.SaveImagesToClipboard = b })
	saveClipboardCheckbox.Checked = ui.config.Capture.SaveImagesToClipboard

	saveLocalCheckbox := widget.NewCheck("Save a local copy of image", func(b bool) { ui.config.Capture.SaveImages = b })
	saveLocalCheckbox.Checked = ui.config.Capture.SaveImages

	savePathEntry := widget.NewEntry()
	savePathEntry.SetText(ui.config.Capture.SaveImagePath)
	savePathEntry.OnChanged = func(s string) { ui.config.Capture.SaveImagePath = s }

	browseButton := widget.NewButton("...", func() {
		path, err := dialog.Directory().Title("Select a file to upload").Browse()
		if err == nil {
			savePathEntry.SetText(path)
			ui.config.Capture.SaveImagePath = path
		}
	})
	saveLocalPathContainer := container.NewBorder(nil, nil, nil, browseButton, savePathEntry)

	onSuccessLeft := container.NewVBox(soundCheckbox, copyLinkCheckbox, openBrowserCheckbox)
	onSuccessRight := container.NewVBox(saveClipboardCheckbox, saveLocalCheckbox, saveLocalPathContainer)

	onSuccessGrid := container.NewGridWithColumns(2, onSuccessLeft, onSuccessRight)
	return container.NewVBox(
		widget.NewSeparator(),
		createGroup("General Settings", startupCheckbox),
		widget.NewSeparator(),
		createGroup("On successful puush", onSuccessGrid),
		widget.NewSeparator(),
	)
}
