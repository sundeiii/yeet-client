package puush

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Files over this size go up in pieces: Cloudflare, which many servers sit
// behind, refuses requests over 100 MB.
const chunkThreshold = 90 << 20

// uploadChunked sends a file in pieces and returns its link like Upload.
func (c *Client) uploadChunked(ctx context.Context, file io.Reader, filename string, options UploadOptions) (string, error) {
	params := url.Values{}
	params.Set("name", filename)
	params.Set("size", strconv.FormatInt(options.Size, 10))
	if options.PoolId > 0 {
		params.Set("p", strconv.Itoa(options.PoolId))
	}
	var started struct {
		Id    string `json:"id"`
		Chunk int    `json:"chunk"`
	}
	if err := c.postJson("/api/chunk/start", params, &started); err != nil {
		return "", err
	}
	if started.Id == "" || started.Chunk <= 0 {
		return "", errors.New("puush: unexpected answer when starting an upload")
	}

	buffer := make([]byte, started.Chunk)
	var offset int64
	for {
		n, readErr := io.ReadFull(file, buffer)
		if n > 0 {
			if err := c.sendChunk(ctx, started.Id, offset, buffer[:n]); err != nil {
				return "", err
			}
			offset += int64(n)
		}
		if readErr == io.EOF || readErr == io.ErrUnexpectedEOF {
			break
		}
		if readErr != nil {
			return "", readErr
		}
	}

	return c.finishChunked(ctx, started.Id, options.Duplicate)
}

// sendChunk sends one piece, trying again a few times when the connection
// hiccups: big uploads take long enough for that to happen.
func (c *Client) sendChunk(ctx context.Context, id string, offset int64, piece []byte) error {
	target := c.FormatURL(fmt.Sprintf("/api/chunk/%s?k=%s&o=%d", url.PathEscape(id), url.QueryEscape(*c.Account.Credentials.Key), offset))

	var lastErr error
	for attempt := 0; attempt < 4; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(attempt) * 2 * time.Second):
			}
		}

		request, err := http.NewRequestWithContext(ctx, "POST", target, bytes.NewReader(piece))
		if err != nil {
			return err
		}
		request.Header.Set("Content-Type", "application/octet-stream")
		request.Header.Set("User-Agent", "puush")

		response, err := c.httpClient.Do(request)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			lastErr = PuushErrorRequestFailure
			continue
		}
		var body struct {
			Received int64 `json:"received"`
		}
		json.NewDecoder(io.LimitReader(response.Body, 4096)).Decode(&body)
		response.Body.Close()

		switch {
		case response.StatusCode == http.StatusOK:
			return nil
		case response.StatusCode == http.StatusConflict && body.Received == offset+int64(len(piece)):
			// It arrived before; only the answer got lost
			return nil
		case response.StatusCode >= 500:
			lastErr = PuushErrorRequestFailure
		default:
			return c.EvaluateHttpResponse(response)
		}
	}
	return lastErr
}

func (c *Client) finishChunked(ctx context.Context, id string, duplicate *bool) (string, error) {
	params := url.Values{}
	params.Set("k", *c.Account.Credentials.Key)
	params.Set("d", "1")
	request, err := http.NewRequestWithContext(ctx, "POST", c.FormatURL("/api/chunk/"+url.PathEscape(id)+"/finish"), strings.NewReader(params.Encode()))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("User-Agent", "puush")

	response, err := c.httpClient.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", PuushErrorRequestFailure
	}
	defer response.Body.Close()

	scanner, err := c.EvaluateResponse(response)
	if err != nil {
		return "", err
	}
	link, usage, wasDuplicate, err := parseUploadResponse(scanner.Text())
	if err != nil {
		return "", err
	}
	if duplicate != nil {
		*duplicate = wasDuplicate
	}
	c.Account.DiskUsage = usage
	return link, nil
}
