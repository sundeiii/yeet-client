package tray

import (
	"context"
	"errors"
	"fmt"
	"github.com/sundeiii/yeet-client/internal/i18n"
	"log"
	"net/url"
	"sync"
	"sync/atomic"

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
	openCallback     func()
	queueCallback    func()
	messagesCallback func()

	watcher *fsnotify.Watcher

	uploadQueue         chan uploadBatch
	uploadQueueStop     chan struct{}
	uploadQueueStart    sync.Once
	uploadQueueStopOnce sync.Once

	// Guards the fields below, which upload goroutines change
	mu           sync.Mutex
	activeUpload *uploadJob
	cancelUpload context.CancelFunc
	failed       []*uploadJob
	pools        []*puush.Pool
	listeners    []func()

	queue          []*QueueEntry
	queueSeq       int
	queueListeners []func()

	pendingDir   string
	countingDown atomic.Bool

	live liveState
}

func NewTrayManager(cfg *config.Config, api *puush.Client) *TrayManager {
	provider, _ := screenshots.GetDefaultProvider()
	return &TrayManager{
		api:             api,
		config:          cfg,
		screenshots:     provider,
		uploadQueue:     make(chan uploadBatch, 255),
		uploadQueueStop: make(chan struct{}),
		pendingDir:      config.PendingDir(),
	}
}

// SetSettingsCallback will set the function that will be called
// once the "Settings..." action has been invoked
func (m *TrayManager) SetSettingsCallback(callback func()) {
	m.settingsCallback = callback
}

// SetOpenCallback sets the function that opens the app's window, for "Open puush".
func (m *TrayManager) SetOpenCallback(callback func()) {
	m.openCallback = callback
}

// SetQueueCallback sets the function that opens the upload queue window.
func (m *TrayManager) SetQueueCallback(callback func()) {
	m.queueCallback = callback
}

// OnChange registers a function that's called (on the main thread) when
// uploads, failed uploads or pools change, so windows can update.
func (m *TrayManager) OnChange(listener func()) {
	m.mu.Lock()
	m.listeners = append(m.listeners, listener)
	m.mu.Unlock()
}

func (m *TrayManager) notifyListeners() {
	m.mu.Lock()
	listeners := append([]func(){}, m.listeners...)
	m.mu.Unlock()
	for _, listener := range listeners {
		listener()
	}
}

// ResetAccountState forgets the logged out account's uploads and pools.
func (m *TrayManager) ResetAccountState() {
	m.mu.Lock()
	m.pools = nil
	m.mu.Unlock()
	fyne.Do(func() {
		m.uploadHistory = nil
		m.rebuildMenuItems()
		m.notifyListeners()
	})
}

// RebuildMenu updates the tray menu after a setting it shows has changed.
func (m *TrayManager) RebuildMenu() {
	m.stateChanged()
}

