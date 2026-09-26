package desktop

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image/color"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/sqweek/dialog"

	"github.com/sundeiii/yeet-client/internal/i18n"
	"github.com/sundeiii/yeet-client/pkg/puush"
)

// Chat bubbles are at most this wide; the text is wrapped to fit.
const bubbleTextWidth = 300

var (
	myBubbleColor    = color.NRGBA{R: 214, G: 232, B: 255, A: 255}
	theirBubbleColor = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	quietTextColor   = color.NRGBA{R: 110, G: 110, B: 110, A: 255}
	onlineColor      = color.NRGBA{R: 46, G: 160, B: 67, A: 255}
)

// messagesView is the app window's chats: the list on the left, the open
// chat on the right. New messages come in over the live connection.
// Its fields are only touched on the main thread.
type messagesView struct {
	ui *UI

	content    fyne.CanvasObject
	list       *widget.List
	listStatus *widget.Label
	newChat    *widget.Entry
	chats      []*puush.Chat

	// The open chat
	with        string // username, as the server writes it
	generation  int    // answers for a chat that's no longer open are dropped
	lastId      int
	hint        bool // the thread shows "Say hi!"
	fetching    bool
	fetchAgain  bool
	placeholder fyne.CanvasObject
	chatPane    fyne.CanvasObject
	title       *widget.Label
	presence    *canvas.Text
	profile     *widget.Hyperlink
	thread      *fyne.Container
	scroll      *container.Scroll
	typing      *widget.Label
	typingTimer *time.Timer
	typingSent  time.Time
	problem     *widget.Label
	entry       *widget.Entry
	send        *widget.Button
	attach      *widget.Button
	sending     int // files still being sent

	// Answering or changing a message, with the bar above the text box
	replyTo    *puush.ChatMessage
	editing    *puush.ChatMessage
	composeBar fyne.CanvasObject
	compose    *widget.Label
	// "Seen" under the last message, when it's the user's and was read
	seen     fyne.CanvasObject
	lastMine bool
	names    map[string]string // display names, for "Replying to Mika"
	// Where the thread was scrolled to, kept while it's loaded again
	keepOffset *fyne.Position

	thumbs map[string]fyne.Resource // previews of files in chats

	avatars   map[string]fyne.Resource
	requested map[string]bool

	visible     bool
	reloadTimer *time.Timer
	stopLive    func()
}

func (ui *UI) buildMessagesTab() *messagesView {
	v := &messagesView{
		ui:        ui,
		avatars:   map[string]fyne.Resource{},
		requested: map[string]bool{},
		thumbs:    map[string]fyne.Resource{},
		names:     map[string]string{},
	}

	// Left: the chats, and a box to start one
	v.newChat = widget.NewEntry()
	v.newChat.SetPlaceHolder(i18n.T("Username"))
	start := func() {
		name := strings.TrimPrefix(strings.TrimSpace(v.newChat.Text), "@")
		if name != "" {
			v.newChat.SetText("")
			v.list.UnselectAll()
			v.openChat(name)
		}
	}
	v.newChat.OnSubmitted = func(string) { start() }
	startButton := widget.NewButtonWithIcon("", theme.ContentAddIcon(), start)

	v.listStatus = widget.NewLabel("")
	v.listStatus.Wrapping = fyne.TextWrapWord
	v.listStatus.Importance = widget.LowImportance

	v.list = widget.NewList(
		func() int { return len(v.chats) },
		v.createChatRow,
		v.updateChatRow,
	)
	v.list.OnSelected = func(id widget.ListItemID) {
		if id < len(v.chats) {
			v.openChat(v.chats[id].With)
		}
	}
	left := container.NewBorder(
		container.NewBorder(nil, nil, nil, startButton, v.newChat),
		v.listStatus, nil, nil, v.list,
	)

	// Right: the open chat
	v.title = widget.NewLabel("")
	v.profile = widget.NewHyperlink(i18n.T("Profile"), nil)
	v.presence = canvas.NewText("", quietTextColor)
	v.presence.TextSize = 11
	v.thread = container.NewVBox()
	v.scroll = container.NewVScroll(container.NewPadded(v.thread))
	v.typing = widget.NewLabel("")
	v.typing.Importance = widget.LowImportance
	v.typing.Hide()
	v.problem = widget.NewLabel("")
	v.problem.Wrapping = fyne.TextWrapWord
	v.problem.Importance = widget.DangerImportance
	v.problem.Hide()
	v.entry = widget.NewEntry()
	v.entry.SetPlaceHolder(i18n.T("Write a message"))
	v.entry.OnSubmitted = func(string) { v.sendMessage() }
	v.entry.OnChanged = func(string) { v.userTyping() }
	v.send = widget.NewButtonWithIcon(i18n.T("Send"), theme.MailSendIcon(), v.sendMessage)
	v.attach = widget.NewButtonWithIcon("", theme.MailAttachmentIcon(), v.pickFiles)

	v.compose = widget.NewLabel("")
	v.compose.Truncation = fyne.TextTruncateEllipsis
	v.compose.Importance = widget.LowImportance
	cancelCompose := widget.NewButtonWithIcon("", theme.CancelIcon(), v.stopComposing)
	cancelCompose.Importance = widget.LowImportance
	composeLine := canvas.NewRectangle(theme.Color(theme.ColorNamePrimary))
	composeLine.SetMinSize(fyne.NewSize(3, 0))
	v.composeBar = container.NewBorder(nil, nil, composeLine, cancelCompose, v.compose)
	v.composeBar.Hide()

	// The name, and on the same line whether they're online
	titles := container.NewHBox(v.title, container.NewCenter(v.presence))
	header := container.NewBorder(nil, widget.NewSeparator(), nil, v.profile, titles)
	footer := container.NewVBox(v.typing, v.problem, v.composeBar, container.NewBorder(nil, nil, v.attach, v.send, v.entry))
	v.chatPane = container.NewBorder(header, footer, nil, nil, v.scroll)
	v.chatPane.Hide()

	hint := widget.NewLabel(i18n.T("Pick a chat, or start one with someone's username."))
	hint.Alignment = fyne.TextAlignCenter
	hint.Wrapping = fyne.TextWrapWord
	hint.Importance = widget.LowImportance
	v.placeholder = container.NewCenter(container.NewGridWrap(fyne.NewSize(300, 80), hint))

	split := container.NewHSplit(left, container.NewStack(v.placeholder, v.chatPane))
	split.Offset = 0.32
	v.content = split

	v.stopLive = ui.tray.OnLive(v.onLive)
	return v
}

