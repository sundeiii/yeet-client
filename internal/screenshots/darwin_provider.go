//go:build darwin

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

type DarwinScreenshotProvider struct {
	binPath string
	timeout time.Duration
	quality Quality
}

func NewDarwinProvider() (ScreenshotProvider, error) {
	binPath, err := resolveScreenCaptureBinary()
	if err != nil {
		return nil, fmt.Errorf("failed to find screencapture binary: %w", err)
	}

	return &DarwinScreenshotProvider{
		binPath: binPath,
		timeout: 5 * time.Minute,
	}, nil
}

// Name returns the name of the screenshot provider
func (p *DarwinScreenshotProvider) Name() string {
	return "macOS (screencapture)"
}

func (p *DarwinScreenshotProvider) Warning() string {
	return ""
}

func (p *DarwinScreenshotProvider) SetQuality(quality Quality) {
	p.quality = quality
}

func (p *DarwinScreenshotProvider) SetFullscreenMode(mode FullscreenMode) {
	// TODO: ...
}

func (p *DarwinScreenshotProvider) Available() bool {
	_, err := resolveScreenCaptureBinary()
	return err == nil
}

// CaptureScreen captures the entire screen
func (p *DarwinScreenshotProvider) CaptureScreen() (io.ReadSeekCloser, error) {
	// -m captures the main monitor only, or standard behavior for desktop capture
	// -x prevents sound
	return p.performCapture("-x", "-m")
}

// CaptureArea captures a specific region of the screen
func (p *DarwinScreenshotProvider) CaptureArea() (io.ReadSeekCloser, error) {
	// -i for interactive capture / area selection
	// -x prevents sound
	return p.performCapture("-x", "-i")
}

// CaptureWindow captures a specific window
func (p *DarwinScreenshotProvider) CaptureWindow() (io.ReadSeekCloser, error) {
	// -W starts in window selection mode
	// -i for interactive capture / area selection
	// -x prevents sound
	return p.performCapture("-x", "-i", "-W")
}

func (p *DarwinScreenshotProvider) performCapture(modeArgs ...string) (io.ReadSeekCloser, error) {
	tmp, err := os.CreateTemp("", "screencapture-*.png")
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

	args := append(modeArgs, path)
	cmd := exec.CommandContext(ctx, p.binPath, args...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		os.Remove(path)

		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, fmt.Errorf("screencapture timed out")
		}

		// Determine if the user cancelled the interactive capture
		msg := strings.TrimSpace(string(output))
		if msg != "" {
			return nil, fmt.Errorf("screencapture failed: %w: %s", err, msg)
		}

		return nil, fmt.Errorf("screencapture failed or was cancelled: %w", err)
	}

	// Verify the file was actually written to and isn't empty
	info, err := os.Stat(path)
	if err != nil || info.Size() == 0 {
		os.Remove(path)
		return nil, errors.New("screenshot cancelled or empty")
	}

	file, err := os.Open(path)
	if err != nil {
		os.Remove(path)
		return nil, fmt.Errorf("open screenshot: %w", err)
	}

	reader := &temporaryFileReader{
		file: file,
		path: path,
	}
	return ApplyQuality(reader, p.quality)
}

func resolveScreenCaptureBinary() (string, error) {
	binPath, err := exec.LookPath("screencapture")
	if err == nil {
		return binPath, nil
	}

	// Fallback to absolute path and hope its available
	binPath = "/usr/sbin/screencapture"
	if _, err := os.Stat(binPath); err != nil {
		return "", fmt.Errorf("screencapture not found: %w", err)
	}

	return binPath, nil
}

func init() {
	ScreenshotProviders = append(ScreenshotProviders, NewDarwinProvider)
}
