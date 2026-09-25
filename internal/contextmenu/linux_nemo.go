//go:build linux

package contextmenu

import (
	"path/filepath"
)

func nemoPath(context linuxContext) string {
	return filepath.Join(context.dataHome, "nemo", "actions", "yeet-upload.nemo_action")
}

func enableNemo(context linuxContext, executable string) error {
	content, err := renderContextMenuTemplate(
		"linux-nemo.action.tmpl",
		newContextMenuTemplateData(executable, context.iconPath),
	)
	if err != nil {
		return err
	}
	return writeOwnedFile(nemoPath(context), content, 0644)
}

func disableNemo(context linuxContext) error {
	return removeOwnedFile(nemoPath(context))
}
