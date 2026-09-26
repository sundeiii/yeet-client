package desktop

import (
	"github.com/sundeiii/yeet-client/internal/i18n"
	"log"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"

	"github.com/sundeiii/yeet-client/assets"
	"github.com/sundeiii/yeet-client/internal/config"
	"github.com/sundeiii/yeet-client/internal/hotkeys"
	appipc "github.com/sundeiii/yeet-client/internal/ipc"
	"github.com/sundeiii/yeet-client/internal/screenshots"
	"github.com/sundeiii/yeet-client/internal/tray"
	"github.com/sundeiii/yeet-client/internal/updater"
	"github.com/sundeiii/yeet-client/pkg/puush"
)

var puushIcon fyne.Resource = fyne.NewStaticResource("icon-puush.png", assets.PuushIconData)

// UI manages the desktop application windows and state.
type UI struct {
	app     fyne.App
	api     *puush.Client
	config  *config.Config
	tray    *tray.TrayManager
	hotkeys *hotkeys.HotkeyManager
	ipc     *appipc.Server

	settingsWindow fyne.Window
	startupWindow  fyne.Window

	// Parts of the app window, while it's open
	windowTabs  *container.AppTabs
	refreshHome func()
	uploads     *uploadsView
	queue       *queueView
	messages    *messagesView

	// The account's profile and avatar, once the server told them
	profile *puush.Profile
	avatar  fyne.Resource

	requestUpdateCheck  func(branch *updater.Branch) bool
	updateCheckFinished func(time.Time)
}

func NewUI(app fyne.App, api *puush.Client, cfg *config.Config) *UI {
	tm := tray.NewTrayManager(cfg, api)
	hkm := hotkeys.NewHotkeyManager(cfg, tm)

	return &UI{
		app:     app,
		api:     api,
		config:  cfg,
		tray:    tm,
		hotkeys: hkm,
	}
}

func (ui *UI) Run() {
	// TODO: Maybe add some sort of theme customization?
	ui.app.Settings().SetTheme(NewWindowsTheme())

	// Update autostart configuration based on current settings
	ui.UpdateAutostartConfiguration(ui.config.General.Startup)
	ui.ReconcileContextMenuConfiguration()

	// Show quickstart window if no credentials have been set
	// Otherwise, re-authenticate to see if the API key is still valid
	if !ui.api.Account.Credentials.HasApiKey() {
		ui.ShowStartupWindow()
	}

	// Initialize & start tray
	if ui.tray != nil {
		ui.tray.Initialize("puush")
		ui.tray.Apply(ui.app)
		ui.tray.SetSettingsCallback(ui.ShowSettingsWindow)
		ui.tray.SetOpenCallback(ui.ShowAppWindow)
		ui.tray.OnChange(ui.onTrayChange)
		ui.tray.SetQueueCallback(ui.ShowQueueWindow)
		ui.tray.SetMessagesCallback(ui.ShowMessagesWindow)
		ui.tray.OnQueueChange(func() {
			if ui.queue != nil {
				ui.queue.refresh()
			}
		})
		ui.tray.StartUploadQueue()
		ui.tray.StartHistoryRefresh()
		ui.startIPC()

		// Start directory monitoring
		if len(ui.config.Capture.MonitorDirectories) > 0 {
			ui.tray.StartMonitor(ui.config.Capture.MonitorDirectories)
		}

		go ui.tray.PerformBackgroundAuthentication()
		go ui.tray.RefreshHistory()
		go ui.tray.RefreshPools()
		ui.tray.StartLive()

		// Setup screenshot provider
		providerName := ui.config.Capture.ScreenshotProvider
		provider, err := screenshots.GetProviderByName(providerName)
		if err == nil {
			ui.tray.SetScreenshotProvider(provider)
		}

		if ui.tray.GetScreenshotProvider() == nil {
			log.Println("No valid screenshot provider could be set")
			ui.tray.ShowErrorNotification(i18n.T("Could not find a screenshot provider. Screenshots may not work properly."))
		}

		ui.hotkeys.Start()
	}

	ui.app.Run()
}

func (ui *UI) Quit() {
	ui.app.Quit()
}

func (ui *UI) OnShutdown() {
	ui.tray.StopMonitor()
	ui.tray.StopUploadQueue()
	ui.tray.StopLive()
	ui.saveAccount()
	ui.CloseIPCServer()
}

