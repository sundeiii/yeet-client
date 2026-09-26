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
	if !grouped && prev != nil {
		// The space between people's messages belongs to neither, so it
		// never lights up
		gap := canvas.NewRectangle(color.Transparent)
		gap.SetMinSize(fyne.NewSize(0, 14))
		v.thread.Add(gap)
	}
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

// messageRow is one message, lit up while the mouse is over it (the
// threadArea decides which one that is), with its menu on right-click.
type messageRow struct {
	widget.BaseWidget

	message   *puush.ChatMessage
	content   fyne.CanvasObject
	highlight *canvas.Rectangle
	hoverTime *canvas.Text // the time, on messages under someone's name
	menu      *fyne.Menu
}

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
		when := canvas.NewText(headerTime(message, time.Now()), quietTextColor)
		when.TextSize = 10
		nameLine := container.New(layout.NewCustomPaddedLayout(0, 0, 4, 0),
			container.NewHBox(nameText, container.NewCenter(when)))

		avatar := v.avatarImage(avatarLink)
		// No room above the avatar: the lit-up area starts at its top edge
		avatarBox := container.New(layout.NewCustomPaddedLayout(0, 0, 10, 10), container.NewVBox(avatar))
		content = container.NewBorder(nil, nil, avatarBox, nil,
			container.New(layout.NewCustomPaddedVBoxLayout(1), nameLine, body))
		if message.Reply != nil && !message.Removed {
			// Like Discord: the message it answers on a line above the name,
			// joined to the avatar by a curved line
			content = container.New(layout.NewCustomPaddedVBoxLayout(2), v.replyLine(message.Reply), content)
		}
	}

	row := &messageRow{message: message, highlight: canvas.NewRectangle(hoverColor)}
	if !header {
		// Where the avatar would be, the time shows while the mouse is over it
		gap := canvas.NewRectangle(color.Transparent)
		gap.SetMinSize(fyne.NewSize(avatarColumn, 0))
		// Always there, but see-through until the mouse is over the message,
		// so showing it doesn't move anything
		row.hoverTime = canvas.NewText(clockTime(message), color.Transparent)
		row.hoverTime.TextSize = 10
		row.hoverTime.Alignment = fyne.TextAlignTrailing
		gutter := container.NewStack(gap, container.NewVBox(
			container.New(layout.NewCustomPaddedLayout(3, 0, 0, 8), row.hoverTime)))
		content = container.NewBorder(nil, nil, gutter, nil, body)
	}
	row.content = content
	row.highlight.Hide()
	row.menu = v.messageMenu(message)
	row.ExtendBaseWidget(row)
	return row
}

func (row *messageRow) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(container.NewStack(row.highlight, row.content))
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

// setHovered lights the message up, and shows its time on the left.
func (row *messageRow) setHovered(on bool) {
	row.highlight.Hidden = !on
	row.highlight.Refresh()
	if row.hoverTime != nil {
		row.hoverTime.Color = color.Transparent
		if on {
			row.hoverTime.Color = quietTextColor
		}
		row.hoverTime.Refresh()
	}
}

// ---- Hovering, like Discord ----

// threadArea follows the mouse over the whole thread and decides which
// message is hovered. There's one action bar, on the hovered message's top
// right edge, sticking up over the message above. While the mouse is on the
// bar, its message stays hovered, even over the part that sticks out; that's
// why this isn't left to each message on its own.
type threadArea struct {
	widget.BaseWidget

	v       *messagesView
	bar     *fyne.Container
	buttons *fyne.Container
	overlay *fyne.Container
	hovered *messageRow
	inside  bool
	mouse   fyne.Position // where the mouse is, on the screen
}

var _ desktop.Hoverable = (*threadArea)(nil)

func newThreadArea(v *messagesView) *threadArea {
	area := &threadArea{v: v, buttons: container.NewHBox()}
	background := canvas.NewRectangle(cardColor)
	background.StrokeColor = cardBorderColor
	background.StrokeWidth = 1
	background.CornerRadius = 6
	area.bar = container.NewStack(background, container.New(layout.NewCustomPaddedLayout(1, 1, 2, 2), area.buttons))
	area.bar.Hide()
	area.overlay = container.New(&barLayout{area: area}, area.bar)
	area.ExtendBaseWidget(area)
	return area
}

func (area *threadArea) CreateRenderer() fyne.WidgetRenderer {
	// The bar is drawn over the messages, so it can overlap the one above
	return widget.NewSimpleRenderer(container.NewStack(area.v.thread, area.overlay))
}

func (area *threadArea) MouseIn(event *desktop.MouseEvent) {
	area.inside = true
	area.track(event.AbsolutePosition)
}

func (area *threadArea) MouseMoved(event *desktop.MouseEvent) {
	area.track(event.AbsolutePosition)
}

func (area *threadArea) MouseOut() {
	area.inside = false
	area.hover(nil)
}

// recheck looks again at what's under the mouse, after scrolling or when
// messages were added, since the mouse didn't move but the messages did.
func (area *threadArea) recheck() {
	if area.inside {
		area.track(area.mouse)
	} else {
		area.hover(nil)
	}
}

// forget drops the hovered message, e.g. before the thread is rebuilt.
func (area *threadArea) forget() {
	area.hovered = nil
	area.bar.Hide()
}

