//go:build windows || darwin

package tray

import "fyne.io/systray"

// Only Windows and macOS support tray tooltips

func setTrayTooltip(text string) {
	systray.SetTooltip(text)
}
