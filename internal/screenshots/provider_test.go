//go:build integration

package screenshots

import (
	"testing"
)

func TestAnyProvider(t *testing.T) {
	if len(ScreenshotProviders) <= 0 {
		t.Skip("no screenshot providers available")
	}

	provider, err := ScreenshotProviders[0]()
	if err != nil {
		t.Skipf("failed to create provider: %v", err)
	}
	t.Logf("Testing provider: '%s'", provider.Name())

	t.Run("CaptureScreen", func(t *testing.T) {
		reader, err := provider.CaptureScreen()
		if err != nil {
			t.Fatalf("CaptureScreen failed: %v", err)
		}
		defer reader.Close()
	})
	t.Run("CaptureArea", func(t *testing.T) {
		reader, err := provider.CaptureArea()
		if err != nil {
			t.Fatalf("CaptureArea failed: %v", err)
		}
		defer reader.Close()
	})
	t.Run("CaptureWindow", func(t *testing.T) {
		reader, err := provider.CaptureWindow()
		if err != nil {
			t.Fatalf("CaptureWindow failed: %v", err)
		}
		defer reader.Close()
	})
}
