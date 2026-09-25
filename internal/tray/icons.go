package tray

import (
	"math"
	"time"

	"github.com/sundeiii/yeet-client/assets"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
)

var (
	puushIcon      fyne.Resource = fyne.NewStaticResource("icon-puush.png", assets.PuushIconData)
	windowIcon     fyne.Resource = fyne.NewStaticResource("icon-window.png", assets.WindowIconData)
	fullscreenIcon fyne.Resource = fyne.NewStaticResource("icon-fullscreen.png", assets.FullscreenIconData)
	uploadIcon     fyne.Resource = fyne.NewStaticResource("icon-upload.png", assets.UploadIconData)
	clipboardIcon  fyne.Resource = fyne.NewStaticResource("icon-clipboard.png", assets.ClipboardIconData)
	selectionIcon  fyne.Resource = fyne.NewStaticResource("icon-selection.png", assets.SelectionIconData)
)

var (
	puushTrayIcon         fyne.Resource = fyne.NewStaticResource("tray-icon-puush.ico", assets.TrayIconData)
	puushTrayCompleteIcon fyne.Resource = fyne.NewStaticResource("tray-icon-complete.ico", assets.TrayCompleteIconData)
	puushTrayFailIcon     fyne.Resource = fyne.NewStaticResource("tray-icon-fail.ico", assets.TrayFailIconData)

	puushTrayProgress0Icon   fyne.Resource = fyne.NewStaticResource("tray-icon-progress0.ico", assets.TrayProgress0IconData)
	puushTrayProgress10Icon  fyne.Resource = fyne.NewStaticResource("tray-icon-progress10.ico", assets.TrayProgress10IconData)
	puushTrayProgress20Icon  fyne.Resource = fyne.NewStaticResource("tray-icon-progress20.ico", assets.TrayProgress20IconData)
	puushTrayProgress30Icon  fyne.Resource = fyne.NewStaticResource("tray-icon-progress30.ico", assets.TrayProgress30IconData)
	puushTrayProgress40Icon  fyne.Resource = fyne.NewStaticResource("tray-icon-progress40.ico", assets.TrayProgress40IconData)
	puushTrayProgress50Icon  fyne.Resource = fyne.NewStaticResource("tray-icon-progress50.ico", assets.TrayProgress50IconData)
	puushTrayProgress60Icon  fyne.Resource = fyne.NewStaticResource("tray-icon-progress60.ico", assets.TrayProgress60IconData)
	puushTrayProgress70Icon  fyne.Resource = fyne.NewStaticResource("tray-icon-progress70.ico", assets.TrayProgress70IconData)
	puushTrayProgress80Icon  fyne.Resource = fyne.NewStaticResource("tray-icon-progress80.ico", assets.TrayProgress80IconData)
	puushTrayProgress90Icon  fyne.Resource = fyne.NewStaticResource("tray-icon-progress90.ico", assets.TrayProgress90IconData)
	puushTrayProgress100Icon fyne.Resource = fyne.NewStaticResource("tray-icon-progress100.ico", assets.TrayProgress100IconData)
)

// ResetTrayIcon resets the puush tray icon back to its original state
func (m *TrayManager) ResetTrayIcon() {
	if desktopApp, ok := m.targetApp.(desktop.App); ok {
		desktopApp.SetSystemTrayIcon(puushTrayIcon)
		setTrayTooltip("puush")
	}
}

func (m *TrayManager) ResetTrayIconSoon() {
	time.AfterFunc(2*time.Second, func() {
		fyne.Do(m.ResetTrayIcon)
	})
}

// OnTrayProgressComplete will indicate a successful
// upload & reset the tray icon afterwards
func (m *TrayManager) OnTrayProgressComplete() {
	fyne.Do(func() {
		if desktopApp, ok := m.targetApp.(desktop.App); ok {
			desktopApp.SetSystemTrayIcon(puushTrayCompleteIcon)
			setTrayTooltip("puush: upload complete!")
			m.ResetTrayIconSoon()
		}
	})
}

// OnTrayProgressFail will indicate a failed
// upload & reset the tray icon afterwards
func (m *TrayManager) OnTrayProgressFail() {
	fyne.Do(func() {
		if desktopApp, ok := m.targetApp.(desktop.App); ok {
			desktopApp.SetSystemTrayIcon(puushTrayFailIcon)
			setTrayTooltip("puush: upload failed!")
			m.ResetTrayIconSoon()
		}
	})
}

// OnTrayProgressUpdate fills the tray icon to the given percentage, with the
// tooltip explaining what's going on
func (m *TrayManager) OnTrayProgressUpdate(percentage float64, tooltip string) {
	fyne.Do(func() {
		if desktopApp, ok := m.targetApp.(desktop.App); ok {
			switch roundedPercentage(percentage) {
			case 100:
				desktopApp.SetSystemTrayIcon(puushTrayProgress100Icon)
			case 90:
				desktopApp.SetSystemTrayIcon(puushTrayProgress90Icon)
			case 80:
				desktopApp.SetSystemTrayIcon(puushTrayProgress80Icon)
			case 70:
				desktopApp.SetSystemTrayIcon(puushTrayProgress70Icon)
			case 60:
				desktopApp.SetSystemTrayIcon(puushTrayProgress60Icon)
			case 50:
				desktopApp.SetSystemTrayIcon(puushTrayProgress50Icon)
			case 40:
				desktopApp.SetSystemTrayIcon(puushTrayProgress40Icon)
			case 30:
				desktopApp.SetSystemTrayIcon(puushTrayProgress30Icon)
			case 20:
				desktopApp.SetSystemTrayIcon(puushTrayProgress20Icon)
			case 10:
				desktopApp.SetSystemTrayIcon(puushTrayProgress10Icon)
			default:
				desktopApp.SetSystemTrayIcon(puushTrayProgress0Icon)
			}
			setTrayTooltip(tooltip)
		}
	})
}

// roundedPercentage rounds the given percentage to the nearest 10% increment
func roundedPercentage(percentage float64) int {
	return int(math.Ceil((percentage-5)/10)) * 10
}
