package puush

import (
	"bufio"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type HistoryItem struct {
	Id       int
	Time     time.Time
	Url      string
	FileName string
	Views    int
}

func NewHistoryItemFromResponse(line string) (*HistoryItem, error) {
	parts := strings.Split(line, ",")
	uploadId, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, errors.New("expected upload ID")
	}

	uploadTime, err := time.Parse(time.DateTime, parts[1])
	if err != nil {
		return nil, errors.New("expected upload time")
	}

	views, err := strconv.Atoi(parts[4])
	if err != nil {
		return nil, errors.New("expected number of views")
	}

	return &HistoryItem{
		Id:       uploadId,
		Time:     uploadTime,
		Url:      parts[2],
		FileName: parts[3],
		Views:    views,
	}, nil
}

func NewHistoryItemsFromResponse(scanner *bufio.Scanner) ([]*HistoryItem, error) {
	amountUploads, err := strconv.Atoi(scanner.Text())
	if err != nil {
		return nil, errors.New("response error: expected number of uploads")
	}

	history := make([]*HistoryItem, 0, amountUploads)
	for range amountUploads {
		if !scanner.Scan() {
			return nil, errors.New("response error: expected more history items")
		}

		line := scanner.Text()
		item, err := NewHistoryItemFromResponse(line)
		if err != nil {
			return nil, errors.New("response error: " + err.Error())
		}
		history = append(history, item)
	}

	return history, nil
}

// History retrieves the 5 most recent uploads of the authenticated user.
func (c *Client) History() ([]*HistoryItem, error) {
	if !c.Account.Credentials.HasApiKey() {
		return nil, PuushErrorInvalidCredentials
	}

	params := url.Values{}
	params.Add("k", *c.Account.Credentials.Key)

	request, err := http.NewRequest("POST", c.FormatURL("/api/hist"), strings.NewReader(params.Encode()))
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

	history, err := NewHistoryItemsFromResponse(scanner)
	if err != nil {
		return nil, err
	}

	return history, nil
}
