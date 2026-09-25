package screenshots

import (
	"errors"
	"io"
)

type ScreenshotProvider interface {
	// Name returns the name of the screenshot provider
	Name() string

	// Warning returns a warning message to be displayed to the user when selecting the provider
	Warning() string

	// SetQuality sets the quality for the screenshots taken by this provider
	SetQuality(quality Quality)

	// SetFullscreenMode sets the mode for fullscreen screenshots taken by this provider
	SetFullscreenMode(mode FullscreenMode)

	// Available checks if the provider is available on the system
	Available() bool

	// CaptureScreen captures the entire screen
	CaptureScreen() (io.ReadSeekCloser, error)

	// CaptureArea captures a specific region of the screen
	CaptureArea() (io.ReadSeekCloser, error)

	// CaptureWindow captures a specific window
	CaptureWindow() (io.ReadSeekCloser, error)
}

// ScreenshotProviders is a list of functions that return available screenshot providers
var ScreenshotProviders []func() (ScreenshotProvider, error)

// GetDefaultProvider returns the first available screenshot provider
func GetDefaultProvider() (ScreenshotProvider, error) {
	for _, providerFunc := range ScreenshotProviders {
		provider, err := providerFunc()
		if err == nil && provider.Available() {
			return provider, nil
		}
	}
	return nil, errors.New("no available screenshot provider found")
}

// GetProviderList returns a list of available screenshot providers by name
func GetProviderList() []string {
	var providerNames []string
	for _, providerFunc := range ScreenshotProviders {
		provider, err := providerFunc()
		if err == nil && provider.Available() {
			providerNames = append(providerNames, provider.Name())
		}
	}
	return providerNames
}

// GetProviderByName returns a screenshot provider by its name
func GetProviderByName(name string) (ScreenshotProvider, error) {
	for _, providerFunc := range ScreenshotProviders {
		provider, err := providerFunc()
		if err != nil {
			continue
		}
		if provider.Name() != name {
			continue
		}
		if !provider.Available() {
			return nil, errors.New("screenshot provider not available")
		}
		return provider, nil
	}
	return nil, errors.New("screenshot provider not found")
}
