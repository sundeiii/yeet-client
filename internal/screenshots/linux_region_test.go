//go:build linux

package screenshots

import (
	"image"
	"testing"
)

func TestParseGeometry(t *testing.T) {
	cases := map[string]image.Rectangle{
		"10,20 300x200\n": image.Rect(10, 20, 310, 220), // slurp
		"-1920,0,800,600":  image.Rect(-1920, 0, -1120, 600), // slop, screen on the left
	}
	for input, want := range cases {
		if got, ok := parseGeometry(input); !ok || got != want {
			t.Errorf("parseGeometry(%q) = %v, %v; want %v", input, got, ok, want)
		}
	}
	for _, bad := range []string{"", "nonsense", "0,0 0x0"} {
		if _, ok := parseGeometry(bad); ok {
			t.Errorf("parseGeometry(%q) should fail", bad)
		}
	}
}
