package tray

import (
	"testing"
	"time"
)

func TestScreenshotName(t *testing.T) {
	at := time.Date(2026, 9, 25, 22, 14, 23, 0, time.UTC)
	cases := []struct{ pattern, window, want string }{
		{"", "", "ss (2026-09-25 at 22.14.23)"},
		{"{window} {date}", "Discord", "Discord 2026-09-25"},
		{"{year}/{month}/{day} {window}", `C:\Users\me: "notes"`, "20260925 CUsersme notes"},
		{"{window}", "", "ss (2026-09-25 at 22.14.23)"},
		{"  shot   {time} ", "", "shot 22.14.23"},
	}
	for _, c := range cases {
		if got := ScreenshotName(c.pattern, c.window, at); got != c.want {
			t.Errorf("ScreenshotName(%q, %q) = %q, want %q", c.pattern, c.window, got, c.want)
		}
	}
}
