package tray

import (
	"strings"
	"time"
)

// DefaultNamePattern is how screenshots were always named.
const DefaultNamePattern = "ss ({date} at {time})"

// ScreenshotName fills in a name pattern: {date} (2026-09-25), {time}
// (22.14.23), {window} (the title of the window that was in front),
// {year}, {month} and {day}. Characters files can't have are left out, and
// an empty result falls back to the usual name.
func ScreenshotName(pattern, window string, at time.Time) string {
	if strings.TrimSpace(pattern) == "" {
		pattern = DefaultNamePattern
	}
	name := strings.NewReplacer(
		"{date}", at.Format("2006-01-02"),
		"{time}", at.Format("15.04.05"),
		"{year}", at.Format("2006"),
		"{month}", at.Format("01"),
		"{day}", at.Format("02"),
		"{window}", window,
	).Replace(pattern)

	name = strings.Map(func(r rune) rune {
		if strings.ContainsRune(`\/:*?"<>|`, r) || r < 32 {
			return -1
		}
		return r
	}, name)
	name = strings.Join(strings.Fields(name), " ")
	if runes := []rune(name); len(runes) > 120 {
		name = strings.TrimSpace(string(runes[:120]))
	}
	if name == "" || strings.Trim(name, ". ") == "" {
		return ScreenshotName(DefaultNamePattern, "", at)
	}
	return name
}
