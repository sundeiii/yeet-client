package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/sundeiii/yeet-client/internal/screenshots"
	"github.com/sundeiii/yeet-client/pkg/puush"
)

// Store defines the interface for loading and saving the
// configuration across different platforms.
type Store interface {
	Load() (*Config, error)
	Save(cfg *Config) error
}

// Config represents the application configuration.
type Config struct {
	Account AccountConfig
	General GeneralConfig
	Capture CaptureConfig
	Hotkeys HotkeyConfig
	Misc    MiscConfig
}

type AccountConfig struct {
	Username string
	Key      string
	Type     puush.AccountType
	Usage    int64
	Expiry   string
}

func (a *AccountConfig) DiskUsageHumanReadable() string {
	return formatBytes(a.Usage)
}

func (a *AccountConfig) HasCredentials() bool {
	return a.Username != "" && a.Key != ""
}

func (a *AccountConfig) Reset() {
	a.Username = ""
	a.Key = ""
	a.Type = puush.AccountTypeRegular
	a.Usage = 0
	a.Expiry = ""
}

func (a *AccountConfig) SubscriptionExpiry() *time.Time {
	if a.Expiry == "" {
		return nil
	}
	expiryTime, err := time.Parse(time.RFC3339, a.Expiry)
	if err != nil {
		return nil
	}
	if expiryTime.IsZero() {
		return nil
	}
	return &expiryTime
}

type GeneralConfig struct {
	OpenBrowser       bool
	NotificationSound bool
	Startup           bool
	ContextMenu       bool
	DisabledToggle    bool
	CopyToClipboard   bool
	AutoUpdate        bool
	// UploadPoolId is the pool new uploads go into; 0 is the account's default
	UploadPoolId int
	// Albums: several files uploaded at once share one album link
	Albums bool
	// Sound after an upload: puush, pop, chime, system or none
	Sound string
	// NotifySuccess shows a notification after each upload (errors always show)
	NotifySuccess bool
	// NotifyPreview shows the uploaded picture in the notification
	NotifyPreview bool
	// Language of the app (en, nl, de); empty follows the system
	Language string
}

type CaptureConfig struct {
	UploadQuality         screenshots.Quality
	FullscreenMode        screenshots.FullscreenMode
	SaveImages            bool
	SaveImagesToClipboard bool
	SaveImagePath         string
	MonitorDirectories    []string
	ScreenshotProvider    string
	// DelaySeconds is the countdown before a delayed capture
	DelaySeconds int
	// LastArea is the latest area picked for "Capture Last Area" (x, y, width, height)
	LastArea []int
	// EditBeforeUpload opens the editor after every screenshot
	EditBeforeUpload bool
	// NamePattern names screenshots, with {date}, {time} and {window}
	NamePattern string
	// RecordingFormat is what screen recordings are saved as: mp4 or gif
	RecordingFormat string
	// LocalCopies maps upload links to the file they came from, for "Show in Folder"
	LocalCopies map[string]string
}

// maxLocalCopies is how many upload links remember their local file
const maxLocalCopies = 50

// RememberLocalCopy records which file on this computer an upload came from.
func (capture *CaptureConfig) RememberLocalCopy(link, path string) {
	if link == "" || path == "" {
		return
	}
	if capture.LocalCopies == nil {
		capture.LocalCopies = map[string]string{}
	}
	capture.LocalCopies[link] = path
	// Forget arbitrary old ones; only the recent uploads menu uses these
	for key := range capture.LocalCopies {
		if len(capture.LocalCopies) <= maxLocalCopies {
			break
		}
		if key != link {
			delete(capture.LocalCopies, key)
		}
	}
}

// Delay is the countdown before a delayed capture.
func (capture *CaptureConfig) Delay() time.Duration {
	if capture.DelaySeconds <= 0 {
		return 3 * time.Second
	}
	return time.Duration(capture.DelaySeconds) * time.Second
}

