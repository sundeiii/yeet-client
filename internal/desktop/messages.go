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
	v.title.Truncation = fyne.TextTruncateEllipsis
	v.profile = widget.NewHyperlink(i18n.T("Profile"), nil)
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

	header := container.NewBorder(nil, widget.NewSeparator(), nil, v.profile, v.title)
	footer := container.NewVBox(v.typing, v.problem, container.NewBorder(nil, nil, v.attach, v.send, v.entry))
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
	row.name.Truncation = fyne.TextTruncateEllipsis
	row.preview.Truncation = fyne.TextTruncateEllipsis
	row.preview.Importance = widget.LowImportance
	row.unread.Importance = widget.HighImportance
	row.ExtendBaseWidget(row)
	return row
}

func (row *chatRow) CreateRenderer() fyne.WidgetRenderer {
	text := container.New(layout.NewCustomPaddedVBoxLayout(-12), row.name, row.preview)
	return widget.NewSimpleRenderer(container.NewBorder(nil, nil, container.NewCenter(row.avatar), row.unread, text))
}

func (v *messagesView) updateChatRow(id widget.ListItemID, object fyne.CanvasObject) {
	row, ok := object.(*chatRow)
	if !ok || id >= len(v.chats) {
		return
	}
	chat := v.chats[id]
	row.name.SetText(chat.Name)
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
		v.title.SetText(thread.Name)
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
		v.lastId = message.Id
		v.thread.Add(v.messageBubble(message))
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
		v.scroll.ScrollToBottom()
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
	v.entry.SetText("")
	v.send.Disable()
	with, generation := v.with, v.generation
	go func() {
		_, err := v.ui.api.SendMessage(with, text)
		fyne.Do(func() {
			if generation != v.generation {
				return
			}
			v.send.Enable()
			if err != nil {
				v.entry.SetText(text)
				v.showProblem(err)
				return
			}
			v.problem.Hide()
			v.fetchNew()
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
	if message.File != nil {
		parts = append(parts, v.fileCard(message.File))
	}
	if message.Text != "" || message.File == nil {
		text := widget.NewLabel(wrapText(message.Text, bubbleTextWidth))
		text.Selectable = true
		parts = append(parts, text)
	}
	stamp := canvas.NewText(message.Time, quietTextColor)
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
	inner := container.New(layout.NewCustomPaddedVBoxLayout(-6), parts...)
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

	details := container.New(layout.NewCustomPaddedVBoxLayout(-8), name, container.New(layout.NewCustomPaddedLayout(0, 0, 8, 8), size))
	if file.Kind == "image" || file.Kind == "video" {
		return container.NewVBox(container.NewPadded(preview), details)
	}
	return container.NewBorder(nil, nil, container.NewPadded(preview), nil, details)
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
