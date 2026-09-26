package desktop

import (
	"bytes"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"net/url"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/sundeiii/yeet-client/internal/i18n"
	"github.com/sundeiii/yeet-client/pkg/puush"
)

// The thread looks like Discord: no bubbles, but rows with an avatar, the
// name and the time above a group of messages from the same person. The row
// under the mouse lights up and gets a small bar with Reply and "...".

const (
	avatarSize   = 36
	avatarColumn = 56 // the avatar and the room around it
	previewWidth = 320
	previewMax   = 220
)

// addMessage adds a message to the thread, under the previous one's name
// when it's from the same person shortly after.
func (v *messagesView) addMessage(message *puush.ChatMessage) {
	prev := v.prev
	v.prev = message

	var sent, prevSent time.Time
	if message.At > 0 {
		sent = time.Unix(message.At, 0).Local()
	}
	if prev != nil && prev.At > 0 {
		prevSent = time.Unix(prev.At, 0).Local()
	}

	// A line with the day when the date changes
	if !sent.IsZero() && (prev == nil || prevSent.IsZero() || !sameDay(sent, prevSent)) {
		v.thread.Add(dayLine(sent, time.Now()))
		prev = nil
	}

	grouped := prev != nil && prev.Mine == message.Mine && message.Reply == nil &&
		(sent.IsZero() || prevSent.IsZero() || sent.Sub(prevSent) < groupWindow)
	v.thread.Add(v.messageRow(message, !grouped))
}

func (v *messagesView) showSeen() {
	label := canvas.NewText(i18n.T("Seen"), quietTextColor)
	label.TextSize = 10
	v.seen = container.New(layout.NewCustomPaddedLayout(-2, 2, avatarColumn+4, 0), label)
	v.thread.Add(v.seen)
}

// dayLine is "Today", "Yesterday" or the date, between the days' messages.
func dayLine(day, now time.Time) fyne.CanvasObject {
	text := i18n.Date(day)
	switch {
	case sameDay(day, now):
		text = i18n.T("Today")
	case sameDay(day, now.AddDate(0, 0, -1)):
		text = i18n.T("Yesterday")
	}
	label := canvas.NewText(text, quietTextColor)
	label.TextSize = 11
	left := canvas.NewRectangle(cardBorderColor)
	left.SetMinSize(fyne.NewSize(0, 1))
	right := canvas.NewRectangle(cardBorderColor)
	right.SetMinSize(fyne.NewSize(0, 1))
	line := container.NewBorder(nil, nil, nil, nil,
		container.NewGridWithColumns(3,
			container.NewVBox(layout.NewSpacer(), left, layout.NewSpacer()),
			container.NewCenter(label),
			container.NewVBox(layout.NewSpacer(), right, layout.NewSpacer())))
	return container.New(layout.NewCustomPaddedLayout(10, 4, 8, 8), line)
}

// ---- One message ----

// messageRow is a message that lights up under the mouse, with its menu
// on right-click or in the bar that shows then.
type messageRow struct {
	widget.BaseWidget

	content   fyne.CanvasObject
	highlight *canvas.Rectangle
	hoverTime *canvas.Text // the time, on messages under someone's name
	actions   fyne.CanvasObject
	menu      *fyne.Menu
}

var _ desktop.Hoverable = (*messageRow)(nil)

