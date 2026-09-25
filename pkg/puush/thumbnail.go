package puush

import (
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Thumbnail retrieves the thumbnail of an uploaded file by its ID.
func (c *Client) Thumbnail(id int) (io.ReadCloser, error) {
	if !c.Account.Credentials.HasApiKey() {
		return nil, PuushErrorInvalidCredentials
	}

	params := url.Values{}
	params.Add("k", *c.Account.Credentials.Key)
	params.Add("i", strconv.Itoa(id))

	request, err := http.NewRequest("POST", c.FormatURL("/api/thumb"), strings.NewReader(params.Encode()))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("User-Agent", "puush")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, err
	}

	if response.StatusCode != http.StatusOK {
		defer response.Body.Close()
		return nil, c.EvaluateHttpResponse(response)
	}

	return response.Body, nil
}
