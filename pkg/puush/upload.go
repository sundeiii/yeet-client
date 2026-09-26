package puush

import (
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
)

// UploadOptions are the optional settings for an upload.
type UploadOptions struct {
	// PoolId is the pool to upload into; 0 uses the account's default pool.
	// Servers that don't know about pools ignore it.
	PoolId int

	// Size of the file, if known. Big files are sent in pieces.
	Size int64

	// Duplicate, when set, tells whether the same file was uploaded before,
	// so the link is the old one. Servers that don't say leave it false.
	Duplicate *bool
}

// Upload sends a file to puush and returns the URL of the uploaded file.
// It will also update the disk usage of the account based on the response from the server.
func (c *Client) Upload(file io.Reader, filename string) (string, error) {
	return c.UploadWithOptions(context.Background(), file, filename, UploadOptions{})
}

// UploadWithOptions is Upload with a pool choice, and a context that can
// cancel the upload while it's running.
func (c *Client) UploadWithOptions(ctx context.Context, file io.Reader, filename string, options UploadOptions) (string, error) {
	if !c.Account.Credentials.HasApiKey() {
		return "", PuushErrorInvalidCredentials
	}
	if options.Size > chunkThreshold {
		link, err := c.uploadChunked(ctx, file, filename, options)
		// Servers without piece uploads get the whole file at once
		if !errors.Is(err, ErrNotSupported) {
			return link, err
		}
	}

	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	go func() {
		var err error
		defer func() {
			pw.CloseWithError(err)
		}()

		part, err := writer.CreateFormFile("f", filename)
		if err != nil {
			return
		}

		_, err = io.Copy(part, file)
		if err != nil {
			return
		}

		err = writer.WriteField("k", *c.Account.Credentials.Key)
		if err != nil {
			return
		}

		err = writer.WriteField("z", "poop")
		if err != nil {
			return
		}

		// Ask the server to say when the file was uploaded before
		err = writer.WriteField("d", "1")
		if err != nil {
			return
		}

		if options.PoolId > 0 {
			err = writer.WriteField("p", strconv.Itoa(options.PoolId))
			if err != nil {
				return
			}
		}

		err = writer.Close()
	}()

	request, err := http.NewRequestWithContext(ctx, "POST", c.FormatURL("/api/up"), pr)
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("User-Agent", "puush")

	response, err := c.httpClient.Do(request)
	if err != nil {
		// Stop the goroutine that's still feeding the request
		pr.CloseWithError(err)
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		// The server couldn't be reached; worth trying again later
		return "", PuushErrorRequestFailure
	}
	defer response.Body.Close()

	scanner, err := c.EvaluateResponse(response)
	if err != nil {
		return "", err
	}

	uploadUrl, updatedDiskUsage, duplicate, err := parseUploadResponse(scanner.Text())
	if err != nil {
		return "", err
	}
	if options.Duplicate != nil {
		*options.Duplicate = duplicate
	}

	c.Account.DiskUsage = updatedDiskUsage
	return uploadUrl, nil
}

// parseUploadResponse reads "0,link,usage,usage" and, from servers that
// say so, a fifth field that's 1 when the file was uploaded before.
func parseUploadResponse(responseLine string) (string, int64, bool, error) {
	responseData := strings.SplitN(responseLine, ",", 5)
	if len(responseData) < 3 || responseData[0] != "0" || responseData[1] == "" {
		return "", 0, false, errors.New("response error: malformed upload response")
	}

	updatedDiskUsage, err := strconv.ParseInt(responseData[2], 10, 64)
	if err != nil {
		return "", 0, false, errors.New("response error: invalid disk usage provided")
	}
	duplicate := len(responseData) == 5 && strings.TrimSpace(responseData[4]) == "1"
	return responseData[1], updatedDiskUsage, duplicate, nil
}
