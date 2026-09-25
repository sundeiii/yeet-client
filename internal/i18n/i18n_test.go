package i18n

import "testing"

func TestMatch(t *testing.T) {
	for locale, want := range map[string]string{
		"nl_BE.UTF-8": "nl", "de-AT": "de", "de": "de", "en_US.UTF-8": "en", "fr_FR": "", "C": "", "": "",
	} {
		if got := match(locale); got != want {
			t.Errorf("match(%q) = %q, want %q", locale, got, want)
		}
	}
}

func TestSetLanguage(t *testing.T) {
	defer SetLanguage("en")
	SetLanguage("de")
	if Current() != "de" {
		t.Fatal("not German")
	}
	if got := T("%d views", 3); got != "3 Aufrufe" {
		t.Errorf("T: %q", got)
	}
}