func (area *threadArea) track(mouse fyne.Position) {
	area.mouse = mouse
	origin := fyne.CurrentApp().Driver().AbsolutePositionForObject(area)
	at := mouse.Subtract(origin)

	// On the bar: its message stays hovered
	if area.hovered != nil && area.bar.Visible() && inside(at, area.bar.Position(), area.bar.Size()) {
		return
	}
	for _, object := range area.v.thread.Objects {
		row, ok := object.(*messageRow)
		if ok && object.Visible() && at.Y >= row.Position().Y && at.Y < row.Position().Y+row.Size().Height {
			area.hover(row)
			return
		}
	}
	area.hover(nil)
}

func inside(at, position fyne.Position, size fyne.Size) bool {
	return at.X >= position.X && at.X < position.X+size.Width && at.Y >= position.Y && at.Y < position.Y+size.Height
}

func (area *threadArea) hover(row *messageRow) {
	if row == area.hovered {
		return
	}
	if area.hovered != nil {
		area.hovered.setHovered(false)
	}
	area.hovered = row
	if row == nil || row.menu == nil {
		if row != nil {
			row.setHovered(true)
		}
		area.bar.Hide()
		return
	}
	row.setHovered(true)

	// The bar's buttons for this message
	message, v := row.message, area.v
	area.buttons.RemoveAll()
	area.buttons.Add(newTapArea(toolIcon(theme.MailReplyIcon()), func() { v.startReply(message) }))
	if message.Mine && !message.Removed {
		area.buttons.Add(newTapArea(toolIcon(theme.DocumentCreateIcon()), func() { v.startEditing(message) }))
	}
	var more *tapArea
	more = newTapArea(toolIcon(theme.MoreHorizontalIcon()), func() {
		position := fyne.CurrentApp().Driver().AbsolutePositionForObject(more)
		row.showMenu(position.AddXY(0, more.Size().Height))
	})
	area.buttons.Add(more)
	area.bar.Show()
	area.overlay.Refresh()
}

// barLayout puts the bar on the hovered message's top right edge, half over
// the message above, like Discord.
type barLayout struct{ area *threadArea }

func (l *barLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	row := l.area.hovered
	if row == nil {
		return
	}
	for _, object := range objects {
		min := object.MinSize()
		object.Resize(min)
		y := max(row.Position().Y-min.Height/2, 0)
		object.Move(fyne.NewPos(size.Width-min.Width-16, y))
	}
}

func (*barLayout) MinSize([]fyne.CanvasObject) fyne.Size { return fyne.NewSize(0, 0) }

// tapArea is something to click that isn't a hover target itself, so the
// thread keeps following the mouse over it.
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
	icon.SetMinSize(fyne.NewSquareSize(18))
	return container.New(layout.NewCustomPaddedLayout(5, 5, 6, 6), icon)
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

// replyLine shows the message a reply answers, above the name: a curved
// line from the avatar, then their small avatar, name and the message.
func (v *messagesView) replyLine(reply *puush.ChatReply) fyne.CanvasObject {
	name, avatarLink, nameColor := v.displayName(), v.otherAvatar(), otherNameColor
	if reply.Mine {
		name, avatarLink, nameColor = v.ownName(), v.ownAvatar(), ownNameColor
	}
	text := reply.Text
	if reply.Removed {
		text = i18n.T("Message deleted")
	}
	avatar := v.avatarImage(avatarLink)
	avatar.SetMinSize(fyne.NewSquareSize(16))
	avatar.CornerRadius = 8
	who := canvas.NewText(name, nameColor)
	who.TextSize = 11
	what := canvas.NewText(fitText(oneLine(text), quoteWidth, 11), quietTextColor)
	what.TextSize = 11

	curve := container.New(&replyCurve{}, canvas.NewLine(cardBorderColor), canvas.NewLine(cardBorderColor))
	return container.NewBorder(nil, nil, curve, nil,
		container.New(layout.NewCustomPaddedLayout(2, 0, 4, 0),
			container.NewHBox(container.NewCenter(avatar), who, what)))
}

// replyCurve draws the line from above the avatar to the reply: up from the
// avatar's middle, then right.
type replyCurve struct{}

func (*replyCurve) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	x := float32(10 + avatarSize/2)
	middle := size.Height / 2
	up, across := objects[0].(*canvas.Line), objects[1].(*canvas.Line)
	up.StrokeWidth, across.StrokeWidth = 2, 2
	up.Position1, up.Position2 = fyne.NewPos(x, size.Height+4), fyne.NewPos(x, middle)
	across.Position1, across.Position2 = fyne.NewPos(x, middle), fyne.NewPos(size.Width-2, middle)
}

func (*replyCurve) MinSize([]fyne.CanvasObject) fyne.Size {
	// As tall as the small avatar, so the lit-up area starts at its top
	return fyne.NewSize(avatarColumn, 16)
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

// headerTime is the time next to a name, like Discord: "Today at 16:58",
// "Yesterday at 21:39", or the date.
func headerTime(message *puush.ChatMessage, now time.Time) string {
	if message.At <= 0 {
		return message.Time // an older server
	}
	sent := time.Unix(message.At, 0).Local()
	switch {
	case sameDay(sent, now):
		return i18n.T("Today at %s", sent.Format("15:04"))
	case sameDay(sent, now.AddDate(0, 0, -1)):
		return i18n.T("Yesterday at %s", sent.Format("15:04"))
	}
	return i18n.Date(sent) + " " + sent.Format("15:04")
}

// clockTime is just the time of day, for messages under someone's name.
func clockTime(message *puush.ChatMessage) string {
	if message.At <= 0 {
		return message.Time
	}
	return time.Unix(message.At, 0).Local().Format("15:04")
}
