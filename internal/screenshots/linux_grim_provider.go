//go:build linux

package screenshots

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

type GrimScreenshotProvider struct {
	grimPath       string
	slurpPath      string
	fullscreenMode FullscreenMode
	quality        Quality
}

func NewGrimProvider() (ScreenshotProvider, error) {
	grimPath, err := exec.LookPath("grim")
	if err != nil {
		return nil, fmt.Errorf("grim not found in PATH: %w", err)
	}
	slurpPath, err := exec.LookPath("slurp")
	if err != nil {
		return nil, fmt.Errorf("slurp not found in PATH: %w", err)
	}

	return &GrimScreenshotProvider{
		grimPath:  grimPath,
		slurpPath: slurpPath,
	}, nil
}

// Name returns the name of the screenshot provider
func (p *GrimScreenshotProvider) Name() string {
	return "grim + slurp"
}

func (p *GrimScreenshotProvider) Warning() string {
	return "grim is only compatible with wlroots-based Wayland compositors (like Sway or Hyprland)."
}

func (p *GrimScreenshotProvider) SetQuality(quality Quality) {
	p.quality = quality
}

func (p *GrimScreenshotProvider) SetFullscreenMode(mode FullscreenMode) {
	p.fullscreenMode = mode
}

// Available checks if both grim and slurp binaries are available in the system
func (p *GrimScreenshotProvider) Available() bool {
	_, errGrim := exec.LookPath("grim")
	_, errSlurp := exec.LookPath("slurp")
	return errGrim == nil && errSlurp == nil
}

// CaptureScreen captures the entire screen
func (p *GrimScreenshotProvider) CaptureScreen() (io.ReadSeekCloser, error) {
	// grim outputs to stdout if "-" is passed as the output file
	return p.performCapture("-")
}

// CaptureArea captures a specific region of the screen
func (p *GrimScreenshotProvider) CaptureArea() (io.ReadSeekCloser, error) {
	return p.captureWithSlurp()
}

// CaptureWindow captures a specific window (fallback to manual region selection)
func (p *GrimScreenshotProvider) CaptureWindow() (io.ReadSeekCloser, error) {
	return p.captureWithSlurp()
}

func (p *GrimScreenshotProvider) captureWithSlurp() (io.ReadSeekCloser, error) {
	slurpCmd := exec.Command(p.slurpPath)
	slurpOut, err := slurpCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("slurp failed or was cancelled: %w", err)
	}

	geom := strings.TrimSpace(string(slurpOut))
	if geom == "" {
		return nil, fmt.Errorf("screenshot was cancelled or empty region selected")
	}

	return p.performCapture("-g", geom, "-")
}

func (p *GrimScreenshotProvider) performCapture(args ...string) (io.ReadSeekCloser, error) {
	cmd := exec.Command(p.grimPath, args...)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("grim failed: %w", err)
	}
	if len(output) == 0 {
		return nil, fmt.Errorf("screenshot was cancelled or is empty")
	}

	reader := &memoryReadCloser{
		Reader: bytes.NewReader(output),
	}
	return ApplyQuality(reader, p.quality)
}

func init() {
	ScreenshotProviders = append(ScreenshotProviders, NewGrimProvider)
}
