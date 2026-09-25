package screenshots

import (
	"bytes"
	"errors"
	"os"
)

// temporaryFileReader is a `ReadSeekCloser` that deletes the underlying file when closed
type temporaryFileReader struct {
	file *os.File
	path string
}

func (c *temporaryFileReader) Read(p []byte) (int, error) {
	return c.file.Read(p)
}

func (c *temporaryFileReader) Seek(offset int64, whence int) (int64, error) {
	return c.file.Seek(offset, whence)
}

func (c *temporaryFileReader) Stat() (os.FileInfo, error) {
	return c.file.Stat()
}

func (c *temporaryFileReader) Close() error {
	closeErr := c.file.Close()
	removeErr := os.Remove(c.path)

	if closeErr != nil {
		return closeErr
	}
	if removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
		return removeErr
	}
	return nil
}

// idk bro i just needed a bytes reader that can close
type memoryReadCloser struct {
	*bytes.Reader
}

func (b *memoryReadCloser) Close() error {
	return nil
}
