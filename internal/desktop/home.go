package desktop

import (
	"fmt"
	"image/color"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/sundeiii/yeet-client/assets"
)

var (
	selectionActionIcon  = fyne.NewStaticResource("icon-selection.png", assets.SelectionIconData)
	fullscreenActionIcon = fyne.NewStaticResource("icon-fullscreen.png", assets.FullscreenIconData)
	windowActionIcon     = fyne.NewStaticResource("icon-window.png", assets.WindowIconData)
	clipboardActionIcon  = fyne.NewStaticResource("icon-clipboard.png", assets.ClipboardIconData)
	uploadActionIcon     = fyne.NewStaticResource("icon-upload.png", assets.UploadIconData)
)

// buildHomeTab is the first page of the app window: the account at a glance,
// quick actions, where uploads go, and uploads that need attention.
func (ui *UI) buildHomeTab(w fyne.Window, goToAccount func()) (fyne.CanvasObject, func()) {
	content := container.NewVBox()
	refresh := func() {
		content.Objects = ui.homeContent(w, goToAccount)
		content.Refresh()
	}
	refresh()
	return container.NewVScroll(content), refresh
}

func (ui *UI) homeContent(w fyne.Window, goToAccount func()) []fyne.CanvasObject {
	logo := canvas.NewImageFromResource(puushIcon)
	logo.FillMode = canvas.ImageFillContain
	logo.SetMinSize(fyne.NewSquareSize(48))

	title := canvas.NewText("puush", color.Black)
	title.TextSize = 20

	status := "You're not logged in yet."
	if ui.config.Account.HasCredentials() {
		status = fmt.Sprintf("Logged in as %s  ·  %s account  ·  %s used",
			ui.config.Account.Username,
			ui.config.Account.Type.String(),
			ui.config.Account.DiskUsageHumanReadable(),
		)
	}
	statusLabel := widget.NewLabel(status)
	statusLabel.Truncation = fyne.TextTruncateEllipsis

	header := container.NewBorder(nil, nil, logo, nil, container.NewVBox(title, statusLabel))
	objects := []fyne.CanvasObject{widget.NewSeparator(), container.NewPadded(header)}

	if !ui.config.Account.HasCredentials() {
		intro := widget.NewLabel("Log in to start sharing screenshots and files. If you don't have an account yet, you can sign up for free.")
		intro.Wrapping = fyne.TextWrapWord
		login := NewBorderedButton("Log in", goToAccount)
		return append(objects, createGroup("Get started", container.NewVBox(intro, container.NewGridWrap(fyne.NewSize(160, 30), login))))
	}

	// The window would end up in the screenshot, so it hides first
	hideThen := func(action func()) func() {
		return func() {
			w.Hide()
			go func() {
				time.Sleep(300 * time.Millisecond)
				action()
			}()
		}
	}

	captureArea := actionButton("Capture Area", selectionActionIcon, hideThen(ui.tray.UploadAreaScreenshot))
	captureDesktop := actionButton("Capture Desktop", fullscreenActionIcon, hideThen(ui.tray.UploadDesktopScreenshot))
	captureWindow := actionButton("Capture Window", windowActionIcon, hideThen(ui.tray.UploadWindowScreenshot))
	captureLast := actionButton("Last Area Again", selectionActionIcon, hideThen(ui.tray.UploadLastAreaScreenshot))
	if !ui.tray.HasLastArea() {
		captureLast.Instance.Disable()
	}
	uploadClipboard := actionButton("Upload Clipboard", clipboardActionIcon, func() { go ui.tray.UploadFromClipboard() })
	uploadFile := actionButton("Upload File", uploadActionIcon, ui.tray.UploadFileFromDialog)

	actions := container.NewGridWithColumns(3,
		captureArea, captureDesktop, captureWindow,
		captureLast, uploadClipboard, uploadFile,
	)
	objects = append(objects, createGroup("Quick actions", actions))

	// Where uploads go
	if pools := ui.tray.Pools(); len(pools) > 0 {
		names := make([]string, len(pools))
		for i, pool := range pools {
			names[i] = pool.Name
		}
		poolSelect := widget.NewSelect(names, nil)
		if current := ui.tray.UploadPool(); current != nil {
			poolSelect.SetSelected(current.Name)
		}
		poolSelect.OnChanged = func(name string) {
			for _, pool := range pools {
				if pool.Name == name {
					ui.tray.SetUploadPool(pool)
				}
			}
		}
		hint := widget.NewLabel("New screenshots and files go into this pool.")
		objects = append(objects, createGroup("Upload to", container.NewBorder(nil, nil, container.NewGridWrap(fyne.NewSize(200, 36), poolSelect), nil, hint)))
	}

	// Uploads that need attention
	var attention []fyne.CanvasObject
	if name, uploading := ui.tray.ActiveUpload(); uploading {
		label := widget.NewLabel("Uploading " + name + "...")
		label.Truncation = fyne.TextTruncateEllipsis
		cancel := NewBorderedButton("Cancel", ui.tray.CancelUpload)
		attention = append(attention, container.NewBorder(nil, nil, nil, container.NewGridWrap(fyne.NewSize(100, 30), cancel), label))
	}
	if failed := ui.tray.FailedUploads(); len(failed) > 0 {
		text := fmt.Sprintf("%d uploads failed and are waiting to be retried.", len(failed))
		if len(failed) == 1 {
			text = fmt.Sprintf("%s failed to upload and is waiting to be retried.", failed[0])
		}
		label := widget.NewLabel(text)
		label.Wrapping = fyne.TextWrapWord
		retry := NewBorderedButton("Retry", func() { go ui.tray.RetryFailedUploads() })
		discard := NewBorderedButton("Discard", func() { go ui.tray.DiscardFailedUploads() })
		buttons := container.NewGridWrap(fyne.NewSize(100, 30), retry, discard)
		attention = append(attention, container.NewBorder(nil, nil, nil, buttons, label))
	}
	if len(attention) > 0 {
		objects = append(objects, createGroup("Uploads", container.NewVBox(attention...)))
	}

	tip := widget.NewLabel("Tip: copied files and images can be uploaded with Upload Clipboard, and files can be uploaded by right-clicking them.")
	tip.Wrapping = fyne.TextWrapWord
	objects = append(objects, layout.NewSpacer(), container.NewPadded(tip))
	return objects
}

func actionButton(label string, icon fyne.Resource, action func()) *BorderedButton {
	button := NewBorderedButton(label, action)
	button.Instance.SetIcon(icon)
	button.Instance.Alignment = widget.ButtonAlignLeading
	return button
}