func (v *messagesView) messageRow(message *puush.ChatMessage, header bool) fyne.CanvasObject {
	body := container.New(layout.NewCustomPaddedVBoxLayout(0), v.messageContent(message)...)

	var content fyne.CanvasObject
	if header {
		name, avatarLink := v.displayName(), v.otherAvatar()
		nameColor := otherNameColor
		if message.Mine {
			name, avatarLink, nameColor = v.ownName(), v.ownAvatar(), ownNameColor
		}
		nameText := canvas.NewText(name, nameColor)
		nameText.TextSize = 13
		nameText.TextStyle = fyne.TextStyle{Bold: true}
		when := canvas.NewText(messageTime(message, time.Now()), quietTextColor)
		when.TextSize = 10
		nameLine := container.New(layout.NewCustomPaddedLayout(6, 0, 4, 0),
			container.NewHBox(nameText, container.NewCenter(when)))

		avatar := v.avatarImage(avatarLink)
		avatarBox := container.New(layout.NewCustomPaddedLayout(6, 0, 10, 10), container.NewVBox(avatar))
		content = container.NewBorder(nil, nil, avatarBox, nil,
			container.New(layout.NewCustomPaddedVBoxLayout(-4), nameLine, body))
		content = container.New(layout.NewCustomPaddedLayout(6, 0, 0, 0), content)
	}

	row := &messageRow{highlight: canvas.NewRectangle(hoverColor)}
	if !header {
		// Where the avatar would be, the time shows while the mouse is over it
		gap := canvas.NewRectangle(color.Transparent)
		gap.SetMinSize(fyne.NewSize(avatarColumn, 0))
		// Always there, but see-through until the mouse is over the message,
		// so showing it doesn't move anything
		row.hoverTime = canvas.NewText(messageTime(message, time.Now()), color.Transparent)
		row.hoverTime.TextSize = 10
		row.hoverTime.Alignment = fyne.TextAlignTrailing
		gutter := container.NewStack(gap, container.NewVBox(
			container.New(layout.NewCustomPaddedLayout(4, 0, 0, 8), row.hoverTime)))
		content = container.NewBorder(nil, nil, gutter, nil, body)
	}
	row.content = content
	row.highlight.Hide()
	if row.menu = v.messageMenu(message); row.menu != nil {
		var more *tapArea
		reply := newTapArea(toolIcon(theme.MailReplyIcon()), func() { v.startReply(message) })
		more = newTapArea(toolIcon(theme.MoreHorizontalIcon()), func() {
			position := fyne.CurrentApp().Driver().AbsolutePositionForObject(more)
			row.showMenu(position.AddXY(0, more.Size().Height))
		})
		background := canvas.NewRectangle(cardColor)
		background.StrokeColor = cardBorderColor
		background.StrokeWidth = 1
		background.CornerRadius = 5
		row.actions = container.NewStack(background, container.NewHBox(reply, more))
		row.actions.Hide()
	}
	row.ExtendBaseWidget(row)
	return row
}

func (row *messageRow) CreateRenderer() fyne.WidgetRenderer {
	objects := []fyne.CanvasObject{row.highlight, row.content}
	if row.actions != nil {
		objects = append(objects, container.New(&cornerLayout{}, row.actions))
	}
	return widget.NewSimpleRenderer(container.NewStack(objects...))
}

func (row *messageRow) TappedSecondary(event *fyne.PointEvent) {
	row.showMenu(event.AbsolutePosition)
}

func (row *messageRow) showMenu(at fyne.Position) {
	if row.menu == nil {
		return
	}
	if canvas := fyne.CurrentApp().Driver().CanvasForObject(row); canvas != nil {
		widget.ShowPopUpMenuAtPosition(row.menu, canvas, at)
	}
}

func (row *messageRow) MouseIn(*desktop.MouseEvent) {
	row.highlight.Show()
	if row.hoverTime != nil {
		row.hoverTime.Color = quietTextColor
		row.hoverTime.Refresh()
	}
	if row.actions != nil {
		row.actions.Show()
	}
}

func (row *messageRow) MouseMoved(*desktop.MouseEvent) {}

func (row *messageRow) MouseOut() {
	row.highlight.Hide()
	if row.hoverTime != nil {
		row.hoverTime.Color = color.Transparent
		row.hoverTime.Refresh()
	}
	if row.actions != nil {
		row.actions.Hide()
	}
}

// cornerLayout puts the bar in the top right corner of the message, over
// it, without making the row any bigger.
type cornerLayout struct{}

func (*cornerLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, object := range objects {
		min := object.MinSize()
		object.Resize(min)
		// Inside the lit-up message, so it's clearly part of it; centred on
		// messages that are only one short line
		y := float32(3)
		if size.Height < min.Height+6 {
			y = (size.Height - min.Height) / 2
		}
		object.Move(fyne.NewPos(size.Width-min.Width-12, y))
	}
}

func (*cornerLayout) MinSize([]fyne.CanvasObject) fyne.Size { return fyne.NewSize(0, 0) }

// tapArea is something to click that isn't a hover target itself, so the
// message under it stays lit while the mouse is over it.
type tapArea struct {
	widget.BaseWidget

	content fyne.CanvasObject
	onTap   func()
}

