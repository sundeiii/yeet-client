package config

import (
	"path/filepath"
	"testing"
)

func TestImageSavePath(t *testing.T) {
	capture := DefaultConfig().Capture

	path := capture.ImageSavePath()
	if path == "" || filepath.Base(path) != "yeet" {
		t.Errorf("default save folder should be a yeet folder, got %q", path)
	}

	capture.SaveImagePath = filepath.Join("somewhere", "else")
	if got := capture.ImageSavePath(); got != capture.SaveImagePath {
		t.Errorf("a chosen folder should win over the default, got %q", got)
	}
}
