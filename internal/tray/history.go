package tray

import (
	"fmt"
	"github.com/sundeiii/yeet-client/internal/i18n"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"github.com/sundeiii/yeet-client/pkg/puush"
)

// How often the history is checked, so uploads made elsewhere (e.g. on the
// website) show up in the menu too
const historyRefreshInterval = 30 * time.Second

// StartHistoryRefresh keeps the recent uploads (and pools) up to date.
func (m *TrayManager) StartHistoryRefresh() {
	go func() {
		ticker := time.NewTicker(historyRefreshInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				m.RefreshHistory()
			case <-m.uploadQueueStop:
				return
			}
		}
	}()
}

// RefreshHistory will update the tray's upload history
func (m *TrayManager) RefreshHistory() {
	if !m.api.Account.Credentials.HasApiKey() {
		return
	}
	history, err := m.api.History()
	if err != nil {
		return
	}

	// This often runs in a background goroutine, but the menu may only be
	// changed on the main thread
	fyne.Do(func() {
		if sameHistory(m.uploadHistory, history) {
			return
		}
		m.uploadHistory = history
		m.rebuildMenuItems()
		m.notifyListeners()
	})
}

// sameHistory reports whether two history lists show the same entries, so the
// tray menu only gets rebuilt when something actually changed.
func sameHistory(a, b []*puush.HistoryItem) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Id != b[i].Id || a[i].Views != b[i].Views || a[i].FileName != b[i].FileName {
			return false
		}
	}
	return true
}

func (m *TrayManager) BuildHistoryMenu() []*fyne.MenuItem {
	recentUploads := fyne.NewMenuItem(i18n.T("Recent Uploads"), func() {})
	recentUploads.Disabled = true
	items := []*fyne.MenuItem{recentUploads}

	for _, historyItem := range m.uploadHistory {
		items = append(items, m.BuildHistoryMenuItem(historyItem))
	}
	return items
}

func (m *TrayManager) BuildHistoryMenuItem(historyItem *puush.HistoryItem) *fyne.MenuItem {
	timeItem := fyne.NewMenuItem(fmt.Sprintf("Uploaded: %s", historyItem.Time.Format("2006-01-02 15:04")), func() {})
	timeItem.Disabled = true

	viewsItem := fyne.NewMenuItem(i18n.T("Views: %d", historyItem.Views), func() {})
	viewsItem.Disabled = true

	openItem := fyne.NewMenuItem(i18n.T("Open in browser"), func() {
		if u, err := url.Parse(historyItem.Url); err == nil {
			fyne.CurrentApp().OpenURL(u)
		}
	})

	copyItem := fyne.NewMenuItem(i18n.T("Copy link to clipboard"), func() {
		fyne.CurrentApp().Clipboard().SetContent(historyItem.Url)
	})

	deleteItem := fyne.NewMenuItem(i18n.T("Delete"), func() {
		go func() {
			newHistory, err := m.api.Delete(historyItem.Id)
			if err != nil {
				m.ShowErrorNotification(puush.FormatError(err))
				return
			}
			fyne.Do(func() {
				m.uploadHistory = newHistory
				m.rebuildMenuItems()
				m.notifyListeners()
			})
		}()
	})

	items := []*fyne.MenuItem{timeItem, viewsItem, fyne.NewMenuItemSeparator(), openItem, copyItem}

	// Screenshots saved locally, and files uploaded from disk
	if localCopy := m.config.Capture.LocalCopies[historyItem.Url]; localCopy != "" {
		if _, err := os.Stat(localCopy); err == nil {
			items = append(items, fyne.NewMenuItem(i18n.T("Show in Folder"), func() {
				if err := ShowInFolder(localCopy); err != nil {
					m.ShowErrorNotification(i18n.T("Could not open the folder."))
				}
			}))
		}
	}
	items = append(items, fyne.NewMenuItemSeparator(), deleteItem)

	historyMenuItem := fyne.NewMenuItem(historyLabel(historyItem, time.Now()), nil)
	historyMenuItem.ChildMenu = fyne.NewMenu(historyItem.FileName, items...)
	historyMenuItem.Icon = historyFileIcon(historyItem.FileName)
	return historyMenuItem
}

// historyLabel is the name of an upload in the menu, with when it was
// uploaded: the time for today's uploads, otherwise the date.
func historyLabel(item *puush.HistoryItem, now time.Time) string {
	name := item.FileName
	if runes := []rune(name); len(runes) > 40 {
		name = string(runes[:25]) + "…" + string(runes[len(runes)-12:])
	}

	when := i18n.ShortDate(item.Time)
	if y, m, d := item.Time.Date(); y == now.Year() && m == now.Month() && d == now.Day() {
		when = item.Time.Format("15:04")
	}
	return escapeMenuLabel(name) + "   ·   " + when
}

// escapeMenuLabel keeps underscores from turning into keyboard shortcuts
func escapeMenuLabel(label string) string {
	return strings.ReplaceAll(label, "_", "__")
}

// FileIcon is an icon for the type of a file, by its name.
func FileIcon(filename string) fyne.Resource {
	return historyFileIcon(filename)
}

