//go:build windows

package screenshots

import (
	"errors"
	"fmt"
	"image"
	"os"
	"runtime"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	procGetCursorInfo            = modUser32.NewProc("GetCursorInfo")
	procGetIconInfo              = modUser32.NewProc("GetIconInfo")
	procDrawIconEx               = modUser32.NewProc("DrawIconEx")
	procPostMessageW             = modUser32.NewProc("PostMessageW")
	procSetWindowDisplayAffinity = modUser32.NewProc("SetWindowDisplayAffinity")
	procGdiFlush                 = modGdi32.NewProc("GdiFlush")
)

const (
	cursorShowing           = 0x1
	diNormal                = 0x3
	wmClose                 = 0x0010
	wmNcHitTest             = 0x0084
	htTransparent           = ^uintptr(0) // -1
	wsExTransparent         = 0x00000020
	wsExNoActivate          = 0x08000000
	lwaColorKey             = 0x00000001
	wdaExcludeFromCapture   = 0x00000011
	swShowNoActivate        = 4
	recordingFrameClassName = "yeetRecordingFrame"
)

type cursorInfo struct {
	Size     uint32
	Flags    uint32
	Cursor   uintptr
	Position point
}

type iconInfo struct {
	Icon     int32
	HotspotX uint32
	HotspotY uint32
	Mask     uintptr
	Color    uintptr
}

// RecordingSupported reports whether the screen can be recorded here.
func RecordingSupported() bool { return true }

// SelectRecordingArea lets the user pick the area to record, like for an
// area screenshot.
func SelectRecordingArea() (image.Rectangle, error) {
	selection, err := selectArea()
	if err != nil {
		return image.Rectangle{}, err
	}
	return selection.area, nil
}

// frameEncoder is where the captured frames go: an MP4 or a GIF.
type frameEncoder interface {
	writeFrame(bgra []byte, width, height int, at time.Duration) error
	finish(at time.Duration) error
}

type mp4Encoder struct{ *mp4Writer }

func (e mp4Encoder) writeFrame(bgra []byte, _, _ int, at time.Duration) error {
	return e.mp4Writer.writeFrame(bgra, at)
}
func (e mp4Encoder) finish(time.Duration) error { return e.mp4Writer.finish() }

type gifEncoder struct {
	*gifWriter
	file *os.File
}

func (e gifEncoder) writeFrame(bgra []byte, width, height int, at time.Duration) error {
	return e.gifWriter.addFrame(bgra, width, height, at)
}

func (e gifEncoder) finish(at time.Duration) error {
	return errors.Join(e.gifWriter.finish(at), e.file.Close())
}

// StartRecording records an area of the screen into a file at path until
// it's stopped.
func StartRecording(area image.Rectangle, format RecordingFormat, path string) (*Recording, error) {
	if area.Empty() {
		return nil, errors.New("the area is empty")
	}
	r := newRecording(format, path)
	ready := make(chan error, 1)
	go r.run(area, ready)
	if err := <-ready; err != nil {
		return nil, err
	}
	return r, nil
}

func (r *Recording) run(area image.Rectangle, ready chan<- error) {
	// GDI and Media Foundation both want one thread
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer close(r.done)

	width, height, fps := area.Dx(), area.Dy(), gifFps
	if r.Format == RecordingMP4 {
		// Videos need an even size, and the encoder has a largest one
		width, height, fps = width&^1, height&^1, videoFps
		if width < 2 || height < 2 {
			ready <- errors.New("the area is too small for a video")
			return
		}
		if width > 4096 || height > 2304 {
			ready <- errors.New("the area is too big for a video; pick a smaller one or record a GIF")
			return
		}
	}

	grabber, err := newFrameGrabber(area.Min, width, height)
	if err != nil {
		ready <- err
		return
	}
	defer grabber.close()

	var encoder frameEncoder
	switch r.Format {
	case RecordingGIF:
		file, err := os.Create(r.Path)
		if err != nil {
			ready <- err
			return
		}
		gifWidth, gifHeight := gifSize(width, height)
		writer, err := newGifWriter(file, gifWidth, gifHeight)
		if err != nil {
			file.Close()
			ready <- err
			return
		}
		encoder = gifEncoder{gifWriter: writer, file: file}
	default:
		writer, err := newMP4Writer(r.Path, width, height, fps)
		if err != nil {
			ready <- err
			return
		}
		encoder = mp4Encoder{writer}
	}

	frame := showRecordingFrame(area)
	defer frame.close()
	r.Started = time.Now()
	ready <- nil

	limit := maxRecordingLength(r.Format)
	ticker := time.NewTicker(time.Second / time.Duration(fps))
	defer ticker.Stop()
	for {
		at := time.Since(r.Started)
		if err := grabber.grab(); err != nil {
			r.err = err
			break
		}
		if err := encoder.writeFrame(grabber.pixels, width, height, at); err != nil {
			r.err = err
			break
		}
		if at >= limit {
			break
		}
		select {
		case <-r.stop:
		case <-ticker.C:
			continue
		}
		break
	}
	if err := encoder.finish(time.Since(r.Started)); err != nil && r.err == nil {
		r.err = err
	}
}

