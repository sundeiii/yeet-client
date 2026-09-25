package desktop

import (
	"github.com/sundeiii/yeet-client/internal/i18n"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

func (ui *UI) buildUpdateTab() fyne.CanvasObject {
	autoUpdateCheckbox := widget.NewCheck(i18n.T("Automatically check for updates"), func(enabled bool) {
		ui.config.General.AutoUpdate = enabled
	})
	autoUpdateCheckbox.Checked = ui.config.General.AutoUpdate

	var checkButton *widget.Button
	checkButton = widget.NewButton(i18n.T("Check for Updates"), func() {
		checkButton.SetText(i18n.T("Checking..."))
		checkButton.Disable()
		if !ui.RequestUpdateCheck(nil) {
			checkButton.SetText(i18n.T("Check for Updates"))
			checkButton.Enable()
			ui.ShowNotification(i18n.T("Update check in progress"), i18n.T("Another update check is already running."))
		}
	})

	lastCheckedLabel := widget.NewLabel(formatLastUpdateCheck(ui.config.Misc.LastUpdate))
	ui.SetUpdateFinishedCallback(func(checkedAt time.Time) {
		checkButton.SetText(i18n.T("Check for Updates"))
		checkButton.Enable()
		lastCheckedLabel.SetText(formatLastUpdateCheck(checkedAt))
	})
	updateInformation := widget.NewForm(
		widget.NewFormItem(i18n.T("Last Checked:"), lastCheckedLabel),
	)
	updateManagement := container.NewGridWithColumns(2, autoUpdateCheckbox, updateInformation)

	return container.NewVBox(
		widget.NewSeparator(),
		createGroup(i18n.T("Update Management"), container.NewVBox(updateManagement, checkButton)),
		widget.NewSeparator(),
	)
}

func formatLastUpdateCheck(checkedAt time.Time) string {
	if checkedAt.IsZero() {
		return i18n.T("Never")
	}
	return checkedAt.Local().Format(time.DateTime)
}
