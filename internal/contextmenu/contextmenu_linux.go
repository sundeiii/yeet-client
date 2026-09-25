//go:build linux

package contextmenu

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/sundeiii/yeet-client/assets"
)

// Linux doesn't have a standardized way to add context menu items to file managers,
// so we have to support each file manager individually

var ErrNoSupportedFileManager = errors.New("no supported file manager was found")

type linuxContext struct {
	dataHome   string
	configHome string
	iconPath   string
	available  func(string) bool
}

type linuxAdapter struct {
	name       string
	executable string
	enable     func(linuxContext, string) error
	disable    func(linuxContext) error
}

var linuxAdapters = []linuxAdapter{
	{name: "Nautilus", executable: "nautilus", enable: enableNautilus, disable: disableNautilus},
	{name: "Dolphin", executable: "dolphin", enable: enableDolphin, disable: disableDolphin},
	{name: "Nemo", executable: "nemo", enable: enableNemo, disable: disableNemo},
	// TODO: Thunar & other file managers
}

func applyPlatform(executable string, enabled bool) error {
	context, err := newLinuxContext()
	if err != nil {
		return err
	}
	return applyLinux(context, executable, enabled)
}

func newLinuxContext() (linuxContext, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return linuxContext{}, fmt.Errorf("find home directory: %w", err)
	}

	dataHome := os.Getenv("XDG_DATA_HOME")
	if !filepath.IsAbs(dataHome) {
		dataHome = filepath.Join(home, ".local", "share")
	}
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if !filepath.IsAbs(configHome) {
		configHome = filepath.Join(home, ".config")
	}

	// Ensure the puush icon exists for the context-menu icon
	iconPath, _ := ensurePuushIcon(dataHome)

	return linuxContext{
		dataHome:   dataHome,
		configHome: configHome,
		iconPath:   iconPath,
		available: func(name string) bool {
			_, err := exec.LookPath(name)
			return err == nil
		},
	}, nil
}

func applyLinux(context linuxContext, executable string, enabled bool) error {
	found := false
	var failures []error

	// Try to enable / disable the context menu for each supported file manager
	for _, adapter := range linuxAdapters {
		if enabled && !context.available(adapter.executable) {
			continue
		}
		found = true

		var err error
		if enabled {
			err = adapter.enable(context, executable)
		} else {
			err = adapter.disable(context)
		}
		if err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", adapter.name, err))
		}
	}

	if enabled && !found {
		return ErrNoSupportedFileManager
	}
	return errors.Join(failures...)
}

func ensurePuushIcon(dataHome string) (string, error) {
	// Icon will be stored in ~/.local/share/icons/hicolor/64x64/apps/puush.png
	iconDir := filepath.Join(dataHome, "icons", "hicolor", "64x64", "apps")
	iconPath := filepath.Join(iconDir, "puush.png")

	if err := os.MkdirAll(iconDir, 0o755); err != nil {
		return "", fmt.Errorf("create icon directory: %w", err)
	}
	if _, err := os.Stat(iconPath); errors.Is(err, os.ErrNotExist) {
		// Icon doesn't exist -> write it
		if err := os.WriteFile(iconPath, assets.PuushIconData, 0o644); err != nil {
			return "", fmt.Errorf("write icon file: %w", err)
		}
	}
	return iconPath, nil
}
