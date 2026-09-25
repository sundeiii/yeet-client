//go:build linux

package contextmenu

import (
	"path/filepath"
)

func dolphinPath(context linuxContext) string {
	return filepath.Join(context.dataHome, "kio", "servicemenus", "yeet-upload.desktop")
}

func enableDolphin(context linuxContext, executable string) error {
	content, err := renderContextMenuTemplate(
		"linux-dolphin.desktop.tmpl",
		newContextMenuTemplateData(executable, context.iconPath),
	)
	if err != nil {
		return err
	}
	return writeOwnedFile(dolphinPath(context), content, 0755)
}

func disableDolphin(context linuxContext) error {
	return removeOwnedFile(dolphinPath(context))
}
