package desktop

import (
	"log"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/sundeiii/yeet-client/internal/i18n"
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
			ui.tray.ShowNotification(i18n.T("Notice"), i18n.T(warning))
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
	qualityBest, qualityHigh, qualityMedium := i18n.T("No Compression"), i18n.T("High (JPG)"), i18n.T("Medium (JPG)")
	qualityOptions := []string{
		qualityBest,
		qualityHigh,
		qualityMedium,
	}
	qualityRadio := widget.NewRadioGroup(qualityOptions, func(s string) {
		switch s {
		case qualityBest:
			ui.config.Capture.UploadQuality = screenshots.QualityBest
		case qualityHigh:
			ui.config.Capture.UploadQuality = screenshots.QualityHigh
		case qualityMedium:
			ui.config.Capture.UploadQuality = screenshots.QualityMedium
		default:
			ui.config.Capture.UploadQuality = screenshots.QualityBest
		}
	})

	switch ui.config.Capture.UploadQuality {
	case screenshots.QualityBest:
		qualityRadio.SetSelected(qualityBest)
	case screenshots.QualityHigh:
		qualityRadio.SetSelected(qualityHigh)
	case screenshots.QualityMedium:
		qualityRadio.SetSelected(qualityMedium)
	default:
		qualityRadio.SetSelected(qualityBest)
	}

	contextMenuCheckbox := widget.NewCheck(i18n.T("Show \"Upload with puush\" in file context menus"), ui.UpdateContextMenuConfiguration)
	contextMenuCheckbox.Checked = ui.config.General.ContextMenu
	contextMenuGroup := createGroup(i18n.T("Context Menu"), contextMenuCheckbox)

	// Fullscreen Capture
	allScreens := i18n.T("Capture all screens")
	mouseScreen := i18n.T("Capture screen containing mouse cursor")
	primaryScreen := i18n.T("Always capture primary screen")
	fullscreenOptions := []string{
		allScreens,
		mouseScreen,
		primaryScreen,
	}
	fullscreenRadio := widget.NewRadioGroup(fullscreenOptions, func(s string) {
		switch s {
		case allScreens:
			ui.config.Capture.FullscreenMode = screenshots.FullscreenModeAllScreens
		case mouseScreen:
			ui.config.Capture.FullscreenMode = screenshots.FullscreenModeMouse
		case primaryScreen:
			ui.config.Capture.FullscreenMode = screenshots.FullscreenModePrimary
		}
	})

	switch ui.config.Capture.FullscreenMode {
	case screenshots.FullscreenModeAllScreens:
		fullscreenRadio.SetSelected(allScreens)
	case screenshots.FullscreenModeMouse:
		fullscreenRadio.SetSelected(mouseScreen)
	case screenshots.FullscreenModePrimary:
		fullscreenRadio.SetSelected(primaryScreen)
	default:
		fullscreenRadio.SetSelected(allScreens)
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
		createGroup(i18n.T("Screenshot Provider"), providerSelect),
		widget.NewSeparator(),
		createGroup(i18n.T("Screen Capture Quality"), qualityRadio),
		widget.NewSeparator(),
		contextMenuGroup,
		widget.NewSeparator(),
		createGroup(i18n.T("Fullscreen Capture"), fullscreenRadio),
		widget.NewSeparator(),
		createGroup(i18n.T("Server URL"), serverUrlEntry),
		widget.NewSeparator(),
	))
}
