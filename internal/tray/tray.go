package tray

import (
	"errors"
	"fmt"
	"log"
	"net/url"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"github.com/fsnotify/fsnotify"
	"github.com/sundeiii/yeet-client/assets"
	"github.com/sundeiii/yeet-client/internal/config"
	"github.com/sundeiii/yeet-client/internal/notifications"
	"github.com/sundeiii/yeet-client/internal/screenshots"
	"github.com/sundeiii/yeet-client/pkg/puush"
	"golang.design/x/clipboard"
)

type TrayManager struct {
	api           *puush.Client
	config        *config.Config
	screenshots   screenshots.ScreenshotProvider
	uploadHistory []*puush.HistoryItem

	menu             *fyne.Menu
	targetApp        fyne.App
	settingsCallback func()

	watcher *fsnotify.Watcher

	uploadQueue         chan []string
	uploadQueueStop     chan struct{}
	uploadQueueStart    sync.Once
	uploadQueueStopOnce sync.Once
}

func NewTrayManager(cfg *config.Config, api *puush.Client) *TrayManager {
	provider, _ := screenshots.GetDefaultProvider()
	return &TrayManager{
		api:             api,
		config:          cfg,
		screenshots:     provider,
		uploadQueue:     make(chan []string, 255),
		uploadQueueStop: make(chan struct{}),
	}
}

// SetSettingsCallback will set the function that will be called
// once the "Settings..." action has been invoked
func (m *TrayManager) SetSettingsCallback(callback func()) {
	m.settingsCallback = callback
}

// GetScreenshotProvider returns the screenshot provider used by the tray manager
func (m *TrayManager) GetScreenshotProvider() screenshots.ScreenshotProvider {
	if m.screenshots == nil {
		return nil
	}
	m.screenshots.SetQuality(m.config.Capture.UploadQuality)
	m.screenshots.SetFullscreenMode(m.config.Capture.FullscreenMode)
	return m.screenshots
}

// SetScreenshotProvider sets the screenshot provider for the tray manager
func (m *TrayManager) SetScreenshotProvider(provider screenshots.ScreenshotProvider) {
	m.screenshots = provider
}

// ShowNotification will display a regular notification with a specified title & message
func (m *TrayManager) ShowNotification(title, message string) {
	go notifications.NewNotification(title, "", message).
		WithIconData(assets.PuushIconData).
		Push()
}

// ShowUploadNotification will display a notification indicating that an upload was successful
func (m *TrayManager) ShowUploadNotification(url string) {
	notification := notifications.NewNotification("puush complete!", "", url).
		WithIconData(assets.PuushIconData).
		WithAction(url)

	if m.config.General.NotificationSound {
		notification = notification.WithSoundData(assets.SuccessSoundData)
	}

	notification.Push()
}

// ShowErrorNotification will display an error notification with the provided message
func (m *TrayManager) ShowErrorNotification(message string) {
	go notifications.NewNotification("puush error", "", message).
		WithIconData(assets.PuushErrorIconData).
		Push()
}

// TogglePuushing will toggle the puushing functionality on or off
func (m *TrayManager) TogglePuushing() {
	m.config.General.DisabledToggle = !m.config.General.DisabledToggle
	// The hotkey calls this from its own goroutine; menus must change on the main thread
	fyne.Do(m.rebuildMenuItems)

	if m.config.General.DisabledToggle {
		m.ShowNotification("puush was disabled!", "Shortcut keys will no longer be accepted.")
	} else {
		m.ShowNotification("puush was enabled!", "Shortcut keys will now be accepted.")
	}
}

// PuushingDisabled returns whether puushing is currently disabled
func (m *TrayManager) PuushingDisabled() bool {
	return m.config.General.DisabledToggle
}

// Refresh will instruct the tray to update its menu.
func (m *TrayManager) Refresh() error {
	if m.menu == nil {
		return errors.New("tray was not initialized")
	}
	m.menu.Refresh()
	return nil
}

