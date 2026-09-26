// Package i18n translates the app. Texts are written in English in the
// code; locales/<code>.json maps them to other languages. Anything missing
// from a translation stays English.
package i18n

import (
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

// Language is one language the app can be shown in.
type Language struct {
	Code string // en, nl, de
	Name string // in the language itself
	Flag string // file name in assets/flags, like osu!'s
}

// Languages lists the languages the app is translated into, English first.
var Languages = []Language{
	{Code: "en", Name: "English", Flag: "GB"},
	{Code: "nl", Name: "Nederlands", Flag: "NL"},
	{Code: "de", Name: "Deutsch", Flag: "DE"},
}

// Default is used when the system's language isn't one of Languages.
const Default = "en"

//go:embed locales/*.json
var locales embed.FS

var (
	mu       sync.RWMutex
	current  = Default
	catalogs = map[string]map[string]string{}
)

func init() {
	for _, lang := range Languages[1:] {
		data, err := locales.ReadFile("locales/" + lang.Code + ".json")
		if err != nil {
			continue
		}
		catalog := map[string]string{}
		if err := json.Unmarshal(data, &catalog); err != nil {
			log.Printf("Translations for %s can't be read: %v", lang.Code, err)
			continue
		}
		catalogs[lang.Code] = catalog
	}
}

// Supported tells whether code is one of Languages.
func Supported(code string) bool {
	for _, lang := range Languages {
		if lang.Code == code {
			return true
		}
	}
	return false
}

// SetLanguage switches the app's language. An empty or unknown code means
// the system's language.
func SetLanguage(code string) {
	if !Supported(code) {
		code = SystemLanguage()
	}
	mu.Lock()
	current = code
	mu.Unlock()
}

// Current is the language the app is shown in.
func Current() string {
	mu.RLock()
	defer mu.RUnlock()
	return current
}

// T translates text into the app's language and fills in args like
// fmt.Sprintf.
func T(text string, args ...any) string {
	mu.RLock()
	if translated, ok := catalogs[current][text]; ok && translated != "" {
		text = translated
	}
	mu.RUnlock()
	if len(args) > 0 {
		return fmt.Sprintf(text, args...)
	}
	return text
}

// In translates text into a given language, whatever the app shows.
func In(code, text string) string {
	if translated, ok := catalogs[code][text]; ok && translated != "" {
		return translated
	}
	return text
}

// Has tells whether the language has a translation for text.
func Has(code, text string) bool {
	_, ok := catalogs[code][text]
	return ok
}

// SystemLanguage is the system's language if the app has it, else English.
func SystemLanguage() string {
	for _, locale := range systemLocales() {
		if code := match(locale); code != "" {
			return code
		}
	}
	return Default
}

// match turns a locale like "nl_BE.UTF-8", "de-AT" or "en" into one of
// Languages, or "".
func match(locale string) string {
	locale = strings.ToLower(strings.TrimSpace(locale))
	if locale == "" || locale == "c" || locale == "posix" {
		return ""
	}
	code, _, _ := strings.Cut(strings.NewReplacer("-", "_", ".", "_").Replace(locale), "_")
	if Supported(code) {
		return code
	}
	return ""
}

var months = map[string][12]string{
	"nl": {"jan", "feb", "mrt", "apr", "mei", "jun", "jul", "aug", "sep", "okt", "nov", "dec"},
	"de": {"Jan.", "Feb.", "März", "Apr.", "Mai", "Juni", "Juli", "Aug.", "Sep.", "Okt.", "Nov.", "Dez."},
}

// Date formats a date and time the way the app's language writes it, like
// "Jan 2, 2006 15:04", "2 jan 2006 15:04" or "2. Jan. 2006 15:04".
func Date(t time.Time) string {
	lang := Current()
	names, ok := months[lang]
	if !ok {
		return t.Format("Jan 2, 2006 15:04")
	}
	separator := " "
	if lang == "de" {
		separator = ". "
	}
	return fmt.Sprintf("%d%s%s %d %s", t.Day(), separator, names[t.Month()-1], t.Year(), t.Format("15:04"))
}

// ShortDate is the day and month, like "Jan 2", "2 jan" or "2. Jan.".
func ShortDate(t time.Time) string {
	lang := Current()
	names, ok := months[lang]
	if !ok {
		return t.Format("Jan 2")
	}
	if lang == "de" {
		return fmt.Sprintf("%d. %s", t.Day(), names[t.Month()-1])
	}
	return fmt.Sprintf("%d %s", t.Day(), names[t.Month()-1])
}
