package desktop

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/sundeiii/yeet-client/internal/i18n"
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
	repeatAreaButton := createHotkeyButton(ui.config.Hotkeys.RepeatArea, func(s string) {
		ui.config.Hotkeys.RepeatArea = s
	})
	delayedAreaButton := createHotkeyButton(ui.config.Hotkeys.DelayedArea, func(s string) {
		ui.config.Hotkeys.DelayedArea = s
	})
	editAreaButton := createHotkeyButton(ui.config.Hotkeys.EditArea, func(s string) {
		ui.config.Hotkeys.EditArea = s
	})
	recordButton := createHotkeyButton(ui.config.Hotkeys.Record, func(s string) {
		ui.config.Hotkeys.Record = s
	})

	rowFullscreen := container.NewGridWithColumns(2, widget.NewLabel(i18n.T("Capture full screen:")), fullScreenButton)
	rowWindow := container.NewGridWithColumns(2, widget.NewLabel(i18n.T("Capture current window:")), currentWindowButton)
	rowArea := container.NewGridWithColumns(2, widget.NewLabel(i18n.T("Capture Area:")), captureAreaButton)
	rowFile := container.NewGridWithColumns(2, widget.NewLabel(i18n.T("Upload File:")), uploadFileButton)
	rowClipboard := container.NewGridWithColumns(2, widget.NewLabel(i18n.T("Upload Clipboard:")), uploadClipboardButton)
	rowToggle := container.NewGridWithColumns(2, widget.NewLabel(i18n.T("Toggle puush functionality:")), togglePuushButton)
	rowRepeatArea := container.NewGridWithColumns(2, widget.NewLabel(i18n.T("Capture last area again:")), repeatAreaButton)
	rowDelayedArea := container.NewGridWithColumns(2, widget.NewLabel(i18n.T("Capture area after a delay:")), delayedAreaButton)
	rowEditArea := container.NewGridWithColumns(2, widget.NewLabel(i18n.T("Capture area and edit:")), editAreaButton)

	content := container.NewVBox(
		rowFullscreen,
		rowWindow,
		rowArea,
		rowEditArea,
		rowRepeatArea,
		rowDelayedArea,
	)
	if ui.tray.RecordingSupported() {
		content.Add(container.NewGridWithColumns(2, widget.NewLabel(i18n.T("Record screen (again to stop):")), recordButton))
	}
	for _, row := range []fyne.CanvasObject{rowFile, rowClipboard, rowToggle} {
		content.Add(row)
	}

	hint := widget.NewLabel(i18n.T("Click a shortcut, then press the new keys. Escape keeps the old one."))
	hint.Wrapping = fyne.TextWrapWord

	return container.NewVScroll(container.NewVBox(
		widget.NewSeparator(),
		createGroup(i18n.T("Keyboard Bindings"), content),
		container.NewPadded(hint),
	))
}
