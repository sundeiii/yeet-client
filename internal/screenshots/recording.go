package screenshots

import (
	"bufio"
	"compress/lzw"
	"errors"
	"image"
	"image/color/palette"
	"io"
	"math"
	"sync"
	"time"

	"golang.org/x/image/draw"
)

// Screen recordings: an area of the screen saved as an MP4 video or an
// animated GIF while it's being recorded. Only Windows can record for now.

// RecordingFormat is the kind of file a recording is saved as.
type RecordingFormat string

const (
	RecordingMP4 RecordingFormat = "mp4"
	RecordingGIF RecordingFormat = "gif"
)

// ErrRecordingNotSupported means this system can't record the screen.
var ErrRecordingNotSupported = errors.New("screen recording isn't supported here")

// Recordings stop by themselves after this long, so a forgotten one doesn't
// fill the disk. GIFs get big much faster than videos.
const (
	maxVideoLength = 30 * time.Minute
	maxGifLength   = 2 * time.Minute
)

// Frames per second. GIFs can't show much more anyway, and stay smaller.
const (
	videoFps = 30
	gifFps   = 15
)

// GIFs are made smaller than this width, which keeps them a sensible size.
const maxGifWidth = 960

// Recording is a recording in progress.
type Recording struct {
	Format  RecordingFormat
	Path    string // where the file is written
	Started time.Time

	stop     chan struct{}
	stopOnce sync.Once
	done     chan struct{}
	err      error
}

func newRecording(format RecordingFormat, path string) *Recording {
	return &Recording{
		Format:  format,
		Path:    path,
		Started: time.Now(),
		stop:    make(chan struct{}),
		done:    make(chan struct{}),
	}
}

// Stop ends the recording and waits until the file is complete.
func (r *Recording) Stop() error {
	r.stopOnce.Do(func() { close(r.stop) })
	<-r.done
	return r.err
}

// Done is closed once the recording ended, whether it was stopped, reached
// its time limit or failed.
func (r *Recording) Done() <-chan struct{} { return r.done }

// Err is why the recording failed, once Done is closed.
func (r *Recording) Err() error { return r.err }

// Elapsed is how long it has been recording.
func (r *Recording) Elapsed() time.Duration { return time.Since(r.Started) }

func maxRecordingLength(format RecordingFormat) time.Duration {
	if format == RecordingGIF {
		return maxGifLength
	}
	return maxVideoLength
}

// ---- Animated GIFs ------------------------------------------------------

// gifWriter writes an animated GIF frame by frame, so a long recording
// doesn't have to fit in memory. Frames only store the part that changed
// since the one before, which keeps screen recordings small.
type gifWriter struct {
	out           *bufio.Writer
	width, height int

	scaled   *image.RGBA // the frame at the GIF's size (BGRA)
	previous []uint8     // the palette indexes of the last frame
	current  []uint8

	// A frame is written once the next one arrives, when its delay is known
	pending      *gifFrame
	writtenDelay int // centiseconds written so far
}

type gifFrame struct {
	rect image.Rectangle
	pix  []uint8
}

// gifSize is the size of a GIF for an area of the screen.
func gifSize(width, height int) (int, int) {
	if width > maxGifWidth {
		height = int(math.Round(float64(height) * maxGifWidth / float64(width)))
		width = maxGifWidth
	}
	return max(width, 1), max(height, 1)
}

func newGifWriter(out io.Writer, width, height int) (*gifWriter, error) {
	g := &gifWriter{
		out:      bufio.NewWriterSize(out, 64<<10),
		width:    width,
		height:   height,
		scaled:   image.NewRGBA(image.Rect(0, 0, width, height)),
		previous: make([]uint8, width*height),
		current:  make([]uint8, width*height),
	}
	w := g.out
	w.WriteString("GIF89a")
	writeUint16(w, width)
	writeUint16(w, height)
	// A global color table of 256 colors
	w.Write([]byte{0xF7, 0, 0})
	for _, c := range palette.Plan9 {
		r, gr, b, _ := c.RGBA()
		w.Write([]byte{byte(r >> 8), byte(gr >> 8), byte(b >> 8)})
	}
	// Loop forever
	w.Write([]byte{0x21, 0xFF, 0x0B})
	w.WriteString("NETSCAPE2.0")
	_, err := w.Write([]byte{0x03, 0x01, 0x00, 0x00, 0x00})
	return g, err
}