// show runs when the tab is shown.
func (v *messagesView) show() {
	if v.visible {
		return
	}
	v.visible = true
	v.loadChats()
	if v.with != "" {
		v.ui.tray.SetOpenChat(v.with)
		v.fetchNew()
	}
}

// hide runs when another tab is shown or the window closes.
func (v *messagesView) hide() {
	v.visible = false
	v.ui.tray.SetOpenChat("")
}

func (v *messagesView) close() {
	v.hide()
	if v.stopLive != nil {
		v.stopLive()
	}
}

// ---- The list of chats ----

type chatRow struct {
	widget.BaseWidget

	avatar  *canvas.Image
	online  *fyne.Container
	name    *widget.Label
	preview *widget.Label
	unread  *widget.Label
}

func (v *messagesView) createChatRow() fyne.CanvasObject {
	row := &chatRow{
		avatar:  canvas.NewImageFromResource(theme.AccountIcon()),
		name:    widget.NewLabel(""),
		preview: widget.NewLabel(""),
		unread:  widget.NewLabel(""),
	}
	row.avatar.FillMode = canvas.ImageFillContain
	row.avatar.SetMinSize(fyne.NewSquareSize(34))
	dot := canvas.NewCircle(onlineColor)
	dot.StrokeColor = color.White
	dot.StrokeWidth = 2
	row.online = container.NewGridWrap(fyne.NewSquareSize(11), dot)
	row.name.Truncation = fyne.TextTruncateEllipsis
	row.preview.Truncation = fyne.TextTruncateEllipsis
	row.preview.Importance = widget.LowImportance
	row.unread.Importance = widget.HighImportance
	row.ExtendBaseWidget(row)
	return row
}

func (row *chatRow) CreateRenderer() fyne.WidgetRenderer {
	text := container.New(layout.NewCustomPaddedVBoxLayout(-6), row.name, row.preview)
	// The green dot of people online sits on the avatar's corner
	corner := container.NewVBox(layout.NewSpacer(), container.NewHBox(layout.NewSpacer(), row.online))
	avatar := container.NewCenter(container.NewStack(row.avatar, corner))
	return widget.NewSimpleRenderer(container.NewBorder(nil, nil, avatar, row.unread, text))
}

