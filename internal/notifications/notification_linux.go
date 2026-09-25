/*
 * Portions of this file are derived from work by Martin Lindhe
 * licensed under the MIT License, see
 * https://github.com/martinlindhe/notify
 */

package notifications

import (
	"os/exec"
)

func (n *Notification) Push() error {
	notification(n.Application, n.Title, n.Text, n.iconPath, n.actionUrl)

	// Attempt to play sound if specified
	if n.soundPath != "" {
		exec.Command("paplay", n.soundPath).Run()
	}
	return nil
}

func notification(appName string, title string, text string, iconPath string, actionUrl string) {
	if actionUrl == "" {
		cmd := exec.Command("notify-send", "-a", appName, "-i", iconPath, title, text)
		cmd.Run()
		return
	}

	go func() {
		cmd := exec.Command("notify-send", "-a", appName, "-i", iconPath, "--action=open=Open", title, "--expire-time=5000", text)
		out, err := cmd.Output()
		if err != nil {
			return
		}
		if string(out) != "open\n" {
			return
		}
		exec.Command("xdg-open", actionUrl).Run()
	}()
}