// Apply applies the tray menu to the specified app.
func (m *TrayManager) Apply(app fyne.App) error {
	if m.menu == nil {
		return errors.New("tray was not initialized")
	}
	if desktopApp, ok := app.(desktop.App); ok {
		desktopApp.SetSystemTrayMenu(m.menu)
		m.targetApp = app
		m.ResetTrayIcon()
		return nil
	}
	return errors.New("provided app is not a desktop app")
}

// Initialize populates the system tray menu.
func (m *TrayManager) Initialize(applicationName string) error {
	m.menu = fyne.NewMenu(applicationName)
	m.rebuildMenuItems()

	err := clipboard.Init()
	if err != nil {
		log.Printf("Error initializing clipboard: %v", err)
		m.ShowErrorNotification("Failed to initialize the clipboard. You may encounter issues when using this feature.")
	}
	return nil
}

func (m *TrayManager) buildString() string {
	if m.targetApp == nil {
		return "puush"
	}
	metadata := m.targetApp.Metadata()
	if metadata.Build == 0 || metadata.Version == "" || metadata.Version == "0.0.0" {
		return "puush dev"
	}
	return fmt.Sprintf("puush %s", metadata.Version)
}

func (m *TrayManager) rebuildMenuItems() {
	if m.menu == nil {
		return
	}
	puushVersion := fyne.NewMenuItem(m.buildString(), func() {})
	puushVersion.Disabled = true

	accountSettings := fyne.NewMenuItem("My Account", func() {
		if !m.api.Account.Credentials.HasApiKey() {
			return
		}
		path := fmt.Sprintf("/login/go/?k=%s", *m.api.Account.Credentials.Key)
		accountUrl, _ := url.Parse(m.api.FormatURL(path))
		fyne.CurrentApp().OpenURL(accountUrl)
	})

	items := []*fyne.MenuItem{
		puushVersion,
		accountSettings,
		fyne.NewMenuItemSeparator(),
	}

	// Append the upload history menu items
	items = append(items, m.BuildHistoryMenu()...)
	items = append(items, fyne.NewMenuItemSeparator())

	captureWindow := fyne.NewMenuItem("Capture Current Window", func() {
		go m.UploadWindowScreenshot()
	})
	captureWindow.Icon = windowIcon
	captureDesktop := fyne.NewMenuItem("Capture Desktop", func() {
		go m.UploadDesktopScreenshot()
	})
	captureDesktop.Icon = fullscreenIcon
	captureArea := fyne.NewMenuItem("Capture Area", func() {
		go m.UploadAreaScreenshot()
	})
	captureArea.Icon = selectionIcon
	uploadFile := fyne.NewMenuItem("Upload File", m.UploadFileFromDialog)
	uploadFile.Icon = uploadIcon
	uploadClipboard := fyne.NewMenuItem("Upload Clipboard", m.UploadFromClipboard)
	uploadClipboard.Icon = clipboardIcon

	disablePuushing := fyne.NewMenuItem("Disable puushing", m.TogglePuushing)
	disablePuushing.Checked = m.config.General.DisabledToggle

	settings := fyne.NewMenuItem("Settings...", func() {
		if m.settingsCallback != nil {
			m.settingsCallback()
		}
	})

	items = append(items,
		captureWindow,
		captureDesktop,
		captureArea,
		uploadClipboard,
		uploadFile,
		fyne.NewMenuItemSeparator(),
		disablePuushing,
		settings,
	)
	m.menu.Items = items

	if m.targetApp == nil {
		return
	}

	// Applying the menu redraws the tray once and adds back the "Quit" button.
	// (menu.Refresh() would redraw it a second time, which could leave
	// duplicate entries behind on Windows.)
	if desktopApp, ok := m.targetApp.(desktop.App); ok {
		desktopApp.SetSystemTrayMenu(m.menu)
	}
}
