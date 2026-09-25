//go:build linux

package screenshots

import (
	"fmt"
	"image"
	"io"
	"os/exec"
	"strings"
)

// "Capture Last Area" on Linux: slurp (Wayland) and slop (X11) tell where
// the selection was, so grim and maim can capture it again.

// parseGeometry reads slurp's "x,y wxh" and slop's "x,y,w,h".
func parseGeometry(geometry string) (image.Rectangle, bool) {
	var x, y, w, h int
	geometry = strings.TrimSpace(geometry)
	if _, err := fmt.Sscanf(geometry, "%d,%d %dx%d", &x, &y, &w, &h); err != nil {
		if _, err := fmt.Sscanf(geometry, "%d,%d,%d,%d", &x, &y, &w, &h); err != nil {
			return image.Rectangle{}, false
		}
	}
	if w <= 0 || h <= 0 {
		return image.Rectangle{}, false
	}
	return image.Rect(x, y, x+w, y+h), true
}

func (p *GrimScreenshotProvider) LastArea() (image.Rectangle, bool) {
	return p.lastArea, !p.lastArea.Empty()
}

func (p *GrimScreenshotProvider) CaptureRegion(area image.Rectangle) (io.ReadSeekCloser, error) {
	if area.Empty() {
		return nil, fmt.Errorf("capture region: the area is empty")
	}
	p.lastArea = area
	geometry := fmt.Sprintf("%d,%d %dx%d", area.Min.X, area.Min.Y, area.Dx(), area.Dy())
	return p.performCapture("-g", geometry, "-")
}

func (p *MaimScreenshotProvider) LastArea() (image.Rectangle, bool) {
	return p.lastArea, !p.lastArea.Empty()
}

func (p *MaimScreenshotProvider) CaptureRegion(area image.Rectangle) (io.ReadSeekCloser, error) {
	if area.Empty() {
		return nil, fmt.Errorf("capture region: the area is empty")
	}
	p.lastArea = area
	geometry := fmt.Sprintf("%dx%d+%d+%d", area.Dx(), area.Dy(), area.Min.X, area.Min.Y)
	return p.performCapture("-g", geometry, "-f", "png")
}

// selectWithSlop asks slop for an area, when it's installed (maim's own
// selection can't report where it was).
func (p *MaimScreenshotProvider) selectWithSlop() (image.Rectangle, bool, error) {
	slop, err := exec.LookPath("slop")
	if err != nil {
		return image.Rectangle{}, false, nil
	}
	out, err := exec.Command(slop, "-f", "%x,%y,%w,%h").Output()
	if err != nil {
		return image.Rectangle{}, true, fmt.Errorf("selection was cancelled: %w", err)
	}
	area, ok := parseGeometry(string(out))
	if !ok {
		return image.Rectangle{}, true, fmt.Errorf("selection was cancelled or empty")
	}
	return area, true, nil
}
