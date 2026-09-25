package screenshots

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"io"
)

// ApplyQuality decompresses and recompresses an image to jpeg if the quality is not `QualityBest`
func ApplyQuality(reader io.ReadSeekCloser, quality Quality) (io.ReadSeekCloser, error) {
	if quality == QualityBest {
		return reader, nil
	}

	// Ensure we're at the start of the file for decoding
	if _, err := reader.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("failed to seek: %w", err)
	}
	defer reader.Close()

	img, _, err := image.Decode(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image for compression: %w", err)
	}

	var buf bytes.Buffer
	var options = jpeg.Options{Quality: quality.Value()}
	fmt.Printf("Compressing image with quality %d%%\n", options.Quality)

	if err := jpeg.Encode(&buf, img, &options); err != nil {
		return nil, fmt.Errorf("failed to compress image: %w", err)
	}
	return &memoryReadCloser{Reader: bytes.NewReader(buf.Bytes())}, nil
}
