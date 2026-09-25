package desktop

import (
	"os/exec"
	"runtime"

	"fyne.io/fyne/v2"
)

// IsDevelopmentBuild checks if the given executable is a development build of the application.
func IsDevelopmentBuild() bool {
	metadata := fyne.CurrentApp().Metadata()
	return metadata.Custom["commit"] == "dev" || metadata.Build == 0
}

// OpenBrowser opens the specified URL in the default browser of the user.
// https://stackoverflow.com/questions/39320371/how-start-web-server-to-open-page-in-browser-in-golang
func OpenBrowser(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start"}
	case "darwin":
		cmd = "open"
	default: // "linux", "freebsd", "openbsd", "netbsd"
		cmd = "xdg-open"
	}
	args = append(args, url)
	return exec.Command(cmd, args...).Start()
}
