package puush

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
)

// LoginLink asks the server for a one-time link that logs the user in on the
// website. Not every puush server supports this.
func (c *Client) LoginLink() (string, error) {
	if !c.Account.Credentials.HasApiKey() {
		return "", PuushErrorInvalidCredentials
	}

	params := url.Values{}
	params.Add("k", *c.Account.Credentials.Key)

	request, err := http.NewRequest("POST", c.FormatURL("/api/loginlink"), strings.NewReader(params.Encode()))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("User-Agent", "puush")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	scanner, err := c.EvaluateResponse(response)
	if err != nil {
		return "", err
	}
	_, link, ok := strings.Cut(scanner.Text(), ",")
	if !ok || !strings.HasPrefix(link, "http") {
		return "", errors.New("puush: unexpected login link response")
	}
	return link, nil
}

// AccountLink is the address that opens the user's account on the website.
// It prefers a one-time login link, and only falls back to a link containing
// the API key for servers that don't offer those.
func (c *Client) AccountLink() string {
	if link, err := c.LoginLink(); err == nil {
		return link
	}
	if !c.Account.Credentials.HasApiKey() {
		return c.FormatURL("/login")
	}
	return c.FormatURL("/login/go/?k=" + url.QueryEscape(*c.Account.Credentials.Key))
}