func newTapArea(content fyne.CanvasObject, onTap func()) *tapArea {
	area := &tapArea{content: content, onTap: onTap}
	area.ExtendBaseWidget(area)
	return area
}

func (area *tapArea) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(area.content)
}

func (area *tapArea) Tapped(*fyne.PointEvent) {
	if area.onTap != nil {
		area.onTap()
	}
}

func (area *tapArea) Cursor() desktop.Cursor { return desktop.PointerCursor }

func toolIcon(resource fyne.Resource) fyne.CanvasObject {
	icon := canvas.NewImageFromResource(theme.NewThemedResource(resource))
	icon.SetMinSize(fyne.NewSquareSize(14))
	// Small enough to fit inside a message that's one short line
	return container.New(layout.NewCustomPaddedLayout(2, 2, 6, 6), icon)
}

// linkText is a file name that opens the file.
func linkText(text, link string) fyne.CanvasObject {
	label := canvas.NewText(text, theme.Color(theme.ColorNameHyperlink))
	label.TextSize = theme.TextSize()
	return newTapArea(container.New(layout.NewCustomPaddedLayout(2, 2, 4, 4), label), func() {
		if parsed, err := url.Parse(link); err == nil {
			fyne.CurrentApp().OpenURL(parsed)
		}
	})
}

// messageContent is what a message shows, without the name and avatar.
func (v *messagesView) messageContent(message *puush.ChatMessage) []fyne.CanvasObject {
	if message.Removed {
		gone := widget.NewLabel(i18n.T("Message deleted"))
		gone.TextStyle = fyne.TextStyle{Italic: true}
		gone.Importance = widget.LowImportance
		return []fyne.CanvasObject{gone}
	}
	var parts []fyne.CanvasObject
	if message.Reply != nil {
		parts = append(parts, v.replyQuote(message.Reply))
	}
	if message.File != nil {
		parts = append(parts, v.fileCard(message.File))
	}
	if message.Text != "" || message.File == nil {
		segments := []widget.RichTextSegment{&widget.TextSegment{Text: message.Text, Style: widget.RichTextStyleInline}}
		if message.Edited {
			segments = append(segments, &widget.TextSegment{
				Text:  " " + i18n.T("(edited)"),
				Style: widget.RichTextStyle{Inline: true, ColorName: theme.ColorNamePlaceHolder, SizeName: theme.SizeNameCaptionText},
			})
		}
		text := widget.NewRichText(segments...)
		text.Wrapping = fyne.TextWrapWord
		// Lines of a group sit close together, like one paragraph
		parts = append(parts, container.New(layout.NewCustomPaddedLayout(-3, -3, 0, 0), text))
	} else if message.Edited {
		edited := canvas.NewText(i18n.T("(edited)"), quietTextColor)
		edited.TextSize = 10
		parts = append(parts, container.New(layout.NewCustomPaddedLayout(0, 2, 4, 0), edited))
	}
	return parts
}

// replyQuote shows the message a reply answers, in short.
func (v *messagesView) replyQuote(reply *puush.ChatReply) fyne.CanvasObject {
	name := v.displayName()
	if reply.Mine {
		name = i18n.T("You")
	}
	text := reply.Text
	if reply.Removed {
		text = i18n.T("Message deleted")
	}
	who := canvas.NewText(name, theme.Color(theme.ColorNamePrimary))
	who.TextSize = 11
	what := canvas.NewText(fitText(oneLine(text), quoteWidth, 11), quietTextColor)
	what.TextSize = 11
	line := canvas.NewRectangle(cardBorderColor)
	line.SetMinSize(fyne.NewSize(3, 0))
	quote := container.NewBorder(nil, nil, line, nil,
		container.New(layout.NewCustomPaddedLayout(0, 0, 6, 0), container.NewHBox(who, what)))
	return container.New(layout.NewCustomPaddedLayout(4, 2, 4, 0), quote)
}