// addFrame adds a frame of BGRA pixels at the given time since the start.
func (g *gifWriter) addFrame(bgra []byte, width, height int, at time.Duration) error {
	source := &image.RGBA{Pix: bgra, Stride: width * 4, Rect: image.Rect(0, 0, width, height)}
	pix := bgra
	if width != g.width || height != g.height {
		draw.ApproxBiLinear.Scale(g.scaled, g.scaled.Rect, source, source.Rect, draw.Src, nil)
		pix = g.scaled.Pix
	}
	// Ordered dithering: smooth gradients with few colors, and the same
	// picture always gets the same pixels, so still parts stay unchanged
	lookup := gifLookup()
	for y := 0; y < g.height; y++ {
		row := pix[y*g.width*4:]
		threshold := &bayer4[y&3]
		for x := 0; x < g.width; x++ {
			p := row[x*4:]
			offset := threshold[x&3]
			g.current[y*g.width+x] = lookup[dither(p[2], offset)<<10|dither(p[1], offset)<<5|dither(p[0], offset)]
		}
	}

	// Only the part that changed
	changed := image.Rectangle{}
	if g.pending == nil {
		changed = image.Rect(0, 0, g.width, g.height)
	} else {
		minX, minY, maxX, maxY := g.width, g.height, -1, -1
		for y := 0; y < g.height; y++ {
			row := y * g.width
			for x := 0; x < g.width; x++ {
				if g.current[row+x] != g.previous[row+x] {
					minX, maxX = min(minX, x), max(maxX, x)
					minY, maxY = min(minY, y), max(maxY, y)
				}
			}
		}
		if maxX < 0 {
			// Nothing changed: the last frame just stays up longer
			return nil
		}
		changed = image.Rect(minX, minY, maxX+1, maxY+1)
	}

	frame := &gifFrame{rect: changed, pix: make([]uint8, 0, changed.Dx()*changed.Dy())}
	for y := changed.Min.Y; y < changed.Max.Y; y++ {
		frame.pix = append(frame.pix, g.current[y*g.width+changed.Min.X:y*g.width+changed.Max.X]...)
	}
	g.previous, g.current = g.current, g.previous

	if err := g.flushPending(at); err != nil {
		return err
	}
	g.pending = frame
	return nil
}

// flushPending writes the waiting frame, shown until the given time.
func (g *gifWriter) flushPending(until time.Duration) error {
	if g.pending == nil {
		return nil
	}
	// Delays add up to the real time, so the GIF doesn't drift; browsers
	// show delays under 2 centiseconds much slower
	delay := max(int(until.Round(10*time.Millisecond)/(10*time.Millisecond))-g.writtenDelay, 2)
	g.writtenDelay += delay

	w, frame := g.out, g.pending
	g.pending = nil
	w.Write([]byte{0x21, 0xF9, 0x04, 0x04})
	writeUint16(w, min(delay, 0xFFFF))
	w.Write([]byte{0x00, 0x00})
	w.WriteByte(0x2C)
	writeUint16(w, frame.rect.Min.X)
	writeUint16(w, frame.rect.Min.Y)
	writeUint16(w, frame.rect.Dx())
	writeUint16(w, frame.rect.Dy())
	w.Write([]byte{0x00, 0x08})

	blocks := &gifBlocks{out: w}
	compressor := lzw.NewWriter(blocks, lzw.LSB, 8)
	if _, err := compressor.Write(frame.pix); err != nil {
		return err
	}
	if err := compressor.Close(); err != nil {
		return err
	}
	return blocks.close()
}

// finish writes the last frame, shown until the recording ended, and the
// end of the file.
func (g *gifWriter) finish(at time.Duration) error {
	if err := g.flushPending(at); err != nil {
		return err
	}
	g.out.WriteByte(0x3B)
	return g.out.Flush()
}

// gifBlocks splits image data into the GIF's blocks of up to 255 bytes.
type gifBlocks struct {
	out *bufio.Writer
	buf [255]byte
	n   int
}

func (b *gifBlocks) Write(data []byte) (int, error) {
	for i, c := range data {
		b.buf[b.n] = c
		b.n++
		if b.n == len(b.buf) {
			if err := b.flush(); err != nil {
				return i, err
			}
		}
	}
	return len(data), nil
}

func (b *gifBlocks) flush() error {
	if b.n == 0 {
		return nil
	}
	b.out.WriteByte(byte(b.n))
	_, err := b.out.Write(b.buf[:b.n])
	b.n = 0
	return err
}

func (b *gifBlocks) close() error {
	if err := b.flush(); err != nil {
		return err
	}
	return b.out.WriteByte(0)
}

func writeUint16(w *bufio.Writer, v int) {
	w.Write([]byte{byte(v), byte(v >> 8)})
}

// bayer4 are the offsets for ordered dithering, spread around zero.
var bayer4 = func() (table [4][4]int) {
	matrix := [4][4]int{{0, 8, 2, 10}, {12, 4, 14, 6}, {3, 11, 1, 9}, {15, 7, 13, 5}}
	for y := range matrix {
		for x := range matrix[y] {
			table[y][x] = (matrix[y][x]*2 - 15) * 3 / 2
		}
	}
	return table
}()

// dither adds the offset to a channel and makes it 5 bits.
func dither(value uint8, offset int) int {
	return min(max(int(value)+offset, 0), 255) >> 3
}

// gifLookup maps colors (5 bits per channel, red first) to the nearest
// color of the palette, which is much faster than searching it per pixel.
var gifLookup = sync.OnceValue(func() *[1 << 15]uint8 {
	table := new([1 << 15]uint8)
	for i := range table {
		r, g, b := i>>10&31, i>>5&31, i&31
		table[i] = uint8(nearestPlan9(r<<3|r>>2, g<<3|g>>2, b<<3|b>>2))
	}
	return table
})

func nearestPlan9(r, g, b int) int {
	best, bestDistance := 0, math.MaxInt
	for i, c := range palette.Plan9 {
		pr, pg, pb, _ := c.RGBA()
		dr, dg, db := r-int(pr>>8), g-int(pg>>8), b-int(pb>>8)
		// Eyes are most sensitive to green, least to blue
		if distance := 3*dr*dr + 4*dg*dg + 2*db*db; distance < bestDistance {
			best, bestDistance = i, distance
		}
	}
	return best
}
