//go:build linux

package screenshots

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

type SpectacleScreenshotProvider struct {
	binPath        string
	timeout        time.Duration
	fullscreenMode FullscreenMode
	quality        Quality
}

func NewSpectacleProvider() (ScreenshotProvider, error) {
	binPath, err := exec.LookPath("spectacle")
	if err != nil {
		return nil, fmt.Errorf("spectacle not found in PATH: %w", err)
	}

	return &SpectacleScreenshotProvider{
		binPath: binPath,
		timeout: 5 * time.Minute,
	}, nil
}

// Name returns the name of the screenshot provider
func (p *SpectacleScreenshotProvider) Name() string {
	return "KDE Spectacle"
}

func (p *SpectacleScreenshotProvider) Warning() string {
	return ""
}

func (p *SpectacleScreenshotProvider) SetQuality(quality Quality) {
	p.quality = quality
}

func (p *SpectacleScreenshotProvider) SetFullscreenMode(mode FullscreenMode) {
	p.fullscreenMode = mode
}

// Available checks if the spectacle binary is available in the system
func (p *SpectacleScreenshotProvider) Available() bool {
	_, err := exec.LookPath("spectacle")
	return err == nil
}

// CaptureScreen captures the entire screen
func (p *SpectacleScreenshotProvider) CaptureScreen() (io.ReadSeekCloser, error) {
	if p.fullscreenMode == FullscreenModeMouse || p.fullscreenMode == FullscreenModePrimary {
		// -m current monitor
		return p.performCapture("-m")
	} else {
		// -f fullscreen (all screens)
		return p.performCapture("-f")
	}
}

// CaptureArea captures a specific region of the screen
func (p *SpectacleScreenshotProvider) CaptureArea() (io.ReadSeekCloser, error) {
	// -r rectangular region
	return p.performCapture("-r")
}

// CaptureWindow captures a specific window
func (p *SpectacleScreenshotProvider) CaptureWindow() (io.ReadSeekCloser, error) {
	// -u window under the cursor
	// -w wait for click
	return p.performCapture("-u", "-w")
}

func (p *SpectacleScreenshotProvider) performCapture(modeArgs ...string) (io.ReadSeekCloser, error) {
	tmp, err := os.CreateTemp("", "spectacle-*.png")
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}

	path := tmp.Name()

	if err := tmp.Close(); err != nil {
		os.Remove(path)
		return nil, fmt.Errorf("close temp file: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()

	// -o output file
	// -n no notification
	// -b background mode
	args := append(modeArgs, "-b", "-n", "-o", path)

	cmd := exec.CommandContext(ctx, p.binPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		os.Remove(path)

		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, fmt.Errorf("spectacle timed out")
		}

		msg := strings.TrimSpace(string(output))
		if msg != "" {
			return nil, fmt.Errorf("spectacle failed: %w: %s", err, msg)
		}

		return nil, fmt.Errorf("spectacle failed: %w", err)
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
	ScreenshotProviders = append(ScreenshotProviders, NewSpectacleProvider)
}
