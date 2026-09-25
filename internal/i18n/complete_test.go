package i18n

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// Every text passed to T in the code has to be in each translation.
// Texts that reach T through a variable are listed in runtimeTexts.

var textPattern = regexp.MustCompile(`i18n\.T\(("(?:[^"\\]|\\.)*")`)

var runtimeTexts = []string{
	// Account types, as "<type> account"
	"Free account", "Pro account", "Unlimited account", "Unknown account",
	// Editor tools
	"Box", "Arrow", "Pen", "Highlight", "Blur",
	// Screenshot providers' warnings
	"Flameshot does not support window captures.",
	"grim is only compatible with wlroots-based Wayland compositors (like Sway or Hyprland).",
	"Make Image (maim) is only available on x11 systems. It may produce unexpected results on wayland.",
}

func collect(t *testing.T) map[string]string {
	found := map[string]string{}
	root := filepath.Join("..", "..")
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, match := range textPattern.FindAllStringSubmatch(string(data), -1) {
			text, err := strconv.Unquote(match[1])
			if err != nil {
				t.Fatalf("%s: %v", path, err)
			}
			found[text] = path
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range runtimeTexts {
		found[text] = "runtime"
	}
	return found
}

func TestTranslationsAreComplete(t *testing.T) {
	texts := collect(t)
	if len(texts) < 150 {
		t.Fatalf("only found %d texts; the pattern is probably broken", len(texts))
	}
	for _, lang := range Languages[1:] {
		var missing []string
		for text := range texts {
			if !Has(lang.Code, text) {
				missing = append(missing, text)
			}
		}
		sort.Strings(missing)
		if len(missing) > 0 {
			if dump := os.Getenv("I18N_DUMP"); dump != "" {
				out, _ := json.MarshalIndent(missing, "", "  ")
				os.WriteFile(dump+"-"+lang.Code+".json", out, 0o644)
			}
			t.Errorf("%s is missing %d translations, like %q", lang.Name, len(missing), missing[0])
		}
	}
}

// Translations must keep the same %s and %d as the English text.
func TestTranslationsKeepPlaceholders(t *testing.T) {
	placeholder := regexp.MustCompile(`%[sdv%]`)
	for code, catalog := range catalogs {
		for english, translated := range catalog {
			want := strings.Join(placeholder.FindAllString(english, -1), "")
			got := strings.Join(placeholder.FindAllString(translated, -1), "")
			if want != got {
				t.Errorf("%s: %q has %q but the translation %q has %q", code, english, want, translated, got)
			}
		}
	}
}