// frameGrabber copies an area of the screen, with the mouse pointer, into
// a bitmap whose pixels can be read directly.
type frameGrabber struct {
	origin        image.Point
	width, height int
	screenDC      uintptr
	memDC         uintptr
	bitmap        uintptr
	oldBitmap     uintptr
	pixels        []byte // BGRA, top row first
}

func newFrameGrabber(origin image.Point, width, height int) (*frameGrabber, error) {
	g := &frameGrabber{origin: origin, width: width, height: height}
	var err error
	if g.screenDC, err = getDC(0); err != nil {
		return nil, err
	}
	if g.memDC, err = createCompatibleDC(g.screenDC); err != nil {
		g.close()
		return nil, err
	}
	if g.bitmap, g.pixels, err = createDIBSection(g.screenDC, width, height); err != nil {
		g.close()
		return nil, err
	}
	if g.oldBitmap, err = selectObject(g.memDC, g.bitmap); err != nil {
		g.close()
		return nil, err
	}
	return g, nil
}

func (g *frameGrabber) grab() error {
	// Without CAPTUREBLT, which makes the mouse pointer flicker
	if err := bitBlt(g.memDC, 0, 0, g.width, g.height, g.screenDC, g.origin.X, g.origin.Y, srccopy); err != nil {
		return fmt.Errorf("capture frame: %w", err)
	}
	g.drawCursor()
	procGdiFlush.Call()
	return nil
}

// drawCursor draws the mouse pointer where it is, if it's showing.
func (g *frameGrabber) drawCursor() {
	info := cursorInfo{Size: uint32(unsafe.Sizeof(cursorInfo{}))}
	if r1, _, _ := procGetCursorInfo.Call(uintptr(unsafe.Pointer(&info))); r1 == 0 || info.Flags&cursorShowing == 0 || info.Cursor == 0 {
		return
	}
	var icon iconInfo
	if r1, _, _ := procGetIconInfo.Call(info.Cursor, uintptr(unsafe.Pointer(&icon))); r1 == 0 {
		return
	}
	if icon.Mask != 0 {
		deleteObject(icon.Mask)
	}
	if icon.Color != 0 {
		deleteObject(icon.Color)
	}
	x := int(info.Position.X) - int(icon.HotspotX) - g.origin.X
	y := int(info.Position.Y) - int(icon.HotspotY) - g.origin.Y
	procDrawIconEx.Call(g.memDC, uintptr(int32(x)), uintptr(int32(y)), info.Cursor, 0, 0, 0, 0, diNormal)
}

func (g *frameGrabber) close() {
	if g.oldBitmap != 0 {
		selectObject(g.memDC, g.oldBitmap)
	}
	if g.bitmap != 0 {
		deleteObject(g.bitmap)
	}
	if g.memDC != 0 {
		deleteDC(g.memDC)
	}
	if g.screenDC != 0 {
		releaseDC(0, g.screenDC)
	}
}

// ---- The red frame around the recorded area ----------------------------

// recordingFrame is a red border just outside the recorded area, so it's
// clear what's being recorded. Clicks go through it, and it's left out of
// screenshots and recordings.
type recordingFrame struct {
	hwnd uintptr
	done chan struct{}
}

var (
	recordingFrameOnce  sync.Once
	recordingFrameErr   error
	recordingFrameProc  = windows.NewCallback(recordingFrameWndProc)
	recordingFrameSizes sync.Map // hwnd -> border width
)

