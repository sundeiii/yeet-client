package desktop

import (
	"fmt"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/sundeiii/yeet-client/internal/tray"
)

// queueView lists this session's uploads: waiting, uploading (with
// progress), done, failed or cancelled.
type queueView struct {
	ui      *UI
	content fyne.CanvasObject
	list    *widget.List
	empty   *widget.Label
	entries []tray.QueueEntry
}

func (ui *UI) buildQueueTab() *queueView {
	v := &queueView{ui: ui}

	v.list = widget.NewList(
		func() int { return len(v.entries) },
		func() fyne.CanvasObject { return newQueueRow() },
		func(id widget.ListItemID, object fyne.CanvasObject) {
			if row, ok := object.(*queueRow); ok && id < len(v.entries) {
				row.show(v.ui, v.entries[id])
			}
		},
	)
	v.list.OnSelected = func(id widget.ListItemID) { v.list.Unselect(id) }

	v.empty = widget.NewLabel("Nothing uploaded since puush started. Screenshots and files show up here while they upload.")
	v.empty.Wrapping = fyne.TextWrapWord

	clear := widget.NewButton("Clear finished", ui.tray.ClearFinishedUploads)
	retryAll := widget.NewButton("Retry all failed", func() { go ui.tray.RetryFailedUploads() })
	bottom := container.NewHBox(clear, retryAll)

	v.content = container.NewBorder(nil, container.NewPadded(bottom), nil, nil, container.NewStack(v.list, container.NewPadded(v.empty)))
	v.refresh()
	return v
}

func (v *queueView) refresh() {
	v.entries = v.ui.tray.QueueEntries()
	if len(v.entries) == 0 {
		v.empty.Show()
	} else {
		v.empty.Hide()
	}
	v.list.Refresh()
}

// queueRow is one upload in the list.
type queueRow struct {
	widget.BaseWidget

	icon     *widget.Icon
	name     *widget.Label
	detail   *canvas.Text
	progress *widget.ProgressBar
	action   *widget.Button
}

func newQueueRow() *queueRow {
	row := &queueRow{
		icon:     widget.NewIcon(theme.FileIcon()),
		name:     widget.NewLabel(""),
		detail:   canvas.NewText("", color.NRGBA{R: 110, G: 110, B: 110, A: 255}),
		progress: widget.NewProgressBar(),
		action:   widget.NewButton("", nil),
	}
	row.name.Truncation = fyne.TextTruncateEllipsis
	row.detail.TextSize = 11
	row.ExtendBaseWidget(row)
	return row
}

func (row *queueRow) CreateRenderer() fyne.WidgetRenderer {
	text := container.NewVBox(row.name, container.NewPadded(row.detail), row.progress)
	right := container.NewCenter(container.NewGridWrap(fyne.NewSize(96, 32), row.action))
	return widget.NewSimpleRenderer(container.NewBorder(nil, nil, container.NewCenter(row.icon), right, text))
}

func (row *queueRow) show(ui *UI, entry tray.QueueEntry) {
	row.name.SetText(entry.Name)
	row.progress.Hide()
	row.action.Hide()
	row.action.OnTapped = nil

	switch entry.Status {
	case tray.QueueWaiting:
		row.icon.SetResource(theme.HistoryIcon())
		row.detail.Text = "Waiting"
	case tray.QueueUploading:
		row.icon.SetResource(theme.UploadIcon())
		row.detail.Text = fmt.Sprintf("Uploading: %d%% of %s", int(entry.Progress), formatSize(entry.Size))
		row.progress.SetValue(entry.Progress / 100)
		row.progress.Show()
		row.action.SetText("Cancel")
		row.action.OnTapped = ui.tray.CancelUpload
		row.action.Show()
	case tray.QueueDone:
		row.icon.SetResource(theme.ConfirmIcon())
		row.detail.Text = entry.Link
		row.action.SetText("Copy link")
		link := entry.Link
		row.action.OnTapped = func() { fyne.CurrentApp().Clipboard().SetContent(link) }
		row.action.Show()
	case tray.QueueFailed:
		row.icon.SetResource(theme.ErrorIcon())
		row.detail.Text = "Failed: " + entry.Error
		if entry.Retryable {
			id := entry.Id
			row.action.SetText("Retry")
			row.action.OnTapped = func() { go ui.tray.RetryUpload(id) }
			row.action.Show()
		}
	case tray.QueueCancelled:
		row.icon.SetResource(theme.CancelIcon())
		row.detail.Text = "Cancelled"
	}
	row.detail.Refresh()
}

func formatSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}
