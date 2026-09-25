package puush

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAccountLink(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/loginlink" {
			http.NotFound(w, r)
			return
		}
		r.ParseForm()
		if r.FormValue("k") != "secret-key" {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("-1"))
			return
		}
		w.Write([]byte("0,https://files.example.com/login/token/onetime"))
	}))
	defer server.Close()

	client := NewClientFromApiKey("me@example.com", "secret-key")
	client.SetBaseURL(server.URL)
	if link := client.AccountLink(); link != "https://files.example.com/login/token/onetime" {
		t.Errorf("expected the one-time link, got %q", link)
	}

	// Servers without one-time links fall back to the old key link
	client.SetBaseURL(server.URL + "/old")
	if link := client.AccountLink(); !strings.HasSuffix(link, "/login/go/?k=secret-key") {
		t.Errorf("expected the fallback link, got %q", link)
	}

	// A rejected key doesn't produce a login link
	wrong := NewClientFromApiKey("me@example.com", "wrong")
	wrong.SetBaseURL(server.URL)
	if _, err := wrong.LoginLink(); err == nil {
		t.Error("a wrong API key should not get a login link")
	}
}
