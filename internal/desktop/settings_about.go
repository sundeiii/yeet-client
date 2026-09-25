package desktop

import (
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

func (ui *UI) buildAboutTab() fyne.CanvasObject {
	metadata := ui.app.Metadata()

	logo := canvas.NewImageFromResource(puushIcon)
	logo.FillMode = canvas.ImageFillContain
	logo.SetMinSize(fyne.NewSquareSize(72))

	appName := widget.NewLabelWithStyle(
		metadata.Name,
		fyne.TextAlignCenter,
		fyne.TextStyle{Bold: true},
	)
	header := container.NewVBox(
		container.NewCenter(logo),
		appName,
	)

	details := container.NewVBox(
		widget.NewLabelWithStyle(
			"Build Information",
			fyne.TextAlignLeading,
			fyne.TextStyle{Bold: true},
		),
		widget.NewForm(
			widget.NewFormItem(
				"Version",
				readOnlyEntry(metadata.Version),
			),
			widget.NewFormItem(
				"Build",
				readOnlyEntry(strconv.Itoa(metadata.Build)),
			),
			widget.NewFormItem(
				"Commit",
				readOnlyEntry(metadata.Custom["commit"]),
			),
		),
	)

	content := container.NewVBox(
		header,
		widget.NewSeparator(),
		details,
	)
	return container.NewPadded(
		createGroupNoIndent("", content),
	)
}

func readOnlyEntry(value string) *widget.Entry {
	entry := widget.NewEntry()
	entry.SetText(value)
	entry.Disable()
	return entry
}
