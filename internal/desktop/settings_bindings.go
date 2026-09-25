package desktop

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

func (ui *UI) buildKeyBindingsTab() fyne.CanvasObject {
	createHotkeyButton := func(initial string, onChange func(string)) *HotkeyButton {
		btn := NewHotkeyButton(initial)
		btn.OnStart = func() {
			ui.hotkeys.Stop()
			if canvas := ui.app.Driver().CanvasForObject(btn); canvas != nil {
				canvas.Focus(btn)
			}
		}
		btn.OnCancelled = func() {
			ui.hotkeys.Start()
			if canvas := ui.app.Driver().CanvasForObject(btn); canvas != nil {
				canvas.Focus(nil)
			}
		}
		btn.OnChanged = func(s string) {
			onChange(s)
			ui.hotkeys.Start()
			if canvas := ui.app.Driver().CanvasForObject(btn); canvas != nil {
				canvas.Focus(nil)
			}
		}
		return btn
	}

	fullScreenButton := createHotkeyButton(ui.config.Hotkeys.FullscreenScreenshot, func(s string) {
		ui.config.Hotkeys.FullscreenScreenshot = s
	})
	currentWindowButton := createHotkeyButton(ui.config.Hotkeys.CurrentWindowScreenshot, func(s string) {
		ui.config.Hotkeys.CurrentWindowScreenshot = s
	})
	captureAreaButton := createHotkeyButton(ui.config.Hotkeys.ScreenSelection, func(s string) {
		ui.config.Hotkeys.ScreenSelection = s
	})
	uploadFileButton := createHotkeyButton(ui.config.Hotkeys.UploadFile, func(s string) {
		ui.config.Hotkeys.UploadFile = s
	})
	uploadClipboardButton := createHotkeyButton(ui.config.Hotkeys.UploadClipboard, func(s string) {
		ui.config.Hotkeys.UploadClipboard = s
	})
	togglePuushButton := createHotkeyButton(ui.config.Hotkeys.Toggle, func(s string) {
		ui.config.Hotkeys.Toggle = s
	})

	rowFullscreen := container.NewGridWithColumns(2, widget.NewLabel("Capture full screen:"), fullScreenButton)
	rowWindow := container.NewGridWithColumns(2, widget.NewLabel("Capture current window:"), currentWindowButton)
	rowArea := container.NewGridWithColumns(2, widget.NewLabel("Capture Area:"), captureAreaButton)
	rowFile := container.NewGridWithColumns(2, widget.NewLabel("Upload File:"), uploadFileButton)
	rowClipboard := container.NewGridWithColumns(2, widget.NewLabel("Upload Clipboard:"), uploadClipboardButton)
	rowToggle := container.NewGridWithColumns(2, widget.NewLabel("Toggle puush functionality:"), togglePuushButton)

	content := container.NewVBox(
		rowFullscreen,
		rowWindow,
		rowArea,
		rowFile,
		rowClipboard,
		rowToggle,
	)

	return container.NewVBox(
		widget.NewSeparator(),
		createGroup("Keyboard Bindings", content),
		widget.NewSeparator(),
	)
}
