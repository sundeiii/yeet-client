//go:build !windows && !darwin

package i18n

import (
	"os"
	"strings"
)

// systemLocales reads the usual locale variables, most specific first.
func systemLocales() []string {
	var locales []string
	if list := os.Getenv("LANGUAGE"); list != "" {
		locales = append(locales, strings.Split(list, ":")...)
	}
	for _, name := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if value := os.Getenv(name); value != "" {
			locales = append(locales, value)
		}
	}
	return locales
}
