package desktop

import (
	"log"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/sundeiii/yeet-client/internal/screenshots"
)

func (ui *UI) buildAdvancedTab(accountViewUpdate func()) fyne.CanvasObject {
	// Screenshot Provider
	var providerNames []string = screenshots.GetProviderList()

	providerSelect := widget.NewSelect(providerNames, func(s string) {
		if s == ui.config.Capture.ScreenshotProvider {
			return
		}

		provider, err := screenshots.GetProviderByName(s)
		if err != nil {
			log.Println("Failed to get screenshot provider: " + err.Error())
			return
		}
		ui.tray.SetScreenshotProvider(provider)
		ui.config.Capture.ScreenshotProvider = s

		if warning := provider.Warning(); warning != "" {
			ui.tray.ShowNotification("Notice", warning)
		}
	})

	// If no provider has been set yet, use the default provider
	if ui.config.Capture.ScreenshotProvider == "" {
		provider, err := screenshots.GetDefaultProvider()
		if err == nil {
			ui.config.Capture.ScreenshotProvider = provider.Name()
		}
	}

	// Select the current provider in the dropdown
	providerSelect.SetSelected(ui.config.Capture.ScreenshotProvider)

	// Screen Capture Quality
	qualityOptions := []string{
		"No Compression",
		"High (JPG)",
		"Medium (JPG)",
	}
	qualityRadio := widget.NewRadioGroup(qualityOptions, func(s string) {
		switch s {
		case "No Compression":
			ui.config.Capture.UploadQuality = screenshots.QualityBest
		case "High (JPG)":
			ui.config.Capture.UploadQuality = screenshots.QualityHigh
		case "Medium (JPG)":
			ui.config.Capture.UploadQuality = screenshots.QualityMedium
		default:
			ui.config.Capture.UploadQuality = screenshots.QualityBest
		}
	})

	switch ui.config.Capture.UploadQuality {
	case screenshots.QualityBest:
		qualityRadio.SetSelected("No Compression")
	case screenshots.QualityHigh:
		qualityRadio.SetSelected("High (JPG)")
	case screenshots.QualityMedium:
		qualityRadio.SetSelected("Medium (JPG)")
	default:
		qualityRadio.SetSelected("No Compression")
	}

	contextMenuCheckbox := widget.NewCheck("Show \"Upload with puush\" in file context menus", ui.UpdateContextMenuConfiguration)
	contextMenuCheckbox.Checked = ui.config.General.ContextMenu
	contextMenuGroup := createGroup("Context Menu", contextMenuCheckbox)

	// Fullscreen Capture
	fullscreenOptions := []string{
		"Capture all screens",
		"Capture screen containing mouse cursor",
		"Always capture primary screen",
	}
	fullscreenRadio := widget.NewRadioGroup(fullscreenOptions, func(s string) {
		switch s {
		case "Capture all screens":
			ui.config.Capture.FullscreenMode = screenshots.FullscreenModeAllScreens
		case "Capture screen containing mouse cursor":
			ui.config.Capture.FullscreenMode = screenshots.FullscreenModeMouse
		case "Always capture primary screen":
			ui.config.Capture.FullscreenMode = screenshots.FullscreenModePrimary
		}
	})

	switch ui.config.Capture.FullscreenMode {
	case screenshots.FullscreenModeAllScreens:
		fullscreenRadio.SetSelected("Capture all screens")
	case screenshots.FullscreenModeMouse:
		fullscreenRadio.SetSelected("Capture screen containing mouse cursor")
	case screenshots.FullscreenModePrimary:
		fullscreenRadio.SetSelected("Always capture primary screen")
	default:
		fullscreenRadio.SetSelected("Capture all screens")
	}

	// Custom Server URL
	serverUrlEntry := widget.NewEntry()
	serverUrlEntry.SetText(ui.config.Misc.ServerURL)
	serverUrlEntry.OnChanged = func(s string) {
		ui.config.Misc.ServerURL = s
	}
	serverUrlEntry.OnSubmitted = func(s string) {
		ui.config.Misc.ServerURL = s
		ui.config.Account.Reset()
		ui.api.SetBaseURL(s)
		ui.api.Account.Credentials.Reset()
		accountViewUpdate()
	}

	return container.NewVScroll(container.NewVBox(
		widget.NewSeparator(),
		createGroup("Screenshot Provider", providerSelect),
		widget.NewSeparator(),
		createGroup("Screen Capture Quality", qualityRadio),
		widget.NewSeparator(),
		contextMenuGroup,
		widget.NewSeparator(),
		createGroup("Fullscreen Capture", fullscreenRadio),
		widget.NewSeparator(),
		createGroup("Server URL", serverUrlEntry),
		widget.NewSeparator(),
	))
}
