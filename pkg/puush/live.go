package puush

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/coder/websocket"
)

// LiveEvent is something that just happened, sent over the live
// connection: "message" (a chat message), "unread" (the unread count),
// "comment" (someone commented on the account's file) or "typing".
type LiveEvent struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// LiveMessage is the data of a "message" event.
type LiveMessage struct {
	With    string       `json:"with"`
	Name    string       `json:"name"`
	Avatar  string       `json:"avatar"`
	Text    string       `json:"text"`
	Unread  int          `json:"unread"`
	FromMe  bool         `json:"fromMe"`
	Message *ChatMessage `json:"message"`
}

// LiveComment is the data of a "comment" event.
type LiveComment struct {
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
	File   string `json:"file"`
	Link   string `json:"link"`
	Text   string `json:"text"`
}

// LiveConnection is an open live connection.
type LiveConnection struct {
	conn *websocket.Conn
}

// ConnectLive opens the live connection, a WebSocket to /api/live.
func (c *Client) ConnectLive(ctx context.Context) (*LiveConnection, error) {
	if !c.Account.Credentials.HasApiKey() {
		return nil, PuushErrorInvalidCredentials
	}
	address := c.FormatURL("/api/live")
	address = "ws" + strings.TrimPrefix(address, "http")
	header := http.Header{}
	header.Set("X-Api-Key", *c.Account.Credentials.Key)
	header.Set("User-Agent", "puush")
	// The same client as the other requests, with a time limit for the
	// handshake only
	httpClient := *c.httpClient
	httpClient.Timeout = 20 * time.Second
	conn, response, err := websocket.Dial(ctx, address, &websocket.DialOptions{HTTPHeader: header, HTTPClient: &httpClient})
	if err != nil {
		if response != nil {
			switch response.StatusCode {
			case http.StatusUnauthorized:
				return nil, PuushErrorInvalidCredentials
			case http.StatusNotFound, http.StatusMethodNotAllowed:
				return nil, ErrNotSupported
			}
		}
		return nil, err
	}
	conn.SetReadLimit(1 << 20)
	return &LiveConnection{conn: conn}, nil
}

// Next waits for the next event.
func (l *LiveConnection) Next(ctx context.Context) (*LiveEvent, error) {
	_, data, err := l.conn.Read(ctx)
	if err != nil {
		return nil, err
	}
	event := &LiveEvent{}
	return event, json.Unmarshal(data, event)
}

// Typing tells the other person in a chat that the user is typing.
func (l *LiveConnection) Typing(ctx context.Context, to string) error {
	data, _ := json.Marshal(map[string]string{"type": "typing", "to": to})
	return l.conn.Write(ctx, websocket.MessageText, data)
}

// Close closes the connection.
func (l *LiveConnection) Close() error {
	return l.conn.Close(websocket.StatusNormalClosure, "")
}

// LoggedOut tells whether the server closed the connection because the
// API key stopped working.
func LoggedOut(err error) bool {
	return websocket.CloseStatus(err) == websocket.StatusPolicyViolation
}
