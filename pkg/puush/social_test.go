package puush

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func writeJson(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}

func TestSnippetAndServerErrors(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		switch {
		case r.FormValue("k") != "secret":
			w.WriteHeader(http.StatusUnauthorized)
		case r.FormValue("text") == "":
			writeJson(w, 400, map[string]string{"error": "There's no text to save."})
		default:
			writeJson(w, 200, map[string]any{"name": "a.txt", "url": "https://x/a", "duplicate": r.FormValue("p") == "3"})
		}
	})

	snippet, err := client.SaveSnippet("hello", "", 3)
	if err != nil || snippet.Url != "https://x/a" || !snippet.Duplicate {
		t.Fatalf("snippet = %+v, %v", snippet, err)
	}
	_, err = client.SaveSnippet("", "", 0)
	if FormatError(err) != "There's no text to save." {
		t.Errorf("the server's message should be shown, got %v", err)
	}
}

func TestOlderServersAreNotSupported(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusNotFound)
	})
	if _, _, err := client.Chats(); !errors.Is(err, ErrNotSupported) {
		t.Errorf("expected ErrNotSupported, got %v", err)
	}
	if _, err := client.ConnectLive(context.Background()); !errors.Is(err, ErrNotSupported) {
		t.Errorf("live: expected ErrNotSupported, got %v", err)
	}
}

func TestAppLogin(t *testing.T) {
	polls := 0
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		switch r.URL.Path {
		case "/api/applogin/start":
			if r.FormValue("k") != "" || r.FormValue("device") != "PC (Linux)" {
				t.Errorf("start got %v", r.Form)
			}
			writeJson(w, 200, map[string]any{"code": "abc", "url": "https://x/app-login/abc", "expires": 600})
		case "/api/applogin/poll":
			polls++
			if polls == 1 {
				writeJson(w, 200, map[string]string{"status": "waiting"})
				return
			}
			writeJson(w, 200, map[string]any{"status": "done", "email": "lilian@example.com", "key": "newkey", "type": 1, "usage": 42})
		}
	})
	client.Account.Reset()

	login, err := client.StartAppLogin("PC (Linux)")
	if err != nil || login.Code != "abc" {
		t.Fatalf("start = %+v, %v", login, err)
	}
	if err := client.PollAppLogin("abc"); err != ErrLoginWaiting {
		t.Fatalf("first poll: %v", err)
	}
	if err := client.PollAppLogin("abc"); err != nil {
		t.Fatalf("second poll: %v", err)
	}
	if *client.Account.Credentials.Key != "newkey" || *client.Account.Credentials.Identifier != "lilian@example.com" || client.Account.DiskUsage != 42 {
		t.Errorf("account after login: %+v", client.Account)
	}
}

func TestAppLoginExpired(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJson(w, 404, map[string]string{"status": "expired"})
	})
	if err := client.PollAppLogin("abc"); err != ErrLoginExpired {
		t.Errorf("expected ErrLoginExpired, got %v", err)
	}
}

func TestLiveConnection(t *testing.T) {
	typed := make(chan string, 1)
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/live" || r.Header.Get("X-Api-Key") != "secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()
		conn.Write(r.Context(), websocket.MessageText, []byte(`{"type":"message","data":{"with":"mika","name":"Mika","text":"hi","unread":2,"message":{"id":7,"text":"hi"}}}`))
		_, data, err := conn.Read(r.Context())
		if err == nil {
			typed <- string(data)
		}
		conn.Close(websocket.StatusPolicyViolation, "logged out")
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	live, err := client.ConnectLive(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer live.Close()

	event, err := live.Next(ctx)
	if err != nil || event.Type != "message" {
		t.Fatalf("event = %+v, %v", event, err)
	}
	var message LiveMessage
	json.Unmarshal(event.Data, &message)
	if message.With != "mika" || message.Unread != 2 || message.Message.Id != 7 {
		t.Errorf("message = %+v", message)
	}

	if err := live.Typing(ctx, "mika"); err != nil {
		t.Fatal(err)
	}
	if got := <-typed; got != `{"to":"mika","type":"typing"}` {
		t.Errorf("typing sent %s", got)
	}
	if _, err := live.Next(ctx); !LoggedOut(err) {
		t.Errorf("expected the logged out close, got %v", err)
	}

	client.Account.Credentials.Key = stringOrNil("wrong")
	if _, err := client.ConnectLive(ctx); err != PuushErrorInvalidCredentials {
		t.Errorf("wrong key: %v", err)
	}
}

func TestUploadResponseDuplicate(t *testing.T) {
	for line, want := range map[string]bool{
		"0,https://x/a.png,100,100":   false, // older servers
		"0,https://x/a.png,100,100,0": false,
		"0,https://x/a.png,100,100,1": true,
	} {
		link, usage, duplicate, err := parseUploadResponse(line)
		if err != nil || link != "https://x/a.png" || usage != 100 || duplicate != want {
			t.Errorf("%s: %q %d %v %v", line, link, usage, duplicate, err)
		}
	}
}