func (v *messagesView) updateChatRow(id widget.ListItemID, object fyne.CanvasObject) {
	row, ok := object.(*chatRow)
	if !ok || id >= len(v.chats) {
		return
	}
	chat := v.chats[id]
	v.names[strings.ToLower(chat.With)] = chat.Name
	row.name.SetText(chat.Name)
	if chat.Online {
		row.online.Show()
	} else {
		row.online.Hide()
	}
	preview := strings.Join(strings.Fields(chat.Text), " ")
	if chat.FromMe {
		preview = i18n.T("You: %s", preview)
	}
	row.preview.SetText(preview)
	if chat.Unread > 0 {
		row.unread.SetText(unreadText(chat.Unread))
		row.unread.Show()
	} else {
		row.unread.Hide()
	}
	row.avatar.Resource = v.avatar(chat.Avatar)
	row.avatar.Refresh()
}

// avatar returns a profile picture, downloading it the first time.
func (v *messagesView) avatar(link string) fyne.Resource {
	if resource, ok := v.avatars[link]; ok {
		return resource
	}
	if link != "" && !v.requested[link] {
		v.requested[link] = true
		go func() {
			data, err := v.ui.api.Picture(link)
			if err != nil {
				return
			}
			resource := fyne.NewStaticResource("avatar.png", data)
			fyne.Do(func() {
				v.avatars[link] = resource
				v.list.Refresh()
			})
		}()
	}
	return theme.AccountIcon()
}

func (v *messagesView) loadChats() {
	if !v.ui.api.Account.Credentials.HasApiKey() {
		v.chats = nil
		v.list.Refresh()
		v.listStatus.SetText(i18n.T("Log in to chat with people."))
		v.listStatus.Show()
		return
	}
	go func() {
		chats, unread, err := v.ui.api.Chats()
		fyne.Do(func() {
			status := ""
			switch {
			case errors.Is(err, puush.ErrNotSupported):
				status = i18n.T("This server doesn't have chats yet.")
			case err != nil:
				status = puush.FormatError(err)
			case len(chats) == 0:
				status = i18n.T("No chats yet. Start one with someone's username.")
			}
			v.listStatus.SetText(status)
			if status == "" {
				v.listStatus.Hide()
			} else {
				v.listStatus.Show()
			}
			if err != nil {
				return
			}
			v.chats = chats
			v.list.Refresh()
			v.ui.tray.ChatRead(unread)
			// Keep the open chat highlighted, wherever it moved to
			for i, chat := range chats {
				if strings.EqualFold(chat.With, v.with) {
					v.list.Select(i)
				}
			}
		})
	}()
}

// reloadChatsSoon reloads the list once, after a burst of messages.
func (v *messagesView) reloadChatsSoon() {
	if v.reloadTimer != nil {
		v.reloadTimer.Stop()
	}
	v.reloadTimer = time.AfterFunc(400*time.Millisecond, func() {
		fyne.Do(func() {
			if v.visible {
				v.loadChats()
			}
		})
	})
}

// ---- The open chat ----

func (v *messagesView) openChat(name string) {
	if strings.EqualFold(name, v.with) {
		return
	}
	v.generation++
	v.with = name
	v.lastId = 0
	v.thread.RemoveAll()
	v.seen = nil
	v.lastMine = false
	v.stopComposing()
	v.showPresence(nil)
	v.title.SetText(name)
	v.setProfileLink(name)
	v.typing.Hide()
	v.problem.Hide()
	v.entry.SetText("")
	v.entry.Enable()
	v.send.Enable()
	v.attach.Enable()
	v.placeholder.Hide()
	v.chatPane.Show()
	if v.visible {
		v.ui.tray.SetOpenChat(name)
	}
	v.fetchNew()
}

func (v *messagesView) setProfileLink(name string) {
	link, err := url.Parse(v.ui.api.FormatURL("/u/" + url.PathEscape(name)))
	if err == nil {
		v.profile.SetURL(link)
	}
}

// fetchNew loads the messages after the last one shown (all of them for a
// chat that was just opened), which also marks them read.
func (v *messagesView) fetchNew() {
	if v.with == "" {
		return
	}
	if v.fetching {
		v.fetchAgain = true
		return
	}
	v.fetching = true
	with, after, generation := v.with, v.lastId, v.generation
	go func() {
		thread, err := v.ui.api.ChatWith(with, after)
		fyne.Do(func() {
			v.fetching = false
			if generation != v.generation {
				// Another chat was opened meanwhile
				v.fetchAgain = false
				v.fetchNew()
				return
			}
			if err != nil {
				v.showProblem(err)
				if after == 0 {
					v.entry.Disable()
					v.send.Disable()
					v.attach.Disable()
				}
				return
			}
			v.showThread(thread, after == 0)
			v.keepOffset = nil
			if v.fetchAgain {
				v.fetchAgain = false
				v.fetchNew()
			}
		})
	}()
}

