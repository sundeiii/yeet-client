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

	uploadUrl, updatedDiskUsage, err := parseUploadResponse(scanner.Text())
	if err != nil {
		return "", err
	}

	c.Account.DiskUsage = updatedDiskUsage
	return uploadUrl, nil
}

func parseUploadResponse(responseLine string) (string, int64, error) {
	responseData := strings.SplitN(responseLine, ",", 4)
	if len(responseData) < 3 || responseData[0] != "0" || responseData[1] == "" {
		return "", 0, errors.New("response error: malformed upload response")
	}

	updatedDiskUsage, err := strconv.ParseInt(responseData[2], 10, 64)
	if err != nil {
		return "", 0, errors.New("response error: invalid disk usage provided")
	}
	return responseData[1], updatedDiskUsage, nil
}