// stateChanged rebuilds the menu and tells the listeners, from any goroutine.
func (m *TrayManager) stateChanged() {
	fyne.Do(func() {
		m.rebuildMenuItems()
		m.notifyListeners()
	})
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

// ShowUploadNotification tells the user an upload is done, with a sound and
// a notification as the settings say. preview is a picture of the upload for
// the notification, if there is one. duplicate says the file was uploaded
// before, so the link is the old one.
func (m *TrayManager) ShowUploadNotification(url string, preview []byte, duplicate bool) {
	sound := m.config.General.Sound
	if m.config.General.NotifySuccess {
		icon := assets.PuushIconData
		if m.config.General.NotifyPreview && len(preview) > 0 {
			icon = preview
		}
		title := i18n.T("puush complete!")
		if duplicate {
			title = i18n.T("Already uploaded, same link as before")
		}
		notification := notifications.NewNotification(title, "", url).
			WithIconData(icon).
			WithAction(url)
		// The app plays its own sound (or none); only "system" leaves it to the system
		if sound != notifications.SoundSystem {
			notification = notification.Silent()
		}
		notification.Push()
	}
	if sound != notifications.SoundSystem && sound != notifications.SoundNone {
		notifications.PlaySound(sound)
	}
}

// ShowErrorNotification will display an error notification with the provided message
func (m *TrayManager) ShowErrorNotification(message string) {
	go notifications.NewNotification(i18n.T("puush error"), "", message).
		WithIconData(assets.PuushErrorIconData).
		Push()
}

// TogglePuushing will toggle the puushing functionality on or off
func (m *TrayManager) TogglePuushing() {
	m.config.General.DisabledToggle = !m.config.General.DisabledToggle
	// The hotkey calls this from its own goroutine; menus must change on the main thread
	m.stateChanged()

	if m.config.General.DisabledToggle {
		m.ShowNotification(i18n.T("puush was disabled!"), i18n.T("Shortcut keys will no longer be accepted."))
	} else {
		m.ShowNotification(i18n.T("puush was enabled!"), i18n.T("Shortcut keys will now be accepted."))
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
	m.loadFailed()
	m.menu = fyne.NewMenu(applicationName)
	m.rebuildMenuItems()

	err := clipboard.Init()
	if err != nil {
		log.Printf("Error initializing clipboard: %v", err)
		m.ShowErrorNotification(i18n.T("Failed to initialize the clipboard. You may encounter issues when using this feature."))
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

	openApp := fyne.NewMenuItem(i18n.T("Open puush"), func() {
		if m.openCallback != nil {
			m.openCallback()
		}
	})

	accountSettings := fyne.NewMenuItem(i18n.T("My Account"), func() {
		if !m.api.Account.Credentials.HasApiKey() {
			return
		}
		// Fetching the one-time login link is a network call; open it once it's there
		go func() {
			accountUrl, err := url.Parse(m.api.AccountLink())
			if err != nil {
				return
			}
			fyne.Do(func() { fyne.CurrentApp().OpenURL(accountUrl) })
		}()
	})

	items := []*fyne.MenuItem{
		puushVersion,
		openApp,
		accountSettings,
	}
	if m.api.Account.Credentials.HasApiKey() {
		label := i18n.T("Messages")
		if unread := m.Unread(); unread > 0 {
			label = i18n.T("Messages (%d unread)", unread)
		}
		items = append(items, fyne.NewMenuItem(label, func() {
			if m.messagesCallback != nil {
				m.messagesCallback()
			}
		}))
	}

	if name, uploading := m.ActiveUpload(); uploading {
		items = append(items, fyne.NewMenuItem(i18n.T("Cancel Upload (%s)", escapeMenuLabel(name)), m.CancelUpload))
	}
	if failed := m.buildFailedMenu(); failed != nil {
		items = append(items, failed)
	}
	items = append(items, fyne.NewMenuItemSeparator())

	// Append the upload history menu items
	items = append(items, m.BuildHistoryMenu()...)
	items = append(items, fyne.NewMenuItemSeparator())

	captureWindow := fyne.NewMenuItem(i18n.T("Capture Current Window"), func() {
		go m.UploadWindowScreenshot()
	})
	captureWindow.Icon = windowIcon
	captureDesktop := fyne.NewMenuItem(i18n.T("Capture Desktop"), func() {
		go m.UploadDesktopScreenshot()
	})
	captureDesktop.Icon = fullscreenIcon
	captureArea := fyne.NewMenuItem(i18n.T("Capture Area"), func() {
		go m.UploadAreaScreenshot()
	})
	captureArea.Icon = selectionIcon
	editArea := fyne.NewMenuItem(i18n.T("Capture Area and Edit"), func() {
		go m.EditAreaScreenshot()
	})
	editArea.Icon = selectionIcon
	captureLastArea := fyne.NewMenuItem(i18n.T("Capture Last Area Again"), func() {
		go m.UploadLastAreaScreenshot()
	})
	captureLastArea.Icon = selectionIcon
	captureLastArea.Disabled = !m.HasLastArea()

	seconds := int(m.config.Capture.Delay().Seconds())
	delayed := fyne.NewMenuItem(i18n.T("Capture in %d Seconds", seconds), nil)
	delayed.ChildMenu = fyne.NewMenu("",
		fyne.NewMenuItem(i18n.T("Area"), func() { go m.DelayedAreaScreenshot() }),
		fyne.NewMenuItem(i18n.T("Desktop"), func() { go m.DelayedDesktopScreenshot() }),
		fyne.NewMenuItem(i18n.T("Current Window"), func() { go m.DelayedWindowScreenshot() }),
	)

	uploadFile := fyne.NewMenuItem(i18n.T("Upload File"), m.UploadFileFromDialog)
	uploadFile.Icon = uploadIcon
	uploadClipboard := fyne.NewMenuItem(i18n.T("Upload Clipboard"), func() {
		go m.UploadFromClipboard()
	})
	uploadClipboard.Icon = clipboardIcon
	uploadText := fyne.NewMenuItem(i18n.T("Upload Text"), func() {
		go m.UploadText()
	})
	uploadText.Icon = clipboardIcon

	queueWindow := fyne.NewMenuItem(i18n.T("Upload Queue..."), func() {
		if m.queueCallback != nil {
			m.queueCallback()
		}
	})

	disablePuushing := fyne.NewMenuItem(i18n.T("Disable puushing"), m.TogglePuushing)
	disablePuushing.Checked = m.config.General.DisabledToggle

	settings := fyne.NewMenuItem(i18n.T("Settings..."), func() {
		if m.settingsCallback != nil {
			m.settingsCallback()
		}
	})

	items = append(items,
		captureWindow,
		captureDesktop,
		captureArea,
		editArea,
		captureLastArea,
		delayed,
		uploadClipboard,
		uploadText,
		uploadFile,
		queueWindow,
		fyne.NewMenuItemSeparator(),
	)
	if pools := m.buildPoolMenu(); pools != nil {
		items = append(items, pools)
	}
	items = append(items, disablePuushing, settings)
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

// buildFailedMenu lists the uploads that are waiting to be retried.
func (m *TrayManager) buildFailedMenu() *fyne.MenuItem {
	names := m.FailedUploads()
	if len(names) == 0 {
		return nil
	}

	var items []*fyne.MenuItem
	for i, name := range names {
		if i == 10 {
			more := fyne.NewMenuItem(i18n.T("and %d more", len(names)-10), func() {})
			more.Disabled = true
			items = append(items, more)
			break
		}
		item := fyne.NewMenuItem(escapeMenuLabel(name), func() {})
		item.Disabled = true
		items = append(items, item)
	}
	items = append(items,
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem(i18n.T("Retry All"), func() { go m.RetryFailedUploads() }),
		fyne.NewMenuItem(i18n.T("Discard All"), func() { go m.DiscardFailedUploads() }),
	)

	menu := fyne.NewMenuItem(i18n.T("Failed Uploads (%d)", len(names)), nil)
	menu.ChildMenu = fyne.NewMenu("", items...)
	return menu
}
