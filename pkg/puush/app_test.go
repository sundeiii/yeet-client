package puush

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client := NewClientFromApiKey("me@example.com", "secret")
	client.SetBaseURL(server.URL)
	return client
}

func TestPoolsAndUploads(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		if r.FormValue("k") != "secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/pools":
			io.WriteString(w, `{"pools":[{"id":4,"name":"Public","type":"Public","uploads":2,"default":true},{"id":6,"name":"Gallery","type":"Gallery","uploads":0}]}`)
		case "/api/uploads":
			if r.FormValue("q") != "cat" || r.FormValue("l") != "20" || r.FormValue("o") != "40" {
				t.Errorf("unexpected form: %v", r.Form)
			}
			io.WriteString(w, `{"total":41,"uploads":[{"id":9,"filename":"cat.png","url":"https://x/abc.png","created":"2026-09-25T10:00:00Z","views":3,"size":1536,"kind":"image","pool":"Public"}]}`)
		}
	})

	pools, err := client.Pools()
	if err != nil || len(pools) != 2 || !pools[0].Default || pools[1].Name != "Gallery" {
		t.Fatalf("Pools() = %+v, %v", pools, err)
	}

	page, err := client.Uploads("cat", 40, 20)
	if err != nil || page.Total != 41 || len(page.Uploads) != 1 {
		t.Fatalf("Uploads() = %+v, %v", page, err)
	}
	upload := page.Uploads[0]
	if upload.Kind != "image" || upload.SizeHumanReadable() != "1.50KB" || upload.Created.Year() != 2026 {
		t.Errorf("unexpected upload %+v", upload)
	}
}

func TestOldServersAreNotSupported(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		// Old servers answer unknown paths with a web page
		w.Header().Set("Content-Type", "text/html")
		io.WriteString(w, "<html></html>")
	})
	if _, err := client.Pools(); !errors.Is(err, ErrNotSupported) {
		t.Errorf("expected ErrNotSupported, got %v", err)
	}

	client = newTestClient(t, http.NotFound)
	if _, err := client.Uploads("", 0, 10); !errors.Is(err, ErrNotSupported) {
		t.Errorf("expected ErrNotSupported for a 404, got %v", err)
	}
}

func TestUploadIntoPool(t *testing.T) {
	var gotPool, gotName, gotBody string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		r.ParseMultipartForm(1 << 20)
		gotPool = r.FormValue("p")
		file, header, _ := r.FormFile("f")
		data, _ := io.ReadAll(file)
		gotName, gotBody = header.Filename, string(data)
		io.WriteString(w, "0,https://x/abc.txt,123,123")
	})

	link, err := client.UploadWithOptions(context.Background(), strings.NewReader("hello"), "a.txt", UploadOptions{PoolId: 6})
	if err != nil || link != "https://x/abc.txt" {
		t.Fatalf("upload = %q, %v", link, err)
	}
	if gotPool != "6" || gotName != "a.txt" || gotBody != "hello" || client.Account.DiskUsage != 123 {
		t.Errorf("server got pool %q, file %q %q, usage %d", gotPool, gotName, gotBody, client.Account.DiskUsage)
	}

	// No pool field when none was chosen
	client.Upload(strings.NewReader("hello"), "a.txt")
	if gotPool != "" {
		t.Errorf("no pool should be sent by default, got %q", gotPool)
	}
}

func TestUploadConnectionErrors(t *testing.T) {
	// A server that can't be reached is a connection error, which can be retried
	client := NewClientFromApiKey("me@example.com", "secret")
	client.SetBaseURL("http://127.0.0.1:1")
	client.httpClient = &http.Client{Timeout: 2 * time.Second}
	_, err := client.Upload(strings.NewReader("hello"), "a.txt")
	if err != PuushErrorRequestFailure || !ShouldRetryError(err) {
		t.Errorf("expected a retryable connection error, got %v", err)
	}
}