func (v *messagesView) showThread(thread *puush.ChatThread, first bool) {
	if first && v.lastId == 0 {
		// The server knows how the name is written (Lilian, not lilian)
		v.with = thread.With
		v.names[strings.ToLower(thread.With)] = thread.Name
		v.title.SetText(thread.Name)
		v.showPresence(thread.Presence)
		v.seen = nil
		v.lastMine = false
		v.setProfileLink(thread.With)
		if v.visible {
			v.ui.tray.SetOpenChat(thread.With)
		}
		v.thread.RemoveAll()
		v.hint = len(thread.Messages) == 0
		if v.hint {
			v.thread.Add(quietLabel(i18n.T("No messages yet. Say hi!")))
		}
	}
	added := false
	for _, message := range thread.Messages {
		if message.Id <= v.lastId {
			continue
		}
		if v.hint {
			v.hint = false
			v.thread.RemoveAll()
		}
		if v.seen != nil {
			v.thread.Remove(v.seen)
			v.seen = nil
		}
		v.lastId = message.Id
		v.thread.Add(v.messageRow(message))
		v.lastMine = message.Mine
		if message.Mine && message.Seen {
			v.showSeen()
		}
		added = true
	}
	if thread.CantSend != "" {
		v.problem.SetText(thread.CantSend)
		v.problem.Show()
		v.entry.Disable()
		v.send.Disable()
		v.attach.Disable()
	} else {
		v.problem.Hide()
		v.entry.Enable()
		v.send.Enable()
		v.attach.Enable()
	}
	if added || first {
		v.thread.Refresh()
		if v.keepOffset != nil {
			v.scroll.Offset = *v.keepOffset
			v.scroll.Refresh()
		} else {
			v.scroll.ScrollToBottom()
		}
		if !first {
			v.typing.Hide()
		}
		v.reloadChatsSoon()
	}
}

func (v *messagesView) showProblem(err error) {
	var serverError *puush.ServerError
	switch {
	case errors.As(err, &serverError):
		v.problem.SetText(serverError.Message)
	case errors.Is(err, puush.ErrNotSupported):
		v.problem.SetText(i18n.T("This server doesn't have chats yet."))
	default:
		v.problem.SetText(puush.FormatError(err))
	}
	v.problem.Show()
}

func (v *messagesView) sendMessage() {
	text := strings.TrimSpace(v.entry.Text)
	if text == "" || v.with == "" {
		return
	}
	editing, replyTo := v.editing, v.replyTo
	v.stopComposing()
	v.entry.SetText("")
	v.send.Disable()
	with, generation := v.with, v.generation
	go func() {
		var err error
		switch {
		case editing != nil:
			_, err = v.ui.api.EditMessage(with, editing.Id, text)
		case replyTo != nil:
			_, err = v.ui.api.SendMessage(with, text, replyTo.Id)
		default:
			_, err = v.ui.api.SendMessage(with, text, 0)
		}
		fyne.Do(func() {
			if generation != v.generation {
				return
			}
			v.send.Enable()
			if err != nil {
				v.entry.SetText(text)
				if editing != nil {
					v.startEditing(editing)
				} else if replyTo != nil {
					v.startReply(replyTo)
				}
				v.showProblem(err)
				return
			}
			v.problem.Hide()
			if editing != nil {
				v.reload()
			} else {
				v.fetchNew()
			}
		})
	}()
}

// pickFiles lets the user pick files to send in the open chat.
func (v *messagesView) pickFiles() {
	if v.with == "" {
		return
	}
	go func() {
		path, err := dialog.File().Title(i18n.T("Send a file")).Load()
		if err != nil || path == "" {
			return
		}
		fyne.Do(func() { v.sendFiles([]string{path}) })
	}()
}

// dropped sends files dropped on the window into the open chat. It returns
// false when no chat is open, so they're uploaded as usual.
func (v *messagesView) dropped(uris []fyne.URI) bool {
	if !v.visible || v.with == "" || v.attach.Disabled() {
		return false
	}
	var paths []string
	for _, uri := range uris {
		if uri.Scheme() == "file" {
			paths = append(paths, uri.Path())
		}
	}
	v.sendFiles(paths)
	return true
}

