package desktop

import (
	"testing"
	"time"

	"github.com/sundeiii/yeet-client/pkg/puush"
)

func TestPresenceAndTimes(t *testing.T) {
	now := time.Date(2026, 9, 26, 15, 0, 0, 0, time.Local)
	for _, c := range []struct {
		p    *puush.Presence
		want string
	}{
		{nil, ""},
		{&puush.Presence{Online: true, Hidden: true}, ""},
		{&puush.Presence{Online: true, Devices: []string{"phone", "web"}}, "Online on phone, the website"},
		{&puush.Presence{Seen: now.Add(-20 * time.Second).Unix()}, "Last online just now"},
		{&puush.Presence{Seen: now.Add(-3 * time.Minute).Unix()}, "Last online 3 min ago"},
		{&puush.Presence{Seen: now.Add(-2 * time.Hour).Unix()}, "Last online today at 13:00"},
		{&puush.Presence{Seen: now.Add(-20 * time.Hour).Unix()}, "Last online yesterday at 19:00"},
	} {
		if got := presenceText(c.p, now); got != c.want {
			t.Errorf("%+v: %q, want %q", c.p, got, c.want)
		}
	}
	if got := messageTime(&puush.ChatMessage{At: now.Add(-time.Hour).Unix()}, now); got != "14:00" {
		t.Errorf("today: %q", got)
	}
	if got := messageTime(&puush.ChatMessage{At: now.AddDate(0, 0, -1).Unix()}, now); got != "Sep 25 15:00" {
		t.Errorf("yesterday: %q", got)
	}
	if got := messageTime(&puush.ChatMessage{Time: "05:18"}, now); got != "05:18" {
		t.Errorf("older server: %q", got)
	}
}
