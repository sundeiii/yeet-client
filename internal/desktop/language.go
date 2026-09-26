package desktop

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/sundeiii/yeet-client/assets"
	"github.com/sundeiii/yeet-client/internal/i18n"
)

// languagePicker is a row of flags to pick the app's language, and a
// button to follow the system's language again.
func (ui *UI) languagePicker() fyne.CanvasObject {
	row := container.NewHBox(widget.NewLabel(i18n.T("Language:")))

	automatic := widget.NewButton(i18n.T("Automatic"), func() { ui.setLanguage("") })
	if ui.config.General.Language == "" {
		automatic.Importance = widget.HighImportance
	}
	row.Add(automatic)

	for _, language := range i18n.Languages {
		code := language.Code
		flag := fyne.NewStaticResource("flag-"+language.Flag+".png", assets.Flags[language.Flag])
		button := widget.NewButtonWithIcon(language.Name, flag, func() { ui.setLanguage(code) })
		if ui.config.General.Language == code {
			button.Importance = widget.HighImportance
		}
		row.Add(button)
	}
	return row
}

// setLanguage switches the app's language and shows the window again in it.
// An empty code follows the system's language.
func (ui *UI) setLanguage(code string) {
	ui.config.General.Language = code
	i18n.SetLanguage(code)
	ui.tray.RebuildMenu()
	// The right-click entry is renamed too
	if ui.config.General.ContextMenu {
		go ui.ReconcileContextMenuConfiguration()
	}

	// The window is rebuilt with the new texts, where it is
	if w := ui.settingsWindow; w != nil {
		if ui.messages != nil {
			ui.messages.close()
		}
		ui.SetUpdateFinishedCallback(nil)
		ui.fillWindow(w, tabSettings)
		ui.settingsTabs.SelectIndex(settingsGeneral)
	} else if w := ui.startupWindow; w != nil {
		w.SetTitle(i18n.T("puush quick start"))
		ui.fillStartupWindow(w)
	}
}
