package puush

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Pools and the full uploads list come from newer endpoints that answer in
// JSON. Servers without them return ErrNotSupported.

// ErrNotSupported means the server doesn't offer this feature.
var ErrNotSupported = errors.New("puush: not supported by this server")

// Pool is one of the account's pools (folders) on the server.
type Pool struct {
	Id      int    `json:"id"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Uploads int    `json:"uploads"`
	Default bool   `json:"default"`
}

// Upload is a file in the account, as listed by Uploads.
type Upload struct {
	Id       int       `json:"id"`
	Filename string    `json:"filename"`
	Url      string    `json:"url"`
	Created  time.Time `json:"created"`
	Views    int       `json:"views"`
	Size     int64     `json:"size"`
	Kind     string    `json:"kind"` // image, video, audio, text, pdf, archive or file
	Pool     string    `json:"pool"`
}

// SizeHumanReadable returns the size like "1.50MB".
func (u *Upload) SizeHumanReadable() string {
	return formatBytes(u.Size)
}

// UploadsPage is one page of the account's uploads.
type UploadsPage struct {
	Total   int       `json:"total"`
	Uploads []*Upload `json:"uploads"`
}

// Pools lists the account's pools.
func (c *Client) Pools() ([]*Pool, error) {
	var body struct {
		Pools []*Pool `json:"pools"`
	}
	if err := c.postJson("/api/pools", url.Values{}, &body); err != nil {
		return nil, err
	}
	return body.Pools, nil
}

// Uploads lists the account's uploads, newest first. The search text filters
// by filename and may be empty.
func (c *Client) Uploads(search string, offset, limit int) (*UploadsPage, error) {
	params := url.Values{}
	params.Add("q", search)
	params.Add("o", strconv.Itoa(offset))
	params.Add("l", strconv.Itoa(limit))

	page := &UploadsPage{}
	if err := c.postJson("/api/uploads", params, page); err != nil {
		return nil, err
	}
	return page, nil
}

func (c *Client) postJson(path string, params url.Values, target any) error {
	if !c.Account.Credentials.HasApiKey() {
		return PuushErrorInvalidCredentials
	}
	params.Set("k", *c.Account.Credentials.Key)

	request, err := http.NewRequest("POST", c.FormatURL(path), strings.NewReader(params.Encode()))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("User-Agent", "puush")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return PuushErrorRequestFailure
	}
	defer response.Body.Close()

	switch {
	case response.StatusCode == http.StatusNotFound || response.StatusCode == http.StatusMethodNotAllowed:
		return ErrNotSupported
	case response.StatusCode != http.StatusOK:
		return c.EvaluateHttpResponse(response)
	case !strings.HasPrefix(response.Header.Get("Content-Type"), "application/json"):
		// Older servers answer unknown paths with a web page
		return ErrNotSupported
	}
	return json.NewDecoder(response.Body).Decode(target)
}

// CreateAlbum groups uploads, given by their links, into an album and
// returns the album's link.
func (c *Client) CreateAlbum(links []string, title string) (string, error) {
	params := url.Values{}
	for _, link := range links {
		params.Add("u", link)
	}
	params.Add("t", title)

	var body struct {
		Url string `json:"url"`
	}
	if err := c.postJson("/api/album", params, &body); err != nil {
		return "", err
	}
	if body.Url == "" {
		return "", errors.New("puush: the server did not return an album link")
	}
	return body.Url, nil
}
