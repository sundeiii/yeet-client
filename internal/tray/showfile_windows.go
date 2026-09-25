//go:build windows

package tray

import (
	"os/exec"
	"syscall"
)

func showInExplorer(path string) error {
	// Explorer wants /select,"path" exactly, which Go's usual argument quoting
	// doesn't produce for paths with spaces
	cmd := exec.Command("explorer.exe")
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: `explorer.exe /select,"` + path + `"`}
	return cmd.Start()
}