func showRecordingFrame(area image.Rectangle) *recordingFrame {
	frame := &recordingFrame{done: make(chan struct{})}
	created := make(chan struct{})
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		defer close(frame.done)
		hwnd, err := createRecordingFrame(area)
		frame.hwnd = hwnd
		close(created)
		if err != nil {
			return
		}
		var message msg
		for {
			r1, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
			if int32(r1) <= 0 {
				return
			}
			procTranslateMessage.Call(uintptr(unsafe.Pointer(&message)))
			procDispatchMessageW.Call(uintptr(unsafe.Pointer(&message)))
		}
	}()
	<-created
	return frame
}

func (f *recordingFrame) close() {
	if f.hwnd != 0 {
		procPostMessageW.Call(f.hwnd, wmClose, 0, 0)
	}
	<-f.done
}

func createRecordingFrame(area image.Rectangle) (uintptr, error) {
	recordingFrameOnce.Do(func() {
		instance, err := getModuleHandle()
		if err != nil {
			recordingFrameErr = err
			return
		}
		className, _ := windows.UTF16PtrFromString(recordingFrameClassName)
		wc := wndclassex{
			CbSize:        uint32(unsafe.Sizeof(wndclassex{})),
			LpfnWndProc:   recordingFrameProc,
			HInstance:     instance,
			LpszClassName: className,
		}
		if r1, _, e1 := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); r1 == 0 {
			recordingFrameErr = syscallErr("RegisterClassExW", e1)
		}
	})
	if recordingFrameErr != nil {
		return 0, recordingFrameErr
	}

	border := 3
	if screenDC, err := getDC(0); err == nil {
		if dpi := getDeviceCaps(screenDC, logPixelsY); dpi > 96 {
			border = 3 * dpi / 96
		}
		releaseDC(0, screenDC)
	}
	outer := area.Inset(-border)
	instance, _ := getModuleHandle()
	className, _ := windows.UTF16PtrFromString(recordingFrameClassName)
	windowName, _ := windows.UTF16PtrFromString("")
	hwnd, _, e1 := procCreateWindowExW.Call(
		uintptr(wsExTopmost|wsExToolWindow|wsExLayered|wsExTransparent|wsExNoActivate),
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(windowName)),
		uintptr(wsPopup),
		uintptr(int32(outer.Min.X)), uintptr(int32(outer.Min.Y)),
		uintptr(int32(outer.Dx())), uintptr(int32(outer.Dy())),
		0, 0, instance, 0,
	)
	if hwnd == 0 {
		return 0, syscallErr("CreateWindowExW", e1)
	}
	recordingFrameSizes.Store(hwnd, border)
	// Magenta inside is see-through
	procSetLayeredWindowAttributes.Call(hwnd, uintptr(colorRef(255, 0, 255)), 0, lwaColorKey)
	// Windows 10 2004 and later leave it out of captures entirely
	procSetWindowDisplayAffinity.Call(hwnd, wdaExcludeFromCapture)
	procShowWindow.Call(hwnd, swShowNoActivate)
	updateWindow(hwnd)
	return hwnd, nil
}

func recordingFrameWndProc(hwnd uintptr, message uint32, wParam, lParam uintptr) uintptr {
	switch message {
	case wmNcHitTest:
		return htTransparent
	case wmPaint:
		var ps paintstruct
		hdc, err := beginPaint(hwnd, &ps)
		if err != nil {
			return 0
		}
		border := 3
		if value, ok := recordingFrameSizes.Load(hwnd); ok {
			border = value.(int)
		}
		client, _ := getClientRect(hwnd)
		red, _ := createSolidBrush(colorRef(230, 40, 40))
		clear, _ := createSolidBrush(colorRef(255, 0, 255))
		fillRect(hdc, &client, red)
		inner := rect{Left: client.Left + int32(border), Top: client.Top + int32(border), Right: client.Right - int32(border), Bottom: client.Bottom - int32(border)}
		fillRect(hdc, &inner, clear)
		deleteObject(red)
		deleteObject(clear)
		endPaint(hwnd, &ps)
		return 0
	case wmDestroy:
		recordingFrameSizes.Delete(hwnd)
		postQuitMessage(0)
		return 0
	}
	return defWindowProc(hwnd, message, wParam, lParam)
}