func historyFileIcon(filename string) fyne.Resource {
	name := strings.ToLower(filename)
	extension := filepath.Ext(name)

	switch extension {
	// Images
	case ".png", ".jpg", ".jpeg", ".jpe", ".jfif",
		".gif", ".bmp", ".dib",
		".webp", ".svg", ".svgz",
		".ico", ".icns",
		".tif", ".tiff",
		".avif", ".heic", ".heif",
		".raw", ".dng", ".cr2", ".cr3", ".nef", ".arw":
		return theme.BrokenImageIcon()

	// Audio
	case ".mp3", ".wav", ".wave",
		".flac", ".ogg", ".oga", ".opus",
		".aac", ".m4a", ".m4b",
		".wma", ".aiff", ".aif",
		".mid", ".midi",
		".amr", ".ac3", ".ape":
		return theme.FileAudioIcon()

	// Playlists
	case ".m3u", ".m3u8", ".pls", ".xspf":
		return theme.MediaMusicIcon()

	// Video
	case ".mp4", ".m4v", ".mkv", ".webm",
		".avi", ".mov", ".qt",
		".wmv", ".flv",
		".mpeg", ".mpg", ".mpe",
		".ogv", ".3gp", ".3g2",
		".mts", ".m2ts", ".vob":
		return theme.FileVideoIcon()

	// Documents
	case ".pdf",
		".doc", ".docx", ".docm",
		".odt", ".ott",
		".rtf",
		".pages",
		".tex":
		return theme.DocumentIcon()

	// Spreadsheets
	case ".xls", ".xlsx", ".xlsm",
		".ods", ".ots",
		".numbers",
		".csv", ".tsv":
		return theme.DocumentIcon()

	// Presentations
	case ".ppt", ".pptx", ".pptm",
		".odp", ".otp",
		".key":
		return theme.DocumentIcon()

	// E-books
	case ".epub", ".mobi", ".azw", ".azw3", ".fb2":
		return theme.DocumentIcon()

	// Calendars
	case ".ics", ".ical", ".ifb", ".vcs":
		return theme.CalendarIcon()

	// Databases and storage
	case ".db", ".db3",
		".sqlite", ".sqlite3",
		".mdb", ".accdb",
		".sqlitedb",
		".bak", ".dump":
		return theme.StorageIcon()

	// Disk and virtual-machine images
	case ".iso", ".img", ".dmg",
		".vhd", ".vhdx",
		".vmdk", ".vdi", ".qcow", ".qcow2":
		return theme.StorageIcon()

	// Email and mailbox files
	case ".eml", ".msg", ".mbox", ".pst", ".ost":
		return theme.MailAttachmentIcon()

	// Configuration files
	case ".ini", ".cfg", ".conf", ".config",
		".properties", ".prefs",
		".env",
		".desktop",
		".reg":
		return theme.SettingsIcon()

	// Structured text and data
	case ".json", ".jsonl", ".ndjson",
		".xml", ".xsd", ".xsl", ".xslt",
		".yaml", ".yml",
		".toml",
		".plist",
		".graphql", ".gql",
		".proto":
		return theme.FileTextIcon()

	// Markup and documentation
	case ".txt", ".text",
		".md", ".markdown", ".mdx",
		".rst", ".adoc", ".asciidoc",
		".html", ".htm", ".xhtml",
		".css", ".scss", ".sass", ".less":
		return theme.FileTextIcon()

	// Source code
	case ".go",
		".c", ".h", ".cc", ".cpp", ".cxx", ".hpp",
		".cs",
		".java", ".kt", ".kts", ".scala",
		".rs",
		".py", ".pyw", ".pyi",
		".rb",
		".php",
		".js", ".jsx", ".mjs", ".cjs",
		".ts", ".tsx",
		".vue", ".svelte",
		".swift", ".m", ".mm",
		".dart",
		".lua",
		".pl", ".pm",
		".r",
		".fs", ".fsx", ".fsi",
		".vb":
		return theme.FileTextIcon()

	// Shell scripts
	case ".sh", ".bash", ".zsh", ".fish",
		".bat", ".cmd", ".ps1",
		".nu":
		return theme.FileTextIcon()

	// Logs and patches
	case ".log", ".patch", ".diff":
		return theme.FileTextIcon()

	// SQL scripts are text rather than database files
	case ".sql":
		return theme.FileTextIcon()

	// Design and graphics project files
	case ".psd", ".psb",
		".ai", ".eps",
		".xcf", ".kra",
		".blend",
		".aseprite",
		".sketch", ".fig",
		".ase", ".aco", ".gpl", ".pal":
		return theme.ColorPaletteIcon()

	// Archives and compressed files
	case ".zip", ".7z", ".rar",
		".tar", ".gz", ".gzip",
		".bz", ".bz2",
		".xz", ".lz", ".lz4",
		".zst", ".zstd",
		".cab", ".arj",
		".tgz", ".tbz", ".tbz2", ".txz":
		return theme.FileApplicationIcon()

	// Executables and libraries
	case ".exe", ".com", ".scr",
		".dll", ".sys", ".drv",
		".so", ".dylib",
		".bin", ".elf",
		".appimage":
		return theme.FileApplicationIcon()

	// Installers and packages
	case ".msi", ".msix", ".msixbundle",
		".deb", ".rpm",
		".pkg", ".mpkg",
		".apk", ".aab",
		".appx", ".appxbundle",
		".flatpak", ".snap":
		return theme.FileApplicationIcon()

	// Compiled code and application bundles
	case ".jar", ".war", ".ear",
		".class",
		".wasm",
		".pyc", ".pyo",
		".o", ".obj",
		".a", ".lib":
		return theme.FileApplicationIcon()

	// Fonts
	case ".ttf", ".otf",
		".woff", ".woff2",
		".eot":
		return theme.FileApplicationIcon()

	// Shortcuts and links
	case ".lnk", ".url", ".webloc":
		return theme.FileApplicationIcon()

	// Incomplete downloads
	case ".part", ".partial", ".crdownload", ".download":
		return theme.DownloadIcon()

	default:
		return theme.FileIcon()
	}
}
