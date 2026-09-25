package i18n

import (
	"os"
	"os/exec"
	"strings"
)

// systemLocales asks macOS for the preferred languages; apps started from
// the Finder don't get LANG.
func systemLocales() []string {
	var locales []string
	if output, err := exec.Command("defaults", "read", "-g", "AppleLanguages").Output(); err == nil {
		// ( "nl-NL", "en-US" )
		for _, line := range strings.Split(string(output), "\n") {
			line = strings.Trim(strings.TrimSpace(line), `",()`)
			if line != "" {
				locales = append(locales, line)
			}
		}
	}
	if value := os.Getenv("LANG"); value != "" {
		locales = append(locales, value)
	}
	return locales
}
