//go:build linux

package contextmenu

import (
	"path/filepath"
)

func nautilusPath(context linuxContext, label string) string {
	return filepath.Join(context.dataHome, "nautilus", "scripts", label)
}

func enableNautilus(context linuxContext, executable string) error {
	content, err := renderContextMenuTemplate(
		"linux-nautilus.sh.tmpl",
		newContextMenuTemplateData(executable, context.iconPath),
	)
	if err != nil {
		return err
	}
	// The script's file name is the label: remove ones in other languages
	for _, label := range allMenuLabels() {
		if label != menuLabel() {
			removeOwnedFile(nautilusPath(context, label))
		}
	}
	return writeOwnedFile(nautilusPath(context, menuLabel()), content, 0755)
}

func disableNautilus(context linuxContext) error {
	var failed error
	for _, label := range allMenuLabels() {
		if err := removeOwnedFile(nautilusPath(context, label)); err != nil {
			failed = err
		}
	}
	return failed
}