func TestHistoryWithCommasInFilenames(t *testing.T) {
	response := "2\n5,2026-09-25 10:00:00,https://x/a.png,a, b, c.png,7\n4,2026-09-24 09:00:00,https://x/b.png,b.png,0"
	scanner := bufio.NewScanner(strings.NewReader(response))
	scanner.Scan()
	items, err := NewHistoryItemsFromResponse(scanner)
	if err != nil {
		t.Fatal(err)
	}
	if items[0].FileName != "a, b, c.png" || items[0].Views != 7 || items[1].FileName != "b.png" {
		t.Errorf("unexpected items: %+v %+v", items[0], items[1])
	}
}

func TestCreateAlbum(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		if r.URL.Path != "/api/album" || len(r.Form["u"]) != 2 || r.FormValue("u") != "https://x/a.png" {
			t.Errorf("unexpected request %s %v", r.URL.Path, r.Form)
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"url": "https://x/a/abcdefgh"}`)
	})
	link, err := client.CreateAlbum([]string{"https://x/a.png", "https://x/b.png"}, "")
	if err != nil || link != "https://x/a/abcdefgh" {
		t.Errorf("CreateAlbum() = %q, %v", link, err)
	}
}

func TestChunkedUpload(t *testing.T) {
	var received bytes.Buffer
	failedOnce := false
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/chunk/start":
			r.ParseForm()
			if r.FormValue("size") != "250" || r.FormValue("name") != "big.mp4" || r.FormValue("p") != "6" {
				t.Errorf("start: %v", r.Form)
			}
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"id": "abc", "chunk": 100}`)
		case r.URL.Path == "/api/chunk/abc/finish":
			io.WriteString(w, "0,https://x/big.mp4,500,500")
		case r.URL.Path == "/api/chunk/abc":
			// The second piece fails once, like a flaky connection
			if r.URL.Query().Get("o") == "100" && !failedOnce {
				failedOnce = true
				w.WriteHeader(http.StatusBadGateway)
				return
			}
			if r.URL.Query().Get("k") != "secret" || r.URL.Query().Get("o") != strconv.Itoa(received.Len()) {
				t.Errorf("piece: %v", r.URL.Query())
			}
			io.Copy(&received, r.Body)
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"received": %d}`, received.Len())
		}
	})

	file := bytes.Repeat([]byte("x"), 250)
	link, err := client.uploadChunked(context.Background(), bytes.NewReader(file), "big.mp4", UploadOptions{PoolId: 6, Size: 250})
	if err != nil || link != "https://x/big.mp4" || client.Account.DiskUsage != 500 {
		t.Fatalf("uploadChunked() = %q, %v", link, err)
	}
	if !bytes.Equal(received.Bytes(), file) || !failedOnce {
		t.Errorf("server got %d bytes (retried: %v)", received.Len(), failedOnce)
	}
}

func TestTwoFactorLogin(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		switch r.FormValue("c") {
		case "":
			w.WriteHeader(http.StatusUnauthorized)
			io.WriteString(w, "-5")
		case "123456":
			io.WriteString(w, "1,thekey,,42")
		default:
			w.WriteHeader(http.StatusUnauthorized)
			io.WriteString(w, "-6")
		}
	})
	email, password := "me@example.com", "hunter22"
	client.Account.Credentials = &Credentials{Identifier: &email, Password: &password}

	if err := client.Authenticate(); err != PuushErrorTwoFactorRequired {
		t.Fatalf("expected a code to be asked for, got %v", err)
	}
	wrong := "000000"
	client.Account.Credentials.TwoFactorCode = &wrong
	if err := client.Authenticate(); err != PuushErrorTwoFactorWrong {
		t.Fatalf("expected a wrong code error, got %v", err)
	}
	right := "123456"
	client.Account.Credentials.TwoFactorCode = &right
	if err := client.Authenticate(); err != nil || *client.Account.Credentials.Key != "thekey" {
		t.Fatalf("login with the code: %v", err)
	}
}
