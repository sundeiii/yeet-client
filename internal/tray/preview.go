package tray

import (
	"bytes"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/image/draw"
)

// Notifications can show a small square of the uploaded picture instead of
// the puush logo.

const (
	previewSize    = 192
	previewMaxFile = 25 << 20
)

var previewExtensions = map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".gif": true}

// previewIcon makes the notification picture for an upload, or nil when it
// isn't a picture.
func previewIcon(job *uploadJob) []byte {
	if job == nil {
		return nil
	}
	data := job.Data
	if data == nil {
		if !previewExtensions[strings.ToLower(filepath.Ext(job.Name))] {
			return nil
		}
		info, err := os.Stat(job.Path)
		if err != nil || info.Size() > previewMaxFile {
			return nil
		}
		if data, err = os.ReadFile(job.Path); err != nil {
			return nil
		}
	}
	return squarePreview(data)
}

// squarePreview scales the middle of a picture down to a small square PNG.
func squarePreview(data []byte) []byte {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil
	}
	bounds := img.Bounds()
	side := min(bounds.Dx(), bounds.Dy())
	if side <= 0 {
		return nil
	}
	crop := image.Rect(0, 0, side, side).Add(image.Pt(
		bounds.Min.X+(bounds.Dx()-side)/2,
		bounds.Min.Y+(bounds.Dy()-side)/2,
	))

	preview := image.NewRGBA(image.Rect(0, 0, previewSize, previewSize))
	draw.CatmullRom.Scale(preview, preview.Bounds(), img, crop, draw.Src, nil)

	var buffer bytes.Buffer
	if err := png.Encode(&buffer, preview); err != nil {
		return nil
	}
	return buffer.Bytes()
}