type HotkeyConfig struct {
	ScreenSelection         string
	FullscreenScreenshot    string
	CurrentWindowScreenshot string
	UploadFile              string
	UploadClipboard         string
	Toggle                  string
	RepeatArea              string
	DelayedArea             string
	EditArea                string
	Record                  string
}

type MiscConfig struct {
	LastUpdate time.Time
	ServerURL  string
}

func (misc *MiscConfig) ParseServerURL() *url.URL {
	obj, err := url.Parse(misc.ServerURL)
	if err != nil {
		// Url seems to be invalid, revert back to default
		misc.ServerURL = DefaultServerURL
		obj, _ = url.Parse(misc.ServerURL)
	}

	// Ensure path is cleared
	obj.Path = ""
	return obj
}

// DefaultServerURL is the server a fresh install talks to. It can be changed
// at build time with -ldflags "-X github.com/sundeiii/yeet-client/internal/config.DefaultServerURL=https://..."
var DefaultServerURL = "https://img.sundei.eu"

// oldServerURLs are earlier addresses of the default server; installs that
// still point at one move to the current address.
var oldServerURLs = []string{"https://p.tupsujumal.ee", "https://p.tupsujumal.ee/"}

// migrate updates settings saved by older versions.
func (cfg *Config) migrate() {
	// The sound used to be a switch
	if !cfg.General.NotificationSound && cfg.General.Sound == "puush" {
		cfg.General.Sound = "none"
	}
	for _, old := range oldServerURLs {
		if cfg.Misc.ServerURL == old {
			cfg.Misc.ServerURL = DefaultServerURL
		}
	}
}

// DefaultConfig returns a Config populated with default values.
func DefaultConfig() *Config {
	return &Config{
		General: GeneralConfig{
			OpenBrowser:       false,
			NotificationSound: true,
			CopyToClipboard:   true,
			Startup:           true,
			ContextMenu:       true,
			DisabledToggle:    false,
			AutoUpdate:        true,
			Albums:            true,
			Sound:             "puush",
			NotifySuccess:     true,
			NotifyPreview:     true,
		},
		Capture: CaptureConfig{
			UploadQuality:         screenshots.QualityBest,
			FullscreenMode:        screenshots.FullscreenModeAllScreens,
			SaveImages:            true,
			SaveImagesToClipboard: false,
			SaveImagePath:         "",
			MonitorDirectories:    []string{},
			ScreenshotProvider:    "",
			DelaySeconds:          3,
			RecordingFormat:       "mp4",
		},
		Hotkeys: HotkeyConfig{
			ScreenSelection:         "Ctrl+Shift+4",
			FullscreenScreenshot:    "Ctrl+Shift+3",
			CurrentWindowScreenshot: "Ctrl+Shift+2",
			UploadFile:              "Ctrl+Shift+U",
			UploadClipboard:         "Ctrl+Shift+5",
			Toggle:                  "Ctrl+Alt+P",
			RepeatArea:              "Ctrl+Shift+6",
			DelayedArea:             "Ctrl+Shift+7",
			EditArea:                "Ctrl+Shift+8",
			Record:                  "Ctrl+Shift+9",
		},
		Misc: MiscConfig{
			LastUpdate: time.Now(),
			ServerURL:  DefaultServerURL,
		},
	}
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%dB", bytes)
	}

	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	units := []string{"KB", "MB", "GB", "TB", "PB", "EB"}
	return fmt.Sprintf("%.2f%s", float64(bytes)/float64(div), units[exp])
}

// DefaultSaveImagePath is where local copies of screenshots go when no folder
// was picked: a "yeet" folder inside the user's Pictures folder.
func DefaultSaveImagePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	pictures := filepath.Join(home, "Pictures")
	// Linux desktops can move the Pictures folder, e.g. when not in English
	if dir := os.Getenv("XDG_PICTURES_DIR"); dir != "" {
		pictures = dir
	}
	return filepath.Join(pictures, "yeet")
}

// ImageSavePath is the folder local copies are saved to.
func (capture *CaptureConfig) ImageSavePath() string {
	if capture.SaveImagePath != "" {
		return capture.SaveImagePath
	}
	return DefaultSaveImagePath()
}
