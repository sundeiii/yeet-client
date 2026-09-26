package tray

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/sundeiii/yeet-client/assets"
	"github.com/sundeiii/yeet-client/internal/i18n"
	"github.com/sundeiii/yeet-client/internal/notifications"
	"github.com/sundeiii/yeet-client/pkg/puush"
)

// The live connection to the server: chat messages and comments arrive as
// they happen, as notifications and in the app's Messages tab.

type liveState struct {
	mu        sync.Mutex
	cancel    context.CancelFunc
	key       string // the server and API key the connection logs in with
	conn      *puush.LiveConnection
	unread    int
	openChat  string // the chat the app window shows, if any
	focused   bool   // the app window is in front
	listeners map[int]func(*puush.LiveEvent)
	nextId    int
}

// StartLive opens the live connection for the logged in account, and keeps
// it open. It does nothing when it's already open for the same account.
func (m *TrayManager) StartLive() {
	if !m.api.Account.Credentials.HasApiKey() {
		m.StopLive()
		return
	}
	// Another account or another server means another connection
	key := m.api.BaseURL + " " + *m.api.Account.Credentials.Key

	m.live.mu.Lock()
	if m.live.cancel != nil && m.live.key == key {
		m.live.mu.Unlock()
		return
	}
	if m.live.cancel != nil {
		m.live.cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.live.cancel = cancel
	m.live.key = key
	m.live.mu.Unlock()

	go m.runLive(ctx)
}

// StopLive closes the live connection, e.g. after logging out.
func (m *TrayManager) StopLive() {
	m.live.mu.Lock()
	if m.live.cancel != nil {
		m.live.cancel()
		m.live.cancel = nil
	}
	m.live.key = ""
	changed := m.live.unread != 0
	m.live.unread = 0
	m.live.mu.Unlock()
	if changed {
		m.unreadChanged()
	}
}

func (m *TrayManager) runLive(ctx context.Context) {
	retry := 2 * time.Second
	for ctx.Err() == nil && m.api.Account.Credentials.HasApiKey() {
		conn, err := m.api.ConnectLive(ctx)
		if err == nil {
			retry = 2 * time.Second
			m.setLiveConnection(conn)
			err = m.readLive(ctx, conn)
			m.setLiveConnection(nil)
			conn.Close()
		}

		wait := retry
		switch {
		case ctx.Err() != nil:
			return
		case errors.Is(err, puush.ErrNotSupported):
			// An older server without chats; it might be updated later
			wait = time.Hour
		case errors.Is(err, puush.PuushErrorInvalidCredentials), puush.LoggedOut(err):
			wait = 5 * time.Minute
		default:
			log.Printf("Live connection closed: %v", err)
			retry = min(retry*2, 2*time.Minute)
		}
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return
		}
	}
}

func (m *TrayManager) setLiveConnection(conn *puush.LiveConnection) {
	m.live.mu.Lock()
	m.live.conn = conn
	m.live.mu.Unlock()
}

func (m *TrayManager) readLive(ctx context.Context, conn *puush.LiveConnection) error {
	// A dead connection (after sleep, or a new network) would otherwise
	// wait for messages forever; dropping it makes runLive connect again
	pingCtx, stopPing := context.WithCancel(ctx)
	defer stopPing()
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-pingCtx.Done():
				return
			case <-ticker.C:
			}
			timeout, done := context.WithTimeout(pingCtx, 15*time.Second)
			err := conn.Ping(timeout)
			done()
			if err != nil && pingCtx.Err() == nil {
				log.Printf("Live connection stopped answering: %v", err)
				conn.CloseNow()
				return
			}
		}
	}()
	for {
		event, err := conn.Next(ctx)
		if err != nil {
			return err
		}
		m.handleLive(event)
	}
}