func (ui *UI) SetIPCServer(server *appipc.Server) {
	ui.ipc = server
}

func (ui *UI) SetUpdateCheckCallback(callback func(branch *updater.Branch) bool) {
	ui.requestUpdateCheck = callback
}

func (ui *UI) RequestUpdateCheck(branch *updater.Branch) bool {
	callback := ui.requestUpdateCheck
	if callback == nil {
		return false
	}
	return callback(branch)
}

func (ui *UI) SetUpdateFinishedCallback(callback func(time.Time)) {
	ui.updateCheckFinished = callback
}

func (ui *UI) FinishUpdateCheck(checkedAt time.Time) {
	callback := ui.updateCheckFinished
	if callback == nil {
		return
	}
	fyne.Do(func() { callback(checkedAt) })
}

func (ui *UI) CloseIPCServer() {
	if ui.ipc != nil {
		ui.ipc.Close()
	}
}

func (ui *UI) ShowNotification(title, message string) {
	if ui.tray != nil {
		ui.tray.ShowNotification(title, message)
	}
}

// saveAccount copies the logged in account into the settings.
func (ui *UI) saveAccount() {
	if ui.api.Account.Credentials.HasApiKey() {
		ui.config.Account.Key = *ui.api.Account.Credentials.Key
		ui.config.Account.Username = *ui.api.Account.Credentials.Identifier
		ui.config.Account.Type = ui.api.Account.Type
		ui.config.Account.Usage = ui.api.Account.DiskUsage

		if ui.api.Account.SubscriptionEnd != nil {
			ui.config.Account.Expiry = ui.api.Account.SubscriptionEnd.Format(time.DateTime)
		}
	}
}

// UpdateAccountConfiguration saves the logged in account and loads what
// belongs to it: uploads, pools, chats and the profile.
func (ui *UI) UpdateAccountConfiguration() {
	ui.saveAccount()
	if ui.api.Account.Credentials.HasApiKey() {
		// Both ask the server, so not on the main thread
		go ui.tray.RefreshHistory()
		go ui.tray.RefreshPools()
		// Chats and comments of this account (again, after logging in)
		ui.tray.StartLive()
		ui.tray.RebuildMenu()
		go ui.refreshProfile()
	}
}

// Logout forgets the account on this computer.
func (ui *UI) Logout() {
	ui.config.Account.Reset()
	ui.api.Account.Reset()
	ui.tray.StopLive()
	ui.tray.ResetAccountState()
	ui.profile = nil
	ui.avatar = nil
	if ui.messages != nil {
		ui.messages.loggedOut()
	}
}

func (ui *UI) UpdateAutostartConfiguration(enabled bool) {
	ui.config.General.Startup = enabled

	if IsDevelopmentBuild() {
		return
	}

	if enabled {
		err := EnableAutostart()
		if err != nil {
			log.Printf("Failed to enable autostart: %v", err)
		}
	} else {
		err := DisableAutostart()
		if err != nil {
			log.Printf("Failed to disable autostart: %v", err)
		}
	}
}

func (ui *UI) activeWindow() fyne.Window {
	if ui.settingsWindow != nil {
		return ui.settingsWindow
	}
	return ui.startupWindow
}

func (ui *UI) showRelevantWindow() {
	// Check if a window is already active & focus it
	if window := ui.activeWindow(); window != nil {
		window.Show()
		window.RequestFocus()
		return
	}
	// Show the app window otherwise
	ui.ShowAppWindow()
}

func (ui *UI) startIPC() {
	if ui.ipc == nil {
		return
	}
	if ui.tray == nil {
		return
	}

	go func() {
		for {
			select {
			case command := <-ui.ipc.Incoming():
				switch command.Action {
				case appipc.ActionUpload:
					if err := ui.tray.EnqueueFiles(command.UploadPaths); err != nil {
						ui.tray.OnUploadError(err)
					}
				case appipc.ActionChooseFile:
					ui.tray.UploadFileFromDialog()
				case appipc.ActionToggleShortcuts:
					ui.tray.TogglePuushing()
				case appipc.ActionAttention:
					fyne.Do(ui.showRelevantWindow)
				}
			case <-ui.ipc.Done():
				return
			}
		}
	}()
}
