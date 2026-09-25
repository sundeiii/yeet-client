package notifications

import (
	"testing"

	"github.com/sundeiii/yeet-client/assets"
)

func TestNotification(t *testing.T) {
	err := NewNotification("app name", "title", "text").
		WithIconData(assets.PuushIconData).
		WithSoundData(assets.SuccessSoundData).
		WithAction("https://example.com").
		Push()

	if err != nil {
		t.Errorf("Failed to push notification: %v", err)
	}
}
