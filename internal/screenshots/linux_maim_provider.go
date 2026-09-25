//go:build linux

package screenshots

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
)

type MaimScreenshotProvider struct {
	binPath        string
	fullscreenMode FullscreenMode
	quality        Quality
}

func NewMaimProvider() (ScreenshotProvider, error) {
	binPath, err := exec.LookPath("maim")
	if err != nil {
		return nil, fmt.Errorf("maim not found in PATH: %w", err)
	}

	return &MaimScreenshotProvider{binPath: binPath}, nil
}

// Name returns the name of the screenshot provider
func (p *MaimScreenshotProvider) Name() string {
	return "Make Image (maim)"
}

func (p *MaimScreenshotProvider) Warning() string {
	return "Make Image (maim) is only available on x11 systems. It may produce unexpected results on wayland."
}

func (p *MaimScreenshotProvider) SetQuality(quality Quality) {
	p.quality = quality
}

func (p *MaimScreenshotProvider) SetFullscreenMode(mode FullscreenMode) {
	p.fullscreenMode = mode
}

// Available checks if the maim binary is available in the system
func (p *MaimScreenshotProvider) Available() bool {
	_, err := exec.LookPath("maim")
	return err == nil
}

// CaptureScreen captures the entire screen
func (p *MaimScreenshotProvider) CaptureScreen() (io.ReadSeekCloser, error) {
	// maim outputs to stdout by default, capture all screens
	return p.performCapture("-f", "png")
}

// CaptureArea captures a specific region of the screen
func (p *MaimScreenshotProvider) CaptureArea() (io.ReadSeekCloser, error) {
	// -s enables interactive selection mode
	return p.performCapture("-s", "-f", "png")
}

// CaptureWindow captures a specific window
func (p *MaimScreenshotProvider) CaptureWindow() (io.ReadSeekCloser, error) {
	// Just like flameshot, maim does not have a dedicated window capture mode
	return p.performCapture("-s", "-f", "png")
}

func (p *MaimScreenshotProvider) performCapture(modeArgs ...string) (io.ReadSeekCloser, error) {
	cmd := exec.Command(p.binPath, modeArgs...)

	// maim outputs the raw image directly to stdout
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("maim failed or was cancelled: %w", err)
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
	ScreenshotProviders = append(ScreenshotProviders, NewMaimProvider)
}
