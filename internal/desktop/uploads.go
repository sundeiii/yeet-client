package desktop

import (
	"errors"
	"fmt"
	"github.com/sundeiii/yeet-client/internal/i18n"
	"image/color"
	"io"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/sqweek/dialog"

	"github.com/sundeiii/yeet-client/internal/tray"
	"github.com/sundeiii/yeet-client/pkg/puush"
)

const uploadsPageSize = 30

// uploadsView is the app window's list of everything in the account, with
// search, thumbnails, and copy / open / delete for each file.
// Its fields are only touched on the main thread.
type uploadsView struct {
	ui *UI

	content fyne.CanvasObject
	list    *widget.List
	search  *widget.Entry
	status  *widget.Label
	more    *widget.Button

	items   []*puush.Upload
	total   int
	query   string
	loaded  bool
	loading bool
	// Answers to older requests are dropped, e.g. while typing a search
	generation  int
	searchTimer *time.Timer
	reloadTimer *time.Timer

	thumbnails map[int]fyne.Resource
	requested  map[int]bool
	thumbSlots chan struct{}
}

func (ui *UI) buildUploadsTab(w fyne.Window) *uploadsView {
	v := &uploadsView{
		ui:         ui,
		thumbnails: map[int]fyne.Resource{},
		requested:  map[int]bool{},
		thumbSlots: make(chan struct{}, 4),
	}

	v.search = widget.NewEntry()
	v.search.SetPlaceHolder(i18n.T("Search by filename"))
	v.search.OnChanged = func(text string) {
		// Wait for a pause in typing
		if v.searchTimer != nil {
			v.searchTimer.Stop()
		}
		v.searchTimer = time.AfterFunc(350*time.Millisecond, func() {
			fyne.Do(func() { v.load(strings.TrimSpace(text), false) })
		})
	}
	refresh := widget.NewButtonWithIcon("", theme.ViewRefreshIcon(), func() { v.load(v.query, false) })

	v.status = widget.NewLabel("")
	v.more = widget.NewButton(i18n.T("Load more"), func() { v.load(v.query, true) })
	v.more.Hide()

	v.list = widget.NewList(
		func() int { return len(v.items) },
		v.createRow,
		v.updateRow,
	)
	v.list.OnSelected = func(id widget.ListItemID) {
		// Rows have their own buttons; selecting would only leave a highlight
		v.list.Unselect(id)
	}

	top := container.NewBorder(nil, nil, nil, refresh, v.search)
	bottom := container.NewBorder(nil, nil, nil, v.more, v.status)
	v.content = container.NewBorder(container.NewPadded(top), bottom, nil, nil, v.list)
	return v
}

// uploadRow is one row of the list, reused as the list scrolls.
type uploadRow struct {
	widget.BaseWidget

	thumb  *canvas.Image
	name   *widget.Label
	meta   *canvas.Text
	copy   *widget.Button
	open   *widget.Button
	folder *widget.Button
	remove *widget.Button
}

func (v *uploadsView) createRow() fyne.CanvasObject {
	row := &uploadRow{
		thumb:  canvas.NewImageFromResource(theme.FileIcon()),
		name:   widget.NewLabel(""),
		meta:   canvas.NewText("", color.NRGBA{R: 110, G: 110, B: 110, A: 255}),
		copy:   widget.NewButtonWithIcon("", theme.ContentCopyIcon(), nil),
		open:   widget.NewButtonWithIcon("", theme.ComputerIcon(), nil),
		folder: widget.NewButtonWithIcon("", theme.FolderOpenIcon(), nil),
		remove: widget.NewButtonWithIcon("", theme.DeleteIcon(), nil),
	}
	row.thumb.FillMode = canvas.ImageFillContain
	row.thumb.SetMinSize(fyne.NewSquareSize(44))
	row.name.Truncation = fyne.TextTruncateEllipsis
	row.meta.TextSize = 11
	row.ExtendBaseWidget(row)
	return row
}

func (row *uploadRow) CreateRenderer() fyne.WidgetRenderer {
	text := container.NewVBox(row.name, container.NewPadded(row.meta))
	buttons := container.NewHBox(row.copy, row.open, row.folder, row.remove)
	return widget.NewSimpleRenderer(container.NewBorder(nil, nil, row.thumb, buttons, text))
}

func (v *uploadsView) updateRow(id widget.ListItemID, object fyne.CanvasObject) {
	row, ok := object.(*uploadRow)
	if !ok || id >= len(v.items) {
		return
	}
	upload := v.items[id]

	row.name.SetText(upload.Filename)
	meta := []string{typeLabel(upload.Filename), upload.SizeHumanReadable(), views(upload.Views), i18n.Date(upload.Created.Local())}
	if upload.Pool != "" {
		meta = append(meta, upload.Pool)
	}
	row.meta.Text = strings.Join(meta, "  ·  ")
	row.meta.Refresh()

	row.thumb.Resource = v.thumbnail(upload)
	row.thumb.Refresh()

	row.copy.OnTapped = func() {
		fyne.CurrentApp().Clipboard().SetContent(upload.Url)
		v.status.SetText(i18n.T("Copied the link to %s", upload.Filename))
	}
	row.open.OnTapped = func() { OpenBrowser(upload.Url) }

	localCopy := v.ui.config.Capture.LocalCopies[upload.Url]
	row.folder.OnTapped = func() { tray.ShowInFolder(localCopy) }
	if localCopy == "" {
		row.folder.Hide()
	} else {
		row.folder.Show()
	}

	row.remove.OnTapped = func() { v.confirmDelete(upload) }
}

