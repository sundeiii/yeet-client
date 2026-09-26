package puush

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/sundeiii/yeet-client/internal/i18n"
)

// Chats, the profile, text snippets and logging in through the website.
// These are newer endpoints of yeet servers; others return ErrNotSupported.

// ServerError is an error message from the server, meant for people.
type ServerError struct{ Message string }

func (e *ServerError) Error() string { return e.Message }

// Profile is the account as the server sees it.
type Profile struct {
	Email   string `json:"email"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Usage   int64  `json:"usage"`
	Limit   int64  `json:"limit"` // -1 for unlimited
	Unread  int    `json:"unread"`
	Profile string `json:"profile"` // link to the profile page, if there's a username
	Avatar  string `json:"avatar"`  // link to the avatar picture (PNG)
	// ChatPool is the pool files sent in chats go into (0 on older servers)
	ChatPool int `json:"chatPool"`
}

// Chat is a conversation in the list of chats.
type Chat struct {
	With   string `json:"with"` // the other person's username
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
	Text   string `json:"text"` // the latest message
	FromMe bool   `json:"fromMe"`
	Unread int    `json:"unread"`
	Time   string `json:"time"`
}

// ChatMessage is one message in a chat.
type ChatMessage struct {
	Id   int       `json:"id"`
	Mine bool      `json:"mine"`
	Text string    `json:"text"`
	Time string    `json:"time"`
	File *ChatFile `json:"file"` // a file sent with the message, if any
}

// ChatFile is a file sent in a chat.
type ChatFile struct {
	Name  string `json:"name"`
	Url   string `json:"url"`
	Thumb string `json:"thumb"`
	Kind  string `json:"kind"` // image, video, audio, text or file
	Size  string `json:"size"` // like "1.2MB"
}

// ChatThread is the messages of a chat.
type ChatThread struct {
	With     string         `json:"with"`
	Name     string         `json:"name"`
	Messages []*ChatMessage `json:"messages"`
	CantSend string         `json:"cantSend"` // why messages can't be sent, if they can't
}

// Snippet is text saved as a text file.
type Snippet struct {
	Name      string `json:"name"`
	Url       string `json:"url"`
	Duplicate bool   `json:"duplicate"`
}

// AppLogin is a started login through the website.
type AppLogin struct {
	Code    string `json:"code"`
	Url     string `json:"url"`
	Expires int    `json:"expires"` // seconds
}

// ErrLoginWaiting means the login wasn't approved on the website yet.
var ErrLoginWaiting = errors.New("puush: waiting for the login to be approved")

// ErrLoginExpired means the login was denied or took too long.
var ErrLoginExpired = errors.New("puush: the login expired or was denied")

// Profile returns the account's profile, storage and unread messages.
func (c *Client) Profile() (*Profile, error) {
	profile := &Profile{}
	return profile, c.appRequest("/api/profile", url.Values{}, true, profile)
}

// Chats lists the account's chats, the latest first, and the unread count.
func (c *Client) Chats() ([]*Chat, int, error) {
	var body struct {
		Chats  []*Chat `json:"chats"`
		Unread int     `json:"unread"`
	}
	err := c.appRequest("/api/chats", url.Values{}, true, &body)
	return body.Chats, body.Unread, err
}

// ChatWith returns a chat's messages after an id (0 for the latest), and
// marks them read.
func (c *Client) ChatWith(name string, after int) (*ChatThread, error) {
	thread := &ChatThread{}
	params := url.Values{}
	params.Set("after", strconv.Itoa(after))
	return thread, c.appRequest("/api/chats/"+url.PathEscape(name), params, true, thread)
}

// SendMessage sends a chat message.
func (c *Client) SendMessage(name, text string) (*ChatMessage, error) {
	message := &ChatMessage{}
	params := url.Values{}
	params.Set("text", text)
	return message, c.appRequest("/api/chats/"+url.PathEscape(name)+"/send", params, true, message)
}

// SendFile sends one of the account's uploads (its link) in a chat, with an
// optional text. The other person can open it even if it's restricted.
func (c *Client) SendFile(name, link, text string) (*ChatMessage, error) {
	message := &ChatMessage{}
	params := url.Values{}
	params.Set("i", link)
	params.Set("text", text)
	return message, c.appRequest("/api/chats/"+url.PathEscape(name)+"/send", params, true, message)
}

// SaveSnippet saves text as a text file and returns its link.
func (c *Client) SaveSnippet(text, name string, poolId int) (*Snippet, error) {
	snippet := &Snippet{}
	params := url.Values{}
	params.Set("text", text)
	params.Set("name", name)
	if poolId > 0 {
		params.Set("p", strconv.Itoa(poolId))
	}
	return snippet, c.appRequest("/api/snippet", params, true, snippet)
}

// StartAppLogin starts logging in through the website. device is shown
// there, like "Windows".
func (c *Client) StartAppLogin(device string) (*AppLogin, error) {
	login := &AppLogin{}
	params := url.Values{}
	params.Set("device", device)
	return login, c.appRequest("/api/applogin/start", params, false, login)
}

// PollAppLogin checks a login through the website. Once approved, the
// client uses the account's API key; until then it returns ErrLoginWaiting.
func (c *Client) PollAppLogin(code string) error {
	var body struct {
		Status string `json:"status"`
		Email  string `json:"email"`
		Key    string `json:"key"`
		Type   int    `json:"type"`
		Usage  int64  `json:"usage"`
	}
	params := url.Values{}
	params.Set("code", code)
	err := c.appRequest("/api/applogin/poll", params, false, &body)
	switch {
	case errors.Is(err, ErrNotSupported):
		return ErrLoginExpired
	case err != nil:
		return err
	case body.Status == "waiting":
		return ErrLoginWaiting
	case body.Status != "done" || body.Key == "":
		return ErrLoginExpired
	}
	c.Account.Credentials = &Credentials{Identifier: stringOrNil(body.Email), Key: stringOrNil(body.Key)}
	c.Account.Type = AccountType(body.Type)
	c.Account.DiskUsage = body.Usage
	return nil
}

// appRequest posts to a JSON endpoint, with the API key when withKey, and
// turns the server's error messages into ServerError.
func (c *Client) appRequest(path string, params url.Values, withKey bool, target any) error {
	if withKey {
		if !c.Account.Credentials.HasApiKey() {
			return PuushErrorInvalidCredentials
		}
		params.Set("k", *c.Account.Credentials.Key)
	}
	request, err := http.NewRequest("POST", c.FormatURL(path), strings.NewReader(params.Encode()))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("User-Agent", "puush")
	// Messages from the server in the app's language
	request.Header.Set("Accept-Language", i18n.Current())

	response, err := c.httpClient.Do(request)
	if err != nil {
		return PuushErrorRequestFailure
	}
	defer response.Body.Close()

	if !strings.HasPrefix(response.Header.Get("Content-Type"), "application/json") {
		// Servers without these endpoints answer with a web page or plain text
		if response.StatusCode == http.StatusUnauthorized {
			return PuushErrorInvalidCredentials
		}
		return ErrNotSupported
	}
	if response.StatusCode == http.StatusOK {
		return json.NewDecoder(response.Body).Decode(target)
	}
	if response.StatusCode == http.StatusUnauthorized {
		return PuushErrorInvalidCredentials
	}
	var failure struct {
		Error string `json:"error"`
	}
	json.NewDecoder(response.Body).Decode(&failure)
	if failure.Error != "" {
		return &ServerError{Message: failure.Error}
	}
	if response.StatusCode == http.StatusNotFound {
		return ErrNotSupported
	}
	return c.EvaluateHttpResponse(response)
}

// Picture downloads a picture from the server, like an avatar. link is a
// full address or a path on the server.
func (c *Client) Picture(link string) ([]byte, error) {
	if strings.HasPrefix(link, "/") {
		link = c.FormatURL(link)
	}
	response, err := c.httpClient.Get(link)
	if err != nil {
		return nil, PuushErrorRequestFailure
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || !strings.HasPrefix(response.Header.Get("Content-Type"), "image/") {
		return nil, ErrNotSupported
	}
	return io.ReadAll(io.LimitReader(response.Body, 2<<20))
}
