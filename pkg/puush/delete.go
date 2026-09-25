package puush

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Delete removes a file from puush by its ID.
// It returns the most recent history items after deletion.
func (c *Client) Delete(id int) ([]*HistoryItem, error) {
	if !c.Account.Credentials.HasApiKey() {
		return nil, PuushErrorInvalidCredentials
	}

	params := url.Values{}
	params.Add("k", *c.Account.Credentials.Key)
	params.Add("i", strconv.Itoa(id))
	params.Add("z", "poop")

	request, err := http.NewRequest("POST", c.FormatURL("/api/del"), strings.NewReader(params.Encode()))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("User-Agent", "puush")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	scanner, err := c.EvaluateResponse(response)
	if err != nil {
		return nil, err
	}

	historyItems, err := NewHistoryItemsFromResponse(scanner)
	if err != nil {
		return nil, err
	}

	return historyItems, nil
}