// fileCard is a file sent in a chat: pictures and videos as a preview, other
// files as a small card with their icon, name and size.
func (v *messagesView) fileCard(file *puush.ChatFile) fyne.CanvasObject {
	open := func() {
		if parsed, err := url.Parse(file.Url); err == nil {
			fyne.CurrentApp().OpenURL(parsed)
		}
	}
	size := canvas.NewText(file.Size, quietTextColor)
	size.TextSize = 11

	if file.Kind == "image" || file.Kind == "video" {
		preview := canvas.NewImageFromResource(theme.MediaPhotoIcon())
		preview.FillMode = canvas.ImageFillContain
		preview.CornerRadius = 6
		preview.SetMinSize(fyne.NewSize(previewWidth, 180))
		v.loadThumb(file.Thumb, preview, true)
		name := linkText(file.Name, file.Url)
		caption := container.NewHBox(name, container.NewCenter(size))
		return container.New(layout.NewCustomPaddedLayout(4, 0, 4, 0), container.NewVBox(
			container.NewHBox(newTapArea(preview, open)),
			caption,
		))
	}

	icon := canvas.NewImageFromResource(theme.FileIcon())
	icon.FillMode = canvas.ImageFillContain
	icon.SetMinSize(fyne.NewSquareSize(32))
	v.loadThumb(file.Thumb, icon, false)
	details := container.New(layout.NewCustomPaddedVBoxLayout(0),
		linkText(file.Name, file.Url),
		container.New(layout.NewCustomPaddedLayout(0, 0, 4, 0), size))
	background := canvas.NewRectangle(cardColor)
	background.StrokeColor = cardBorderColor
	background.StrokeWidth = 1
	background.CornerRadius = 6
	card := container.NewStack(background, container.New(layout.NewCustomPaddedLayout(6, 6, 8, 12),
		container.NewBorder(nil, nil, container.NewCenter(icon), nil, details)))
	return container.New(layout.NewCustomPaddedLayout(4, 2, 4, 0), container.NewHBox(card))
}

// loadThumb shows a file's thumbnail in image once it's downloaded. Previews
// take the picture's shape, at most previewWidth by previewMax.
func (v *messagesView) loadThumb(link string, image *canvas.Image, sized bool) {
	if link == "" {
		return
	}
	show := func(resource fyne.Resource) {
		image.Resource = resource
		if sized {
			if width, height, ok := pictureSize(resource.Content()); ok {
				scale := min(float32(previewWidth)/float32(width), float32(previewMax)/float32(height), 1)
				image.SetMinSize(fyne.NewSize(float32(width)*scale, float32(height)*scale))
			}
		}
		image.Refresh()
	}
	if resource, ok := v.thumbs[link]; ok {
		show(resource)
		return
	}
	go func() {
		data, err := v.ui.api.Picture(link)
		if err != nil {
			return
		}
		// File icons are SVG pictures
		name := "thumb"
		trimmed := bytes.TrimSpace(data)
		if bytes.HasPrefix(trimmed, []byte("<svg")) || bytes.HasPrefix(trimmed, []byte("<?xml")) {
			name = "thumb.svg"
		}
		resource := fyne.NewStaticResource(name, data)
		fyne.Do(func() {
			atBottom := v.nearBottom()
			v.thumbs[link] = resource
			show(resource)
			v.thread.Refresh()
			if atBottom {
				v.scroll.ScrollToBottom()
			}
		})
	}()
}

func pictureSize(data []byte) (int, int, bool) {
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width == 0 || config.Height == 0 {
		return 0, 0, false
	}
	return config.Width, config.Height, true
}

// ---- Names and avatars ----

func (v *messagesView) ownName() string {
	if profile := v.ui.profile; profile != nil && profile.Name != "" {
		return profile.Name
	}
	return i18n.T("You")
}

func (v *messagesView) ownAvatar() string {
	if profile := v.ui.profile; profile != nil {
		return profile.Avatar
	}
	return ""
}

func (v *messagesView) otherAvatar() string {
	for _, chat := range v.chats {
		if strings.EqualFold(chat.With, v.with) && chat.Avatar != "" {
			return chat.Avatar
		}
	}
	return v.ui.api.FormatURL("/u/" + url.PathEscape(v.with) + "/avatar?png=1")
}

// avatarImage is a round avatar, filled in once it's downloaded.
func (v *messagesView) avatarImage(link string) *canvas.Image {
	avatar := canvas.NewImageFromResource(v.avatar(link))
	avatar.FillMode = canvas.ImageFillContain
	avatar.CornerRadius = avatarSize / 2
	avatar.SetMinSize(fyne.NewSquareSize(avatarSize))
	if _, loaded := v.avatars[link]; !loaded && link != "" {
		v.avatarImages[link] = append(v.avatarImages[link], avatar)
	}
	return avatar
}