// sendFiles uploads files into the account's Chat pool and sends each one
// as a message, one after another.
func (v *messagesView) sendFiles(paths []string) {
	if len(paths) == 0 {
		return
	}
	with, generation := v.with, v.generation
	v.sending += len(paths)
	v.showSending()
	go func() {
		// The server says which pool chat files go into
		poolId := 0
		if profile, err := v.ui.api.Profile(); err == nil {
			poolId = profile.ChatPool
		}
		for _, path := range paths {
			err := v.sendFile(with, path, poolId)
			fyne.Do(func() {
				v.sending--
				if generation != v.generation {
					return
				}
				if err != nil {
					v.showProblem(err)
				} else {
					v.problem.Hide()
					v.fetchNew()
				}
				v.showSending()
			})
		}
	}()
}

func (v *messagesView) sendFile(with, path string, poolId int) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if info.IsDir() {
		return errors.New(i18n.T("Folders can't be sent, only files."))
	}
	link, err := v.ui.api.UploadWithOptions(context.Background(), file, filepath.Base(path), puush.UploadOptions{PoolId: poolId, Size: info.Size()})
	if err != nil {
		return err
	}
	_, err = v.ui.api.SendFile(with, link, "")
	return err
}

func (v *messagesView) showSending() {
	if v.sending > 0 {
		v.typing.SetText(i18n.T("Sending %d file(s)…", v.sending))
		v.typing.Show()
	} else {
		v.typing.Hide()
	}
}

// userTyping lets the other person know, at most every few seconds.
func (v *messagesView) userTyping() {
	if v.with == "" || v.entry.Text == "" || time.Since(v.typingSent) < 3*time.Second {
		return
	}
	v.typingSent = time.Now()
	go v.ui.tray.Typing(v.with)
}

// onLive gets the live events, from another goroutine.
func (v *messagesView) onLive(event *puush.LiveEvent) {
	switch event.Type {
	case "message":
		var message puush.LiveMessage
		if json.Unmarshal(event.Data, &message) != nil {
			return
		}
		fyne.Do(func() {
			if !v.visible {
				return
			}
			if strings.EqualFold(message.With, v.with) {
				v.fetchNew()
			}
			v.reloadChatsSoon()
		})

	case "changed":
		var changed struct {
			With string `json:"with"`
		}
		if json.Unmarshal(event.Data, &changed) != nil {
			return
		}
		fyne.Do(func() {
			if strings.EqualFold(changed.With, v.with) {
				v.reload()
			}
			v.reloadChatsSoon()
		})

	case "seen":
		var seen struct {
			With string `json:"with"`
		}
		if json.Unmarshal(event.Data, &seen) != nil {
			return
		}
		fyne.Do(func() {
			if strings.EqualFold(seen.With, v.with) && v.lastMine && v.seen == nil {
				v.showSeen()
				v.thread.Refresh()
			}
		})

	case "presence":
		var presence struct {
			With string `json:"with"`
			puush.Presence
		}
		if json.Unmarshal(event.Data, &presence) != nil {
			return
		}
		fyne.Do(func() {
			if strings.EqualFold(presence.With, v.with) {
				v.showPresence(&presence.Presence)
			}
			for _, chat := range v.chats {
				if strings.EqualFold(chat.With, presence.With) && chat.Online != (presence.Online && !presence.Hidden) {
					chat.Online = presence.Online && !presence.Hidden
					v.list.Refresh()
				}
			}
		})

	case "typing":
		var typing struct {
			With string `json:"with"`
		}
		if json.Unmarshal(event.Data, &typing) != nil {
			return
		}
		fyne.Do(func() {
			if !strings.EqualFold(typing.With, v.with) {
				return
			}
			v.typing.SetText(i18n.T("%s is typing…", v.title.Text))
			v.typing.Show()
			if v.typingTimer != nil {
				v.typingTimer.Stop()
			}
			v.typingTimer = time.AfterFunc(5*time.Second, func() { fyne.Do(v.typing.Hide) })
		})
	}
}

