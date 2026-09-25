/*
 * Portions of this file are derived from work by Martin Lindhe
 * licensed under the MIT License, see
 * https://github.com/martinlindhe/notify
 */

package notifications

import (
	toast "gopkg.in/toast.v1"
)

func (n *Notification) Push() error {
	note := notification(n.Application, n.Title, n.Text, n.iconPath, n.actionUrl)
	return note.Push()
}

func notification(appName string, title string, text string, iconPath string, actionUrl string) toast.Notification {
	// TODO: Use balloon notifications for windows 7 and below
	return toast.Notification{
		AppID:               appName,
		Title:               title,
		Message:             text,
		Icon:                iconPath,
		ActivationArguments: actionUrl,
	}
}
