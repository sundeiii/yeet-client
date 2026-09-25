//go:build linux

package screenshots

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"strings"
)

type GnomeScreenshotProvider struct {
	binPath        string
	fullscreenMode FullscreenMode
	quality        Quality
}

func NewGnomeProvider() (ScreenshotProvider, error) {
	binPath, err := exec.LookPath("gnome-screenshot")
	if err != nil {
		return nil, fmt.Errorf("gnome-screenshot not found in PATH: %w", err)
	}

	return &GnomeScreenshotProvider{binPath: binPath}, nil
}

// Name returns the name of the screenshot provider
func (p *GnomeScreenshotProvider) Name() string {
	return "GNOME Screenshot"
}

func (p *GnomeScreenshotProvider) Warning() string {
	return ""
}

func (p *GnomeScreenshotProvider) SetQuality(quality Quality) {
	p.quality = quality
}

func (p *GnomeScreenshotProvider) SetFullscreenMode(mode FullscreenMode) {
	p.fullscreenMode = mode
}

// Available checks if the gnome-screenshot binary is available & if the user is running GNOME
func (p *GnomeScreenshotProvider) Available() bool {
	_, err := exec.LookPath("gnome-screenshot")
	if err != nil {
		return false
	}

	desktop := strings.ToLower(os.Getenv("XDG_CURRENT_DESKTOP"))
	return slices.Contains(strings.Split(desktop, ":"), "gnome")
}

// CaptureScreen captures the entire screen
func (p *GnomeScreenshotProvider) CaptureScreen() (io.ReadSeekCloser, error) {
	return p.performCapture()
}

// CaptureArea captures a specific region of the screen
func (p *GnomeScreenshotProvider) CaptureArea() (io.ReadSeekCloser, error) {
	return p.performCapture("-a")
}

// CaptureWindow captures a specific window
func (p *GnomeScreenshotProvider) CaptureWindow() (io.ReadSeekCloser, error) {
	return p.performCapture("-w")
}

func (p *GnomeScreenshotProvider) performCapture(modeArgs ...string) (io.ReadSeekCloser, error) {
	tmp, err := os.CreateTemp("", "gnome-screenshot-*.png")
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}
	path := tmp.Name()

	// Close the file so gnome-screenshot can write to it
	if err := tmp.Close(); err != nil {
		os.Remove(path)
		return nil, fmt.Errorf("close temp file: %w", err)
	}

	// -f output file
	args := append(modeArgs, "-f", path)

	cmd := exec.Command(p.binPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		os.Remove(path)

		msg := strings.TrimSpace(string(output))
		if msg != "" {
			return nil, fmt.Errorf("gnome-screenshot failed: %w: %s", err, msg)
		}

		return nil, fmt.Errorf("gnome-screenshot failed or was cancelled: %w", err)
	}

	file, err := os.Open(path)
	if err != nil {
		os.Remove(path)
		return nil, fmt.Errorf("open screenshot: %w", err)
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		os.Remove(path)
		return nil, fmt.Errorf("stat screenshot: %w", err)
	}
	if info.Size() == 0 {
		file.Close()
		os.Remove(path)
		return nil, fmt.Errorf("screenshot was cancelled or is empty")
	}

	reader := &temporaryFileReader{
		file: file,
		path: path,
	}
	return ApplyQuality(reader, p.quality)
}

func init() {
	ScreenshotProviders = append(ScreenshotProviders, NewGnomeProvider)
}
