package i18n

import "golang.org/x/sys/windows"

// systemLocales is the Windows display language, like "nl-NL".
func systemLocales() []string {
	languages, err := windows.GetUserPreferredUILanguages(windows.MUI_LANGUAGE_NAME)
	if err != nil {
		return nil
	}
	return languages
}
