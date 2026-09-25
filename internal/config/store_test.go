package config

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestJsonStore(t *testing.T) {
	tempDir := t.TempDir()
	store := &JsonStore{
		Path: filepath.Join(tempDir, "config.json"),
	}

	// Create a dummy config with some changes from defaults
	cfg := DefaultConfig()
	cfg.Account.Username = "test_user"
	cfg.General.Startup = false
	cfg.Capture.UploadQuality = 2

	// Save the config
	if err := store.Save(cfg); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	// Ensure the file was actually created
	if _, err := os.Stat(store.Path); os.IsNotExist(err) {
		t.Fatalf("expected config file to be created at %s", store.Path)
	}

	// Load the config back
	updatedCfg, err := store.Load()
	if err != nil {
		t.Fatalf("failed to load saved config: %v", err)
	}

	// Verify the changes
	if updatedCfg.Account.Username != "test_user" {
		t.Errorf("expected Username 'test_user', got '%s'", updatedCfg.Account.Username)
	}
	if updatedCfg.General.Startup != false {
		t.Errorf("expected Startup to be false, got true")
	}
	if updatedCfg.Capture.UploadQuality != 2 {
		t.Errorf("expected UploadQuality to be 2, got %d", updatedCfg.Capture.UploadQuality)
	}
}

func TestRememberLocalCopy(t *testing.T) {
	capture := &CaptureConfig{}
	capture.RememberLocalCopy("", "C:/a.png")
	capture.RememberLocalCopy("https://x/a", "")
	if len(capture.LocalCopies) != 0 {
		t.Fatal("empty links or paths shouldn't be remembered")
	}

	for i := range maxLocalCopies + 10 {
		capture.RememberLocalCopy(fmt.Sprintf("https://x/%d", i), fmt.Sprintf("C:/%d.png", i))
	}
	if len(capture.LocalCopies) != maxLocalCopies {
		t.Errorf("expected %d remembered copies, got %d", maxLocalCopies, len(capture.LocalCopies))
	}
	last := fmt.Sprintf("https://x/%d", maxLocalCopies+9)
	if capture.LocalCopies[last] != fmt.Sprintf("C:/%d.png", maxLocalCopies+9) {
		t.Error("the newest copy must always be kept")
	}
}

func TestOldConfigsGetNewDefaults(t *testing.T) {
	store := &JsonStore{Path: filepath.Join(t.TempDir(), "config.json")}
	// A config written by an older version, without the newer settings
	os.WriteFile(store.Path, []byte(`{"Hotkeys": {"ScreenSelection": "Ctrl+Shift+9"}, "Capture": {"SaveImages": false}}`), 0644)

	cfg, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Hotkeys.ScreenSelection != "Ctrl+Shift+9" || cfg.Capture.SaveImages {
		t.Error("saved settings must be kept")
	}
	if cfg.Hotkeys.RepeatArea == "" || cfg.Capture.Delay() != 3*time.Second {
		t.Errorf("new settings should get their defaults, got %q and %v", cfg.Hotkeys.RepeatArea, cfg.Capture.Delay())
	}
}

func TestOldServerAddressMoves(t *testing.T) {
	store := &JsonStore{Path: filepath.Join(t.TempDir(), "config.json")}
	os.WriteFile(store.Path, []byte(`{"Misc": {"ServerURL": "https://p.tupsujumal.ee"}}`), 0644)
	cfg, _ := store.Load()
	if cfg.Misc.ServerURL != DefaultServerURL {
		t.Errorf("expected the new address, got %q", cfg.Misc.ServerURL)
	}

	// Other servers are left alone
	os.WriteFile(store.Path, []byte(`{"Misc": {"ServerURL": "https://puush.example.com"}}`), 0644)
	cfg, _ = store.Load()
	if cfg.Misc.ServerURL != "https://puush.example.com" {
		t.Errorf("a custom server must not change, got %q", cfg.Misc.ServerURL)
	}
}
