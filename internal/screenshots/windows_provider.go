//go:build windows

package screenshots

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
)

type WindowsScreenshotProvider struct {
	quality  Quality
	lastArea image.Rectangle
}

func NewWindowsScreenshotProvider() (ScreenshotProvider, error) {
	p := &WindowsScreenshotProvider{}
	if !p.Available() {
		return nil, errors.New("windows screenshot provider is not available")
	}
	return p, nil
}

func (p *WindowsScreenshotProvider) Name() string {
	return "Windows (Native)"
}

func (p *WindowsScreenshotProvider) Warning() string {
	return ""
}

func (p *WindowsScreenshotProvider) SetQuality(quality Quality) {
	p.quality = quality
}

func (p *WindowsScreenshotProvider) SetFullscreenMode(mode FullscreenMode) {
	// TODO: ...
}

func (p *WindowsScreenshotProvider) Available() bool {
	return true // TODO
}

func (p *WindowsScreenshotProvider) CaptureScreen() (io.ReadSeekCloser, error) {
	x := getSystemMetrics(smXVirtualScreen)
	y := getSystemMetrics(smYVirtualScreen)
	width := getSystemMetrics(smCXVirtualScreen)
	height := getSystemMetrics(smCYVirtualScreen)

	img, err := captureScreenRect(x, y, width, height)
	if err != nil {
		return nil, fmt.Errorf("capture screen: %w", err)
	}

	reader, err := newPngReader(img)
	if err != nil {
		return nil, err
	}
	return ApplyQuality(reader, p.quality)
}

func (p *WindowsScreenshotProvider) CaptureArea() (io.ReadSeekCloser, error) {
	selection, err := selectArea()
	if err != nil {
		return nil, fmt.Errorf("select area: %w", err)
	}
	p.lastArea = selection.area

	// The selector froze the screen, so the picture is what was on it then
	reader, err := newPngReader(selection.Image())
	if err != nil {
		return nil, err
	}
	return ApplyQuality(reader, p.quality)
}

func (p *WindowsScreenshotProvider) LastArea() (image.Rectangle, bool) {
	return p.lastArea, !p.lastArea.Empty()
}

func (p *WindowsScreenshotProvider) CaptureRegion(area image.Rectangle) (io.ReadSeekCloser, error) {
	if area.Empty() {
		return nil, errors.New("capture region: the area is empty")
	}

	img, err := captureScreenRect(area.Min.X, area.Min.Y, area.Dx(), area.Dy())
	if err != nil {
		return nil, fmt.Errorf("capture region: %w", err)
	}
	p.lastArea = area

	reader, err := newPngReader(img)
	if err != nil {
		return nil, err
	}
	return ApplyQuality(reader, p.quality)
}

func (p *WindowsScreenshotProvider) CaptureWindow() (io.ReadSeekCloser, error) {
	hwnd, err := getForegroundWindow()
	if err != nil {
		return nil, fmt.Errorf("get foreground window: %w", err)
	}

	img, err := captureWindow(hwnd)
	if err != nil {
		return nil, fmt.Errorf("capture window: %w", err)
	}

	reader, err := newPngReader(img)
	if err != nil {
		return nil, err
	}
	return ApplyQuality(reader, p.quality)
}

func newPngReader(img image.Image) (io.ReadSeekCloser, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("encode png: %w", err)
	}
	return &memoryReadCloser{bytes.NewReader(buf.Bytes())}, nil
}

func init() {
	ScreenshotProviders = append(ScreenshotProviders, NewWindowsScreenshotProvider)
}