// thumbnail returns the picture for a row: the server's thumbnail for
// images, videos and audio with cover art, or an icon for the file type.
func (v *uploadsView) thumbnail(upload *puush.Upload) fyne.Resource {
	if resource, ok := v.thumbnails[upload.Id]; ok {
		return resource
	}
	icon := tray.FileIcon(upload.Filename)
	if upload.Kind != "image" && upload.Kind != "video" && upload.Kind != "audio" {
		return icon
	}
	if !v.requested[upload.Id] {
		v.requested[upload.Id] = true
		go v.fetchThumbnail(upload.Id)
	}
	return icon
}

func (v *uploadsView) fetchThumbnail(uploadId int) {
	v.thumbSlots <- struct{}{}
	defer func() { <-v.thumbSlots }()

	body, err := v.ui.api.Thumbnail(uploadId)
	if err != nil {
		return
	}
	defer body.Close()
	data, err := io.ReadAll(io.LimitReader(body, 2<<20))
	if err != nil || len(data) == 0 {
		return
	}

	resource := fyne.NewStaticResource(fmt.Sprintf("thumbnail-%d", uploadId), data)
	fyne.Do(func() {
		v.thumbnails[uploadId] = resource
		v.list.Refresh()
	})
}

func (v *uploadsView) confirmDelete(upload *puush.Upload) {
	// The native dialog waits for an answer, so not on the main thread
	go func() {
		confirmed := dialog.Message(i18n.T("Delete %s? Its link will stop working."), upload.Filename).
			Title(i18n.T("Delete upload")).
			YesNo()
		if !confirmed {
			return
		}

		_, err := v.ui.api.Delete(upload.Id)
		fyne.Do(func() {
			if err != nil {
				v.status.SetText(i18n.T("Could not delete %s: %s", upload.Filename, puush.FormatError(err)))
				return
			}
			v.status.SetText(i18n.T("Deleted %s", upload.Filename))
			v.load(v.query, false)
		})
		v.ui.tray.RefreshHistory()
	}()
}

// loadIfEmpty loads the first page the first time the page is shown.
func (v *uploadsView) loadIfEmpty() {
	if !v.loaded && !v.loading {
		v.load(v.query, false)
	}
}

// reloadSoon refreshes the list after uploads changed, once things settle.
func (v *uploadsView) reloadSoon() {
	if !v.loaded {
		return
	}
	if v.reloadTimer != nil {
		v.reloadTimer.Stop()
	}
	v.reloadTimer = time.AfterFunc(time.Second, func() {
		fyne.Do(func() {
			if v.ui.uploads == v {
				v.load(v.query, false)
			}
		})
	})
}

// load fetches the first page for a search, or the next page with more.
func (v *uploadsView) load(query string, more bool) {
	if !v.ui.api.Account.Credentials.HasApiKey() {
		v.items = nil
		v.list.Refresh()
		v.status.SetText(i18n.T("Log in to see your uploads here."))
		v.more.Hide()
		return
	}

	v.generation++
	generation := v.generation
	offset := 0
	if more {
		offset = len(v.items)
	}
	v.loading = true
	v.status.SetText(i18n.T("Loading..."))

	go func() {
		page, err := v.ui.api.Uploads(query, offset, uploadsPageSize)
		fyne.Do(func() {
			if generation != v.generation {
				return
			}
			v.loading = false
			if err != nil {
				switch {
				case errors.Is(err, puush.ErrNotSupported):
					v.status.SetText(i18n.T("This server doesn't support browsing uploads in the app."))
				default:
					v.status.SetText(i18n.T("Could not load your uploads: %s", puush.FormatError(err)))
				}
				return
			}

			v.loaded = true
			v.query = query
			v.total = page.Total
			if more {
				v.items = append(v.items, page.Uploads...)
			} else {
				v.items = page.Uploads
				v.list.ScrollToTop()
			}
			v.list.Refresh()

			switch {
			case v.total == 0 && query != "":
				v.status.SetText(i18n.T("Nothing matches that search."))
			case v.total == 0:
				v.status.SetText(i18n.T("Nothing uploaded yet."))
			default:
				v.status.SetText(i18n.T("Showing %d of %d", len(v.items), v.total))
			}
			if len(v.items) < v.total {
				v.more.Show()
			} else {
				v.more.Hide()
			}
		})
	}()
}

// typeLabel is a file's extension in capitals, like "PNG".
func typeLabel(filename string) string {
	dot := strings.LastIndex(filename, ".")
	if dot < 0 || len(filename)-dot > 6 {
		return i18n.T("FILE")
	}
	return strings.ToUpper(filename[dot+1:])
}

func views(count int) string {
	if count == 1 {
		return i18n.T("1 view")
	}
	return i18n.T("%d views", count)
}
