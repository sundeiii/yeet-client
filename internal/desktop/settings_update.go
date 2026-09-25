package desktop

import (
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/sundeiii/yeet-client/internal/updater"
)

func (ui *UI) buildUpdateTab() fyne.CanvasObject {
	autoUpdateCheckbox := widget.NewCheck("Automatically check for updates", func(enabled bool) {
		ui.config.General.AutoUpdate = enabled
	})
	autoUpdateCheckbox.Checked = ui.config.General.AutoUpdate

	var checkButton *widget.Button
	checkForUpdates := func(branch *updater.Branch) {
		checkButton.SetText("Checking...")
		checkButton.Disable()
		if !ui.RequestUpdateCheck(branch) {
			checkButton.SetText("Check for Updates")
			checkButton.Enable()
			ui.ShowNotification("Update check in progress", "Another update check is already running.")
		}
	}

	branchSelectInitialized := false
	branchSelect := widget.NewSelect(
		[]string{
			updater.BranchStable.String(),
			updater.BranchNightly.String(),
		},
		func(selected string) {
			if !branchSelectInitialized {
				return
			}
			branch := updater.NewBranchFromString(selected)
			checkForUpdates(&branch)
		},
	)
	branchSelect.SetSelected(ui.config.General.UpdateBranch.String())
	branchSelectInitialized = true

	checkButton = widget.NewButton("Check for Updates", func() {
		branch := ui.config.General.UpdateBranch
		checkForUpdates(&branch)
	})
	updateChannel := container.NewGridWithColumns(2, branchSelect, checkButton)

	lastCheckedLabel := widget.NewLabel(formatLastUpdateCheck(ui.config.Misc.LastUpdate))
	ui.SetUpdateFinishedCallback(func(checkedAt time.Time) {
		checkButton.SetText("Check for Updates")
		checkButton.Enable()
		lastCheckedLabel.SetText(formatLastUpdateCheck(checkedAt))
	})
	updateInformation := widget.NewForm(
		widget.NewFormItem("Last Checked:", lastCheckedLabel),
	)
	updateManagement := container.NewGridWithColumns(2, autoUpdateCheckbox, updateInformation)

	return container.NewVBox(
		widget.NewSeparator(),
		createGroup("Update Management", updateManagement),
		widget.NewSeparator(),
		createGroup("Update Channel", updateChannel),
		widget.NewSeparator(),
	)
}

func formatLastUpdateCheck(checkedAt time.Time) string {
	if checkedAt.IsZero() {
		return "Never"
	}
	return checkedAt.Local().Format(time.DateTime)
}
