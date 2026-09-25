package tray

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// ShowInFolder opens the folder that contains a file, with the file selected
// where the file manager supports that.
func ShowInFolder(path string) error {
	if _, err := os.Stat(path); err != nil {
		return err
	}

	switch runtime.GOOS {
	case "windows":
		return showInExplorer(path)
	case "darwin":
		return exec.Command("open", "-R", path).Start()
	default:
		// Most Linux file managers can select the file over D-Bus
		err := exec.Command("dbus-send", "--session", "--print-reply",
			"--dest=org.freedesktop.FileManager1", "/org/freedesktop/FileManager1",
			"org.freedesktop.FileManager1.ShowItems",
			"array:string:file://"+filepath.ToSlash(path), "string:").Run()
		if err == nil {
			return nil
		}
		return exec.Command("xdg-open", filepath.Dir(path)).Start()
	}
}
