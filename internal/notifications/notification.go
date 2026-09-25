package notifications

import "os"

type Notification struct {
	Application string
	Title       string
	Text        string

	iconPath  string
	soundPath string
	actionUrl string
}

func NewNotification(appName string, title string, text string) *Notification {
	return &Notification{
		Application: appName,
		Title:       title,
		Text:        text,
	}
}

func (n *Notification) WithAction(actionUrl string) *Notification {
	n.actionUrl = actionUrl
	return n
}

func (n *Notification) WithIcon(iconPath string) *Notification {
	n.iconPath = iconPath
	return n
}

func (n *Notification) WithSound(soundPath string) *Notification {
	n.soundPath = soundPath
	return n
}

func (n *Notification) WithIconData(iconData []byte) *Notification {
	// Create temporary file to store the icon data
	tmpFile, err := os.CreateTemp("", "notify-icon-*.png")
	if err != nil {
		return n
	}
	defer tmpFile.Close()

	// Write the icon data to the temporary file
	if _, err := tmpFile.Write(iconData); err != nil {
		return n
	}

	return n.WithIcon(tmpFile.Name())
}

func (n *Notification) WithSoundData(soundData []byte) *Notification {
	// Create temporary file to store the sound data
	tmpFile, err := os.CreateTemp("", "notify-sound-*.ogg")
	if err != nil {
		return n
	}
	defer tmpFile.Close()

	// Write the sound data to the temporary file
	if _, err := tmpFile.Write(soundData); err != nil {
		return n
	}

	return n.WithSound(tmpFile.Name())
}
