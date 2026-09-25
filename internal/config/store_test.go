package config

import (
	"os"
	"path/filepath"
	"testing"
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
