package desktop

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/sqweek/dialog"
	"github.com/sundeiii/yeet-client/internal/config"
	"github.com/sundeiii/yeet-client/internal/notifications"
)

func (ui *UI) buildGeneralTab() fyne.CanvasObject {
	startupCheckbox := widget.NewCheck("Start puush on startup", ui.UpdateAutostartConfiguration)
	startupCheckbox.Checked = ui.config.General.Startup

	// Sound after an upload, with a button to hear it
	soundNames := map[string]string{
		notifications.SoundPuush:  "puush (classic)",
		notifications.SoundPop:    "Pop",
		notifications.SoundChime:  "Chime",
		notifications.SoundSystem: "System sound",
		notifications.SoundNone:   "No sound",
	}
	var soundLabels []string
	for _, name := range notifications.SoundChoices {
		soundLabels = append(soundLabels, soundNames[name])
	}
	soundSelect := widget.NewSelect(soundLabels, func(label string) {
		for name, l := range soundNames {
			if l == label {
				ui.config.General.Sound = name
				ui.config.General.NotificationSound = name != notifications.SoundNone
			}
		}
	})
	soundSelect.SetSelected(soundNames[ui.config.General.Sound])
	playButton := widget.NewButtonWithIcon("", theme.MediaPlayIcon(), func() {
		notifications.PlaySound(ui.config.General.Sound)
	})
	soundCheckbox := container.NewBorder(nil, nil, widget.NewLabel("Sound:"), playButton, soundSelect)

	notifyCheckbox := widget.NewCheck("Show a notification after each upload", func(b bool) { ui.config.General.NotifySuccess = b })
	notifyCheckbox.Checked = ui.config.General.NotifySuccess
	previewCheckbox := widget.NewCheck("Show the picture in the notification", func(b bool) { ui.config.General.NotifyPreview = b })
	previewCheckbox.Checked = ui.config.General.NotifyPreview

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
	// An empty box means the default folder, so show where that is
	savePathEntry.SetPlaceHolder(config.DefaultSaveImagePath())
	savePathEntry.OnChanged = func(s string) { ui.config.Capture.SaveImagePath = s }

	browseButton := widget.NewButton("...", func() {
		path, err := dialog.Directory().Title("Choose where to save screenshots").Browse()
		if err == nil {
			savePathEntry.SetText(path)
			ui.config.Capture.SaveImagePath = path
		}
	})
	saveLocalPathContainer := container.NewBorder(nil, nil, nil, browseButton, savePathEntry)

	onSuccessLeft := container.NewVBox(soundCheckbox, notifyCheckbox, previewCheckbox, copyLinkCheckbox, openBrowserCheckbox)
	onSuccessRight := container.NewVBox(saveClipboardCheckbox, saveLocalCheckbox, saveLocalPathContainer)

	onSuccessGrid := container.NewGridWithColumns(2, onSuccessLeft, onSuccessRight)

	// Countdown for "Capture in N Seconds"
	delayOptions := []string{"3 seconds", "5 seconds", "10 seconds"}
	delaySelect := widget.NewSelect(delayOptions, func(s string) {
		var seconds int
		fmt.Sscanf(s, "%d", &seconds)
		ui.config.Capture.DelaySeconds = seconds
		ui.tray.RebuildMenu()
	})
	delaySelect.SetSelected(fmt.Sprintf("%d seconds", int(ui.config.Capture.Delay().Seconds())))
	delayRow := container.NewHBox(widget.NewLabel("Delayed captures wait"), delaySelect)

	editCheckbox := widget.NewCheck("Open the editor after every screenshot", func(b bool) { ui.config.Capture.EditBeforeUpload = b })
	editCheckbox.Checked = ui.config.Capture.EditBeforeUpload

	albumCheckbox := widget.NewCheck("Share several files as one album link", func(b bool) { ui.config.General.Albums = b })
	albumCheckbox.Checked = ui.config.General.Albums

	return container.NewVScroll(container.NewVBox(
		widget.NewSeparator(),
		createGroup("General Settings", startupCheckbox),
		widget.NewSeparator(),
		createGroup("On successful puush", onSuccessGrid),
		widget.NewSeparator(),
		createGroup("Capturing", container.NewVBox(delayRow, editCheckbox, albumCheckbox)),
		widget.NewSeparator(),
	))
}