func (m *TrayManager) handleLive(event *puush.LiveEvent) {
	switch event.Type {
	case "unread":
		var data struct {
			Count int `json:"count"`
		}
		if json.Unmarshal(event.Data, &data) == nil {
			m.setUnread(data.Count)
		}

	case "message":
		var message puush.LiveMessage
		if json.Unmarshal(event.Data, &message) != nil {
			return
		}
		if message.FromMe {
			break
		}
		m.live.mu.Lock()
		// Only when it's really on screen; a minimized window doesn't count
		open := m.live.focused && strings.EqualFold(m.live.openChat, message.With)
		m.live.mu.Unlock()
		if !open {
			m.setUnread(message.Unread)
			go m.notifyMessage(&message)
		}

	case "comment":
		var comment puush.LiveComment
		if json.Unmarshal(event.Data, &comment) == nil {
			go m.notifyComment(&comment)
		}
	}

	m.live.mu.Lock()
	listeners := make([]func(*puush.LiveEvent), 0, len(m.live.listeners))
	for _, listener := range m.live.listeners {
		listeners = append(listeners, listener)
	}
	m.live.mu.Unlock()
	for _, listener := range listeners {
		listener(event)
	}
}

// notifyMessage shows a chat message; clicking it opens the chat.
func (m *TrayManager) notifyMessage(message *puush.LiveMessage) {
	notifications.NewNotification(message.Name, "", message.Text).
		WithIconData(m.avatar(message.Avatar)).
		WithAction(m.api.FormatURL("/account/messages/" + message.With)).
		Push()
}

// notifyComment shows a comment on one of the account's files; clicking it
// opens the file.
func (m *TrayManager) notifyComment(comment *puush.LiveComment) {
	notifications.NewNotification(i18n.T("%s commented on %s", comment.Name, comment.File), "", comment.Text).
		WithIconData(m.avatar(comment.Avatar)).
		WithAction(comment.Link).
		Push()
}

// avatar downloads a profile picture for a notification, or gives the
// puush icon.
func (m *TrayManager) avatar(link string) []byte {
	if link == "" {
		return assets.PuushIconData
	}
	data, err := m.api.Picture(link + "&png=1")
	if err != nil || len(data) == 0 {
		return assets.PuushIconData
	}
	return data
}

func (m *TrayManager) setUnread(count int) {
	m.live.mu.Lock()
	changed := m.live.unread != count
	m.live.unread = count
	m.live.mu.Unlock()
	if changed {
		m.unreadChanged()
	}
}

func (m *TrayManager) unreadChanged() {
	m.stateChanged()
	m.OnTrayIdle()
}

// Unread is the number of unread chat messages.
func (m *TrayManager) Unread() int {
	m.live.mu.Lock()
	defer m.live.mu.Unlock()
	return m.live.unread
}

// SetOpenChat tells which chat the app window shows ("" for none), so its
// messages don't pop up as notifications.
func (m *TrayManager) SetOpenChat(name string) {
	m.live.mu.Lock()
	m.live.openChat = name
	m.live.mu.Unlock()
}

// SetAppFocused tells whether the app window is in front.
func (m *TrayManager) SetAppFocused(focused bool) {
	m.live.mu.Lock()
	m.live.focused = focused
	m.live.mu.Unlock()
}

// ChatRead updates the unread count after the app showed a chat.
func (m *TrayManager) ChatRead(unread int) {
	m.setUnread(unread)
}

// OnLive calls listener (from another goroutine) with every live event,
// until the returned function is called.
func (m *TrayManager) OnLive(listener func(*puush.LiveEvent)) func() {
	m.live.mu.Lock()
	defer m.live.mu.Unlock()
	if m.live.listeners == nil {
		m.live.listeners = map[int]func(*puush.LiveEvent){}
	}
	m.live.nextId++
	id := m.live.nextId
	m.live.listeners[id] = listener
	return func() {
		m.live.mu.Lock()
		delete(m.live.listeners, id)
		m.live.mu.Unlock()
	}
}

// Typing tells the other person in a chat that the user is typing.
func (m *TrayManager) Typing(to string) {
	m.live.mu.Lock()
	conn := m.live.conn
	m.live.mu.Unlock()
	if conn == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn.Typing(ctx, to)
}

// SetMessagesCallback sets the function that opens the app on its
// Messages tab.
func (m *TrayManager) SetMessagesCallback(callback func()) {
	m.messagesCallback = callback
}
