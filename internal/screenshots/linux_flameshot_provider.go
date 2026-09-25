//go:build linux

package screenshots

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
)

type FlameshotScreenshotProvider struct {
	binPath        string
	fullscreenMode FullscreenMode
	quality        Quality
}

func NewFlameshotProvider() (ScreenshotProvider, error) {
	binPath, err := exec.LookPath("flameshot")
	if err != nil {
		return nil, fmt.Errorf("flameshot not found in PATH: %w", err)
	}

	return &FlameshotScreenshotProvider{binPath: binPath}, nil
}

// Name returns the name of the screenshot provider
func (p *FlameshotScreenshotProvider) Name() string {
	return "Flameshot"
}

func (p *FlameshotScreenshotProvider) Warning() string {
	return "Flameshot does not support window captures."
}

func (p *FlameshotScreenshotProvider) SetQuality(quality Quality) {
	p.quality = quality
}

func (p *FlameshotScreenshotProvider) SetFullscreenMode(mode FullscreenMode) {
	p.fullscreenMode = mode
}

// Available checks if the flameshot binary is available in the system
func (p *FlameshotScreenshotProvider) Available() bool {
	_, err := exec.LookPath("flameshot")
	return err == nil
}

// CaptureScreen captures the entire screen
func (p *FlameshotScreenshotProvider) CaptureScreen() (io.ReadSeekCloser, error) {
	if p.fullscreenMode == FullscreenModeMouse || p.fullscreenMode == FullscreenModePrimary {
		// `screen` mode captures the monitor with the cursor
		return p.performCapture("screen")
	} else {
		// `full` mode captures all monitors
		return p.performCapture("full")
	}
}

// CaptureArea captures a specific region of the screen
func (p *FlameshotScreenshotProvider) CaptureArea() (io.ReadSeekCloser, error) {
	// `gui` mode allows the user to select a region
	return p.performCapture("gui")
}

// CaptureWindow captures a specific window
func (p *FlameshotScreenshotProvider) CaptureWindow() (io.ReadSeekCloser, error) {
	// Flameshot doesn't have a cli flag for selecting a window without the GUI...
	return p.performCapture("gui")
}

func (p *FlameshotScreenshotProvider) performCapture(modeArgs ...string) (io.ReadSeekCloser, error) {
	// -r outputs the raw png directly to stdout
	modeArgs = append(modeArgs, "-r")
	cmd := exec.Command(p.binPath, modeArgs...)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("flameshot failed: %w", err)
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
	ScreenshotProviders = append(ScreenshotProviders, NewFlameshotProvider)
}
