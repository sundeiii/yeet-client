//go:build windows

package screenshots

import (
	"bytes"
	"image"
	"image/gif"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestRecordingForReal records the top left of the screen as a video and
// a GIF. It needs a desktop, so it only runs when asked:
//
//	YEET_RECORD_TEST=<folder to keep the files in> go test ./internal/screenshots -run TestRecordingForReal
func TestRecordingForReal(t *testing.T) {
	folder := os.Getenv("YEET_RECORD_TEST")
	if folder == "" {
		t.Skip("set YEET_RECORD_TEST to a folder to record the screen")
	}
	area := image.Rect(0, 0, 801, 601) // odd on purpose: videos need an even size

	for _, format := range []RecordingFormat{RecordingMP4, RecordingGIF} {
		path := filepath.Join(folder, "test."+string(format))
		recording, err := StartRecording(area, format, path)
		if err != nil {
			t.Fatalf("%s: %v", format, err)
		}
		time.Sleep(2 * time.Second)
		if err := recording.Stop(); err != nil {
			t.Fatalf("%s: stop: %v", format, err)
		}
		data, err := os.ReadFile(path)
		if err != nil || len(data) < 1000 {
			t.Fatalf("%s: file: %v, %d bytes", format, err, len(data))
		}
		t.Logf("%s: %d KB", format, len(data)/1024)

		switch format {
		case RecordingMP4:
			if !bytes.Contains(data[:64], []byte("ftyp")) {
				t.Errorf("not an MP4 file")
			}
		case RecordingGIF:
			decoded, err := gif.DecodeAll(bytes.NewReader(data))
			if err != nil {
				t.Fatalf("GIF: %v", err)
			}
			total := 0
			for _, delay := range decoded.Delay {
				total += delay
			}
			if total < 150 || total > 300 {
				t.Errorf("the GIF should last about 2 seconds, got %d centiseconds in %d frames", total, len(decoded.Image))
			}
		}
	}
}
