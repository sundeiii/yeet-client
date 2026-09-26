package notifications

import (
	"encoding/xml"
	"strings"
	"testing"
)

// Texts from other people are data, never code or markup
func TestToastXMLEscapes(t *testing.T) {
	text := `$(Start-Process calc) "@ ]]> <b>&amp; 📎 ä`
	out := toastXML(`it's "me" <x>`, text, `C:\icon.png`, "https://example.test/?a=1&b=2", true)
	var parsed struct {
		Launch string   `xml:"launch,attr"`
		Texts  []string `xml:"visual>binding>text"`
	}
	if err := xml.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("not valid XML: %v\n%s", err, out)
	}
	if len(parsed.Texts) != 2 || parsed.Texts[0] != `it's "me" <x>` || parsed.Texts[1] != text {
		t.Errorf("texts changed: %q", parsed.Texts)
	}
	if parsed.Launch != "https://example.test/?a=1&b=2" {
		t.Errorf("launch: %q", parsed.Launch)
	}
	if strings.Contains(toastScript, "{{") || strings.Contains(toastScript, "@\"") {
		t.Error("the script must not have texts pasted into it")
	}
}
