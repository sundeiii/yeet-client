//go:build linux

package contextmenu

import (
	"path/filepath"
)

func nautilusPath(context linuxContext) string {
	return filepath.Join(context.dataHome, "nautilus", "scripts", menuLabel)
}

func enableNautilus(context linuxContext, executable string) error {
	content, err := renderContextMenuTemplate(
		"linux-nautilus.sh.tmpl",
		newContextMenuTemplateData(executable, context.iconPath),
	)
	if err != nil {
		return err
	}
	return writeOwnedFile(nautilusPath(context), content, 0755)
}

func disableNautilus(context linuxContext) error {
	return removeOwnedFile(nautilusPath(context))
}