// messageBubble is one message: on the right for the user's own, on the
// left for the other person's.
func (v *messagesView) messageBubble(message *puush.ChatMessage) fyne.CanvasObject {
	parts := []fyne.CanvasObject{}
	switch {
	case message.Removed:
		gone := widget.NewLabel(i18n.T("Message deleted"))
		gone.TextStyle = fyne.TextStyle{Italic: true}
		gone.Importance = widget.LowImportance
		parts = append(parts, gone)
	default:
		if message.Reply != nil {
			parts = append(parts, v.replyQuote(message.Reply))
		}
		if message.File != nil {
			parts = append(parts, v.fileCard(message.File))
		}
		if message.Text != "" || message.File == nil {
			parts = append(parts, widget.NewLabel(wrapText(message.Text, bubbleTextWidth)))
		}
	}
	stampText := message.Time
	if message.Edited && !message.Removed {
		stampText = i18n.T("%s · edited", message.Time)
	}
	stamp := canvas.NewText(stampText, quietTextColor)
	stamp.TextSize = 10

	background := canvas.NewRectangle(theirBubbleColor)
	if message.Mine {
		background.FillColor = myBubbleColor
		stamp.Alignment = fyne.TextAlignTrailing
	}
	background.CornerRadius = 8
	background.StrokeColor = color.NRGBA{R: 220, G: 224, B: 228, A: 255}
	background.StrokeWidth = 1

	parts = append(parts, container.New(layout.NewCustomPaddedLayout(0, 6, 8, 8), stamp))
	inner := container.New(layout.NewCustomPaddedVBoxLayout(-4), parts...)
	bubble := container.NewStack(background, inner)
	if message.Mine {
		return container.NewHBox(layout.NewSpacer(), bubble)
	}
	return container.NewHBox(bubble, layout.NewSpacer())
}

// fileCard is a file sent in a chat: a preview for pictures and videos,
// and its name (which opens it) and size.
func (v *messagesView) fileCard(file *puush.ChatFile) fyne.CanvasObject {
	link, _ := url.Parse(file.Url)
	name := widget.NewHyperlink(file.Name, link)
	name.Truncation = fyne.TextTruncateEllipsis
	size := canvas.NewText(file.Size, quietTextColor)
	size.TextSize = 11

	preview := canvas.NewImageFromResource(theme.FileIcon())
	preview.FillMode = canvas.ImageFillContain
	if file.Kind == "image" || file.Kind == "video" {
		preview.SetMinSize(fyne.NewSize(bubbleTextWidth*0.7, 150))
	} else {
		preview.SetMinSize(fyne.NewSquareSize(40))
	}
	if resource, ok := v.thumbs[file.Thumb]; ok {
		preview.Resource = resource
	} else if file.Thumb != "" {
		go func() {
			data, err := v.ui.api.Picture(file.Thumb)
			if err != nil {
				return
			}
			// File icons are SVG pictures
			name := "thumb"
			if bytes.HasPrefix(bytes.TrimSpace(data), []byte("<svg")) || bytes.HasPrefix(bytes.TrimSpace(data), []byte("<?xml")) {
				name = "thumb.svg"
			}
			resource := fyne.NewStaticResource(name, data)
			fyne.Do(func() {
				v.thumbs[file.Thumb] = resource
				preview.Resource = resource
				preview.Refresh()
			})
		}()
	}

	details := container.New(layout.NewCustomPaddedVBoxLayout(-2), name, container.New(layout.NewCustomPaddedLayout(0, 0, 8, 8), size))
	if file.Kind == "image" || file.Kind == "video" {
		return container.NewVBox(container.NewPadded(preview), details)
	}
	// A bubble is only as wide as what's in it, and a name that's cut short
	// asks for no room at all, so the card keeps a width of its own
	width := min(fyne.MeasureText(file.Name, theme.TextSize(), fyne.TextStyle{}).Width+90, bubbleTextWidth)
	sizer := canvas.NewRectangle(color.Transparent)
	sizer.SetMinSize(fyne.NewSize(max(width, 180), 0))
	return container.NewStack(sizer, container.NewBorder(nil, nil, container.NewPadded(preview), nil, details))
}

func quietLabel(text string) fyne.CanvasObject {
	label := widget.NewLabel(text)
	label.Importance = widget.LowImportance
	label.Alignment = fyne.TextAlignCenter
	return label
}

