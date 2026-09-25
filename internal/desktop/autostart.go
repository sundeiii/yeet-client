package desktop

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/emersion/go-autostart"
	"github.com/sundeiii/yeet-client/assets"
)

func EnableAutostart() error {
	app, err := getAutostartApp()
	if err != nil {
		return err
	}
	if app == nil {
		return fmt.Errorf("autostart is not supported on this platform")
	}

	if app.IsEnabled() {
		return nil
	}
	return app.Enable()
}

func DisableAutostart() error {
	app, err := getAutostartApp()
	if err != nil {
		return err
	}

	if !app.IsEnabled() {
		return nil
	}
	return app.Disable()
}

func getAutostartApp() (*autostart.App, error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("get executable path: %w", err)
	}

	if runtime.GOOS == "darwin" {
		// Check if we are inside an app bundle
		// os.Executable will point to the binary inside the bundle,
		// but we need to point to the bundle itself
		// We expect the following path: /puush.app/Contents/MacOS/puush
		bundlePath := filepath.Dir(filepath.Dir(filepath.Dir(executable)))
		if strings.HasSuffix(bundlePath, ".app") {
			executable = bundlePath
		}
	}

	return &autostart.App{
		Name:        "yeet-client",
		DisplayName: "puush",
		Icon:        exportIcon(),
		Exec:        []string{executable},
	}, nil
}

func exportIcon() string {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	iconPath := filepath.Join(cacheDir, "yeet", "puush.png")

	// Check if icon already exists
	_, err = os.Stat(iconPath)
	if err == nil {
		return iconPath
	}

	// Export icon if it doesn't exist
	os.MkdirAll(filepath.Dir(iconPath), 0755)
	os.WriteFile(iconPath, assets.PuushIconData, 0644)
	return iconPath
}
