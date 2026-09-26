//go:build windows

package contextmenu

import (
	"errors"
	"fmt"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// Windows has 2 ways to add explorer context-menu commands:
// - "Legacy" context-menu commands, registered through the registry, such as
//   HKCU\Software\Classes\*\shell
// - Windows 11's modern context menu, which requires an IExplorerCommand
//   implementation in a native COM DLL, registered through a package manifest
//   No clue how to do that right now, so ...
//
// TODO: Add windows 11 context menu integration

type windowsContextMenu struct {
	root registry.Key
	path string
}

func applyPlatform(executable string, enabled bool) error {
	installer := windowsContextMenu{
		root: registry.CURRENT_USER,
		path: `Software\Classes\*\shell\yeet-upload`,
	}
	if enabled {
		return installer.enable(executable)
	}
	return installer.disable()
}

func (installer windowsContextMenu) enable(executable string) error {
	menu, _, err := registry.CreateKey(installer.root, installer.path, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("create Explorer context-menu key: %w", err)
	}
	defer menu.Close()

	values := map[string]string{
		"MUIVerb":          menuLabel(),
		"Icon":             windows.EscapeArg(executable) + ",0",
		"MultiSelectModel": "Player",
	}
	for name, value := range values {
		if err := menu.SetStringValue(name, value); err != nil {
			return fmt.Errorf("set explorer context-menu value %s: %w", name, err)
		}
	}

	command, _, err := registry.CreateKey(installer.root, installer.path+`\command`, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("create explorer context-menu command: %w", err)
	}
	defer command.Close()

	commandLine := windows.EscapeArg(executable) + ` -upload "%1"`
	if err := command.SetStringValue("", commandLine); err != nil {
		return fmt.Errorf("set explorer context-menu command: %w", err)
	}
	return nil
}

func (installer windowsContextMenu) disable() error {
	if err := registry.DeleteKey(installer.root, installer.path+`\command`); err != nil && !errors.Is(err, registry.ErrNotExist) {
		return fmt.Errorf("remove explorer context-menu command: %w", err)
	}
	if err := registry.DeleteKey(installer.root, installer.path); err != nil && !errors.Is(err, registry.ErrNotExist) {
		return fmt.Errorf("remove explorer context-menu key: %w", err)
	}
	return nil
}
