package screenshots

import (
	"bytes"
	"image/gif"
	"testing"
	"time"
)

// bgraFrame is a frame of one color, with a white square at x.
func bgraFrame(width, height, x int) []byte {
	pix := make([]byte, width*height*4)
	for i := 0; i < len(pix); i += 4 {
		pix[i], pix[i+1], pix[i+2], pix[i+3] = 200, 40, 20, 255 // blue-ish BGRA: B=200 G=40 R=20
	}
	for y := 10; y < 20; y++ {
		for dx := 0; dx < 10; dx++ {
			i := (y*width + x + dx) * 4
			pix[i], pix[i+1], pix[i+2] = 255, 255, 255
		}
	}
	return pix
}

func TestGifWriter(t *testing.T) {
	var out bytes.Buffer
	g, err := newGifWriter(&out, 64, 48)
	if err != nil {
		t.Fatal(err)
	}
	g.addFrame(bgraFrame(64, 48, 0), 64, 48, 0)
	g.addFrame(bgraFrame(64, 48, 0), 64, 48, 70*time.Millisecond) // nothing changed
	g.addFrame(bgraFrame(64, 48, 30), 64, 48, 130*time.Millisecond)
	if err := g.finish(500 * time.Millisecond); err != nil {
		t.Fatal(err)
	}

	decoded, err := gif.DecodeAll(bytes.NewReader(out.Bytes()))
	if err != nil {
		t.Fatalf("not a valid GIF: %v", err)
	}
	if len(decoded.Image) != 2 {
		t.Fatalf("expected 2 frames (the unchanged one skipped), got %d", len(decoded.Image))
	}
	if decoded.Delay[0] != 13 || decoded.Delay[1] != 37 {
		t.Errorf("delays should add up to the real time: %v", decoded.Delay)
	}
	if decoded.LoopCount != 0 {
		t.Errorf("should loop forever, got %d", decoded.LoopCount)
	}
	// The second frame only covers what changed: the square moved from x 0-9 to 30-39
	if r := decoded.Image[1].Rect; r.Min.X != 0 || r.Max.X != 40 || r.Min.Y != 10 || r.Max.Y != 20 {
		t.Errorf("changed part: %v", r)
	}
	// Colors come out right, not with red and blue swapped
	r, _, b, _ := decoded.Image[0].At(50, 40).RGBA()
	if b>>8 < 150 || r>>8 > 80 {
		t.Errorf("background should be blue-ish, got r=%d b=%d", r>>8, b>>8)
	}
	white, _, _, _ := decoded.Image[0].At(5, 15).RGBA()
	if white>>8 < 240 {
		t.Errorf("the square should be white, got %d", white>>8)
	}
}

func TestGifScaling(t *testing.T) {
	width, height := gifSize(1920, 1080)
	if width != 960 || height != 540 {
		t.Errorf("1920x1080 -> %dx%d", width, height)
	}
	var out bytes.Buffer
	g, _ := newGifWriter(&out, width, height)
	g.addFrame(bgraFrame(1920, 1080, 100), 1920, 1080, 0)
	g.finish(time.Second)
	decoded, err := gif.DecodeAll(bytes.NewReader(out.Bytes()))
	if err != nil || decoded.Config.Width != 960 || decoded.Config.Height != 540 {
		t.Fatalf("scaled GIF: %v %+v", err, decoded.Config)
	}
	if w, h := gifSize(400, 300); w != 400 || h != 300 {
		t.Errorf("small areas keep their size: %dx%d", w, h)
	}
}
