package tray

import "testing"

func TestTextPreview(t *testing.T) {
	if got := textPreview("  Hello\n\nworld  ", 150); got != "Hello world" {
		t.Errorf("got %q", got)
	}
	if got := textPreview("abcdefghij", 5); got != "abcd…" {
		t.Errorf("got %q", got)
	}
}