// wrapText breaks text into lines that fit in width, between words where it
// can.
func wrapText(text string, width float32) string {
	size := theme.TextSize()
	fits := func(line string) bool {
		return fyne.MeasureText(line, size, fyne.TextStyle{}).Width <= width
	}
	var lines []string
	for _, paragraph := range strings.Split(text, "\n") {
		line := ""
		for _, word := range strings.Fields(paragraph) {
			candidate := word
			if line != "" {
				candidate = line + " " + word
			}
			if fits(candidate) {
				line = candidate
				continue
			}
			if line != "" {
				lines = append(lines, line)
				line = ""
			}
			// A word longer than a line (like a link) is split anywhere
			for !fits(word) {
				runes := []rune(word)
				cut := len(runes) - 1
				for cut > 1 && !fits(string(runes[:cut])) {
					cut--
				}
				lines = append(lines, string(runes[:cut]))
				word = string(runes[cut:])
			}
			line = word
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func unreadText(n int) string {
	if n > 99 {
		return "99+"
	}
	return strconv.Itoa(n)
}

// loggedOut forgets the chats of the account that logged out.
func (v *messagesView) loggedOut() {
	v.generation++
	v.with = ""
	v.lastId = 0
	v.thread.RemoveAll()
	v.chatPane.Hide()
	v.placeholder.Show()
	v.list.UnselectAll()
	v.loadChats()
}

// reload loads the open chat again, e.g. after a message was changed,
// staying where it was scrolled to.
func (v *messagesView) reload() {
	if v.with == "" {
		return
	}
	offset := v.scroll.Offset
	v.keepOffset = &offset
	v.lastId = 0
	v.fetchNew()
}

func (v *messagesView) showSeen() {
	label := canvas.NewText(i18n.T("Seen"), quietTextColor)
	label.TextSize = 10
	label.Alignment = fyne.TextAlignTrailing
	v.seen = container.New(layout.NewCustomPaddedLayout(-6, 0, 0, 10), label)
	v.thread.Add(v.seen)
}

// showPresence shows whether the other person is online, and on what.
func (v *messagesView) showPresence(presence *puush.Presence) {
	v.presence.Text = presenceText(presence, time.Now())
	if presence != nil && presence.Online && !presence.Hidden {
		v.presence.Color = onlineColor
	} else {
		v.presence.Color = quietTextColor
	}
	v.presence.Refresh()
}

func presenceText(presence *puush.Presence, now time.Time) string {
	if presence == nil || presence.Hidden {
		return ""
	}
	if presence.Online {
		var devices []string
		for _, device := range presence.Devices {
			switch device {
			case "phone":
				devices = append(devices, i18n.T("phone"))
			case "app":
				devices = append(devices, i18n.T("the app"))
			case "web":
				devices = append(devices, i18n.T("the website"))
			}
		}
		if len(devices) == 0 {
			return i18n.T("Online")
		}
		return i18n.T("Online on %s", strings.Join(devices, ", "))
	}
	if presence.Seen <= 0 {
		return ""
	}
	seen := time.Unix(presence.Seen, 0).Local()
	switch {
	case now.Sub(seen) < time.Minute:
		return i18n.T("Last online just now")
	case sameDay(seen, now):
		return i18n.T("Last online today at %s", seen.Format("15:04"))
	case sameDay(seen, now.AddDate(0, 0, -1)):
		return i18n.T("Last online yesterday at %s", seen.Format("15:04"))
	}
	return i18n.T("Last online %s", i18n.Date(seen))
}

func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

// ---- Answering and changing messages ----

// messageRow is a message with its menu: right-click it, or the "..."
// button that shows while the mouse is over it.
type messageRow struct {
	widget.BaseWidget

	content fyne.CanvasObject
	more    *widget.Button
	menu    *fyne.Menu
}

var _ desktop.Hoverable = (*messageRow)(nil)

func (v *messagesView) messageRow(message *puush.ChatMessage) fyne.CanvasObject {
	bubble := v.messageBubble(message)
	menu := v.messageMenu(message)
	if menu == nil {
		if message.Mine {
			return container.NewHBox(layout.NewSpacer(), bubble)
		}
		return container.NewHBox(bubble, layout.NewSpacer())
	}
	row := &messageRow{menu: menu}
	row.more = widget.NewButtonWithIcon("", theme.MoreHorizontalIcon(), func() {
		row.showMenu(fyne.CurrentApp().Driver().AbsolutePositionForObject(row.more).AddXY(0, row.more.Size().Height))
	})
	row.more.Importance = widget.LowImportance
	row.more.Hide()
	if message.Mine {
		row.content = container.NewHBox(layout.NewSpacer(), container.NewCenter(row.more), bubble)
	} else {
		row.content = container.NewHBox(bubble, container.NewCenter(row.more), layout.NewSpacer())
	}
	row.ExtendBaseWidget(row)
	return row
}

func (row *messageRow) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(row.content)
}

func (row *messageRow) TappedSecondary(event *fyne.PointEvent) {
	row.showMenu(event.AbsolutePosition)
}

func (row *messageRow) showMenu(at fyne.Position) {
	if canvas := fyne.CurrentApp().Driver().CanvasForObject(row); canvas != nil {
		widget.ShowPopUpMenuAtPosition(row.menu, canvas, at)
	}
}

func (row *messageRow) MouseIn(*desktop.MouseEvent)    { row.more.Show() }
func (row *messageRow) MouseMoved(*desktop.MouseEvent) {}
func (row *messageRow) MouseOut()                      { row.more.Hide() }

// messageMenu is what can be done with a message; nil for deleted ones.
func (v *messagesView) messageMenu(message *puush.ChatMessage) *fyne.Menu {
	if message.Removed {
		return nil
	}
	clipboard := fyne.CurrentApp().Clipboard()
	items := []*fyne.MenuItem{
		fyne.NewMenuItem(i18n.T("Reply"), func() { v.startReply(message) }),
	}
	if message.Text != "" {
		items = append(items, fyne.NewMenuItem(i18n.T("Copy text"), func() { clipboard.SetContent(message.Text) }))
	}
	if message.File != nil {
		items = append(items, fyne.NewMenuItem(i18n.T("Copy link"), func() { clipboard.SetContent(message.File.Url) }))
	}
	if message.Mine {
		items = append(items,
			fyne.NewMenuItemSeparator(),
			fyne.NewMenuItem(i18n.T("Edit"), func() { v.startEditing(message) }),
			fyne.NewMenuItem(i18n.T("Delete"), func() { v.confirmRemove(message) }),
		)
	}
	return fyne.NewMenu("", items...)
}

func (v *messagesView) startReply(message *puush.ChatMessage) {
	v.editing = nil
	v.replyTo = message
	name := i18n.T("yourself")
	if !message.Mine {
		name = v.displayName()
	}
	v.compose.SetText(i18n.T("Replying to %s: %s", name, oneLine(messageSummary(message))))
	v.composeBar.Show()
	v.focusEntry()
}

func (v *messagesView) startEditing(message *puush.ChatMessage) {
	v.replyTo = nil
	v.editing = message
	v.compose.SetText(i18n.T("Editing your message"))
	v.composeBar.Show()
	v.entry.SetText(message.Text)
	v.focusEntry()
}

func (v *messagesView) stopComposing() {
	if v.editing != nil {
		v.entry.SetText("")
	}
	v.replyTo = nil
	v.editing = nil
	v.composeBar.Hide()
}

func (v *messagesView) focusEntry() {
	if canvas := fyne.CurrentApp().Driver().CanvasForObject(v.entry); canvas != nil {
		canvas.Focus(v.entry)
	}
}

func (v *messagesView) confirmRemove(message *puush.ChatMessage) {
	with, generation := v.with, v.generation
	// The native dialog waits for an answer, so not on the main thread
	go func() {
		confirmed := dialog.Message("%s", i18n.T("Delete this message? It's deleted for both of you.")).
			Title(i18n.T("Delete message")).
			YesNo()
		if !confirmed {
			return
		}
		err := v.ui.api.RemoveMessage(with, message.Id)
		fyne.Do(func() {
			if generation != v.generation {
				return
			}
			if err != nil {
				v.showProblem(err)
				return
			}
			v.reload()
		})
	}()
}

func (v *messagesView) displayName() string {
	if name := v.names[strings.ToLower(v.with)]; name != "" {
		return name
	}
	return v.with
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
	what := canvas.NewText(fitText(oneLine(text), bubbleTextWidth-24, 11), quietTextColor)
	what.TextSize = 11
	line := canvas.NewRectangle(theme.Color(theme.ColorNamePrimary))
	line.SetMinSize(fyne.NewSize(3, 0))
	lines := container.New(layout.NewCustomPaddedVBoxLayout(0), who, what)
	return container.New(layout.NewCustomPaddedLayout(6, 0, 8, 8),
		container.NewBorder(nil, nil, line, nil, container.New(layout.NewCustomPaddedLayout(0, 0, 6, 0), lines)))
}

// messageSummary is a message in a few words, for the reply bar.
func messageSummary(message *puush.ChatMessage) string {
	if message.Text != "" {
		return message.Text
	}
	if message.File != nil {
		return message.File.Name
	}
	return ""
}

func oneLine(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

// fitText shortens text with "…" until it fits in width.
func fitText(text string, width float32, size float32) string {
	fits := func(line string) bool {
		return fyne.MeasureText(line, size, fyne.TextStyle{}).Width <= width
	}
	if fits(text) {
		return text
	}
	runes := []rune(text)
	for len(runes) > 0 && !fits(string(runes)+"…") {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + "…"
}
