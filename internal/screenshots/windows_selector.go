//go:build windows

package screenshots

import (
	"errors"
	"fmt"
	"github.com/sundeiii/yeet-client/internal/i18n"
	"image"
	"runtime"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

var ErrAreaSelectionCancelled = errors.New("area selection cancelled")

// The selector freezes the screen: it takes a screenshot first and shows it
// dimmed, with the selected part at full brightness. What you see while
// selecting is exactly what gets uploaded, and menus or tooltips that close
// once the selector takes focus are still in the picture.
//
// Controls: drag to select, or click to take the window (or screen) under
// the cursor; hold Shift for a square, hold Space while dragging to move the
// selection, Escape or right-click to cancel.

const (
	selectorWindowClass = "PuushAreaSelector"

	// Brightness of the area outside the selection, out of 256
	selectorDim = 110

	magnifierSource = 15 // pixels around the cursor
	magnifierZoom   = 8
	magnifierOffset = 24
)

var (
	selectorClassOnce sync.Once
	selectorClassErr  error
	selectorStates    sync.Map

	selectorWndProc = windows.NewCallback(selectorWindowProc)

	selectionColor = colorRef(140, 198, 62) // puush green
)

// areaSelection is the result of the selector: the chosen area in screen
// coordinates and the frozen screenshot (positioned in screen coordinates)
// it was chosen from.
type areaSelection struct {
	area image.Rectangle
	shot *image.NRGBA
}

// Image returns the selected part of the frozen screenshot.
func (s areaSelection) Image() image.Image {
	return s.shot.SubImage(s.area)
}

type selectionState struct {
	hwnd uintptr

	virtualX int
	virtualY int
	width    int
	height   int

	frozen *frozenScreen

	// Windows and screens that a click selects, frontmost first
	targets     []rect
	hover       rect
	hasHover    bool
	clickTarget rect
	hasClick    bool

	cursor   point
	dragging bool
	moving   bool
	start    point
	current  point
	moveFrom point

	result    rect
	cancelled bool
	done      bool
}

// frozenScreen holds the screenshot in the device contexts the selector
// paints from, plus a back buffer so repainting doesn't flicker.
type frozenScreen struct {
	width, height int

	brightDC, dimDC, backDC    uintptr
	brightBmp, dimBmp, backBmp uintptr
	oldBright, oldDim, oldBack uintptr

	font         uintptr
	labelPadding int
	labelBrush   uintptr
	selectionPen uintptr
	guidePen     uintptr
	whitePen     uintptr
}

func newFrozenScreen(shot *image.NRGBA) (*frozenScreen, error) {
	width, height := shot.Rect.Dx(), shot.Rect.Dy()

	screenDC, err := getDC(0)
	if err != nil {
		return nil, err
	}
	defer releaseDC(0, screenDC)

	f := &frozenScreen{width: width, height: height}
	fail := func(err error) (*frozenScreen, error) {
		f.close()
		return nil, err
	}

	var brightBits, dimBits []byte
	if f.brightBmp, brightBits, err = createDIBSection(screenDC, width, height); err != nil {
		return fail(err)
	}
	if f.dimBmp, dimBits, err = createDIBSection(screenDC, width, height); err != nil {
		return fail(err)
	}
	if f.backBmp, err = createCompatibleBitmap(screenDC, width, height); err != nil {
		return fail(err)
	}

	// The screenshot is RGBA; bitmaps want BGRA
	pix := shot.Pix
	for i := 0; i+3 < len(pix) && i+3 < len(brightBits); i += 4 {
		r, g, b := pix[i], pix[i+1], pix[i+2]
		brightBits[i], brightBits[i+1], brightBits[i+2], brightBits[i+3] = b, g, r, 0xff
		dimBits[i] = byte(uint16(b) * selectorDim >> 8)
		dimBits[i+1] = byte(uint16(g) * selectorDim >> 8)
		dimBits[i+2] = byte(uint16(r) * selectorDim >> 8)
		dimBits[i+3] = 0xff
	}

	for _, dc := range []*uintptr{&f.brightDC, &f.dimDC, &f.backDC} {
		if *dc, err = createCompatibleDC(screenDC); err != nil {
			return fail(err)
		}
	}
	if f.oldBright, err = selectObject(f.brightDC, f.brightBmp); err != nil {
		return fail(err)
	}
	if f.oldDim, err = selectObject(f.dimDC, f.dimBmp); err != nil {
		return fail(err)
	}
	if f.oldBack, err = selectObject(f.backDC, f.backBmp); err != nil {
		return fail(err)
	}

	// Text scales with the display, like the rest of Windows
	dpi := getDeviceCaps(screenDC, logPixelsY)
	if dpi <= 0 {
		dpi = 96
	}
	f.font = createFont(13*dpi/96, 600, "Segoe UI")
	f.labelPadding = 5 * dpi / 96

	f.labelBrush, _ = createSolidBrush(colorRef(24, 24, 24))
	f.selectionPen, _ = createPen(psSolid, 1, selectionColor)
	f.guidePen, _ = createPen(psDot, 1, colorRef(200, 200, 200))
	f.whitePen, _ = createPen(psSolid, 1, colorRef(255, 255, 255))
	return f, nil
}

func (f *frozenScreen) close() {
	if f.oldBright != 0 {
		selectObject(f.brightDC, f.oldBright)
	}
	if f.oldDim != 0 {
		selectObject(f.dimDC, f.oldDim)
	}
	if f.oldBack != 0 {
		selectObject(f.backDC, f.oldBack)
	}
	for _, dc := range []uintptr{f.brightDC, f.dimDC, f.backDC} {
		if dc != 0 {
			deleteDC(dc)
		}
	}
	for _, obj := range []uintptr{
		f.brightBmp, f.dimBmp, f.backBmp, f.font,
		f.labelBrush, f.selectionPen, f.guidePen, f.whitePen,
	} {
		if obj != 0 {
			deleteObject(obj)
		}
	}
}

func selectArea() (areaSelection, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := registerSelectorClass(); err != nil {
		return areaSelection{}, err
	}

	virtualX := getSystemMetrics(smXVirtualScreen)
	virtualY := getSystemMetrics(smYVirtualScreen)
	width := getSystemMetrics(smCXVirtualScreen)
	height := getSystemMetrics(smCYVirtualScreen)

	if width <= 0 || height <= 0 {
		return areaSelection{}, errors.New("virtual screen size is invalid")
	}

	shot, err := captureScreenRect(virtualX, virtualY, width, height)
	if err != nil {
		return areaSelection{}, fmt.Errorf("freeze screen: %w", err)
	}
	// Place the image at its screen position, so screen coordinates index it
	shot.Rect = shot.Rect.Add(image.Pt(virtualX, virtualY))

	frozen, err := newFrozenScreen(shot)
	if err != nil {
		return areaSelection{}, err
	}
	defer frozen.close()

	state := &selectionState{
		virtualX: virtualX,
		virtualY: virtualY,
		width:    width,
		height:   height,
		frozen:   frozen,
		// Listed before the selector's own window covers everything
		targets: clickTargets(),
	}
	state.cursor, _ = getCursorPosition()
	state.hover, state.hasHover = targetAt(state.targets, state.cursor)

	hwnd, err := createSelectorWindow(virtualX, virtualY, width, height)
	if err != nil {
		return areaSelection{}, err
	}

	state.hwnd = hwnd
	selectorStates.Store(hwnd, state)
	defer selectorStates.Delete(hwnd)

	showWindow(hwnd)
	updateWindow(hwnd)

	procSetForegroundWindow.Call(hwnd)
	procSetFocus.Call(hwnd)

	var message msg

	for {
		r1, _, e1 := procGetMessageW.Call(
			uintptr(unsafe.Pointer(&message)),
			0,
			0,
			0,
		)

		switch int32(r1) {
		case -1:
			destroyWindow(hwnd)
			return areaSelection{}, syscallErr("GetMessageW", e1)
		case 0:
			if state.cancelled {
				return areaSelection{}, ErrAreaSelectionCancelled
			}

			area := image.Rect(
				int(state.result.Left), int(state.result.Top),
				int(state.result.Right), int(state.result.Bottom),
			).Intersect(shot.Rect)
			if area.Empty() {
				return areaSelection{}, ErrAreaSelectionCancelled
			}

			return areaSelection{area: area, shot: shot}, nil
		default:
			procTranslateMessage.Call(
				uintptr(unsafe.Pointer(&message)),
			)
			procDispatchMessageW.Call(
				uintptr(unsafe.Pointer(&message)),
			)
		}
	}
}

func registerSelectorClass() error {
	selectorClassOnce.Do(func() {
		instance, err := getModuleHandle()
		if err != nil {
			selectorClassErr = err
			return
		}

		cursor, err := loadCursor(idcCross)
		if err != nil {
			selectorClassErr = err
			return
		}

		className, err := windows.UTF16PtrFromString(selectorWindowClass)
		if err != nil {
			selectorClassErr = err
			return
		}

		wc := wndclassex{
			CbSize:        uint32(unsafe.Sizeof(wndclassex{})),
			LpfnWndProc:   selectorWndProc,
			HInstance:     instance,
			HCursor:       cursor,
			HbrBackground: 0,
			LpszClassName: className,
		}

		r1, _, e1 := procRegisterClassExW.Call(
			uintptr(unsafe.Pointer(&wc)),
		)
		if r1 == 0 {
			selectorClassErr = syscallErr("RegisterClassExW", e1)
		}
	})

	return selectorClassErr
}

func createSelectorWindow(
	x, y, width, height int,
) (uintptr, error) {
	instance, err := getModuleHandle()
	if err != nil {
		return 0, err
	}

	className, _ := windows.UTF16PtrFromString(selectorWindowClass)
	windowName, _ := windows.UTF16PtrFromString("")

	r1, _, e1 := procCreateWindowExW.Call(
		uintptr(wsExTopmost|wsExToolWindow),
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(windowName)),
		uintptr(wsPopup),
		uintptr(int32(x)),
		uintptr(int32(y)),
		uintptr(int32(width)),
		uintptr(int32(height)),
		0,
		0,
		instance,
		0,
	)
	if r1 == 0 {
		return 0, syscallErr("CreateWindowExW", e1)
	}
	return r1, nil
}

func selectorWindowProc(
	hwnd uintptr,
	message uint32,
	wParam uintptr,
	lParam uintptr,
) uintptr {
	value, ok := selectorStates.Load(hwnd)
	if !ok {
		return defWindowProc(hwnd, message, wParam, lParam)
	}

	state := value.(*selectionState)

	switch message {
	case wmEraseBkgnd:
		return 1

	case wmKeyDown:
		switch wParam {
		case vkEscape:
			cancelSelection(state)
			return 0
		case vkSpace:
			if state.dragging && !state.moving {
				state.moving = true
				state.moveFrom = state.cursor
			}
			return 0
		case vkShift:
			invalidateRect(hwnd, false)
			return 0
		}

	case wmKeyUp:
		switch wParam {
		case vkSpace:
			state.moving = false
			return 0
		case vkShift:
			invalidateRect(hwnd, false)
			return 0
		}

	case wmRButtonUp:
		cancelSelection(state)
		return 0

	case wmLButtonDown:
		pt, err := getCursorPosition()
		if err != nil {
			cancelSelection(state)
			return 0
		}

		state.dragging = true
		state.start = pt
		state.current = pt
		state.cursor = pt
		state.clickTarget, state.hasClick = targetAt(state.targets, pt)

		procSetCapture.Call(hwnd)
		invalidateRect(hwnd, false)

		return 0

	case wmMouseMove:
		pt, err := getCursorPosition()
		if err != nil {
			return 0
		}

		if state.dragging && state.moving {
			// Space is held: move the whole selection with the mouse
			dx, dy := pt.X-state.moveFrom.X, pt.Y-state.moveFrom.Y
			state.start.X += dx
			state.start.Y += dy
			state.current.X += dx
			state.current.Y += dy
			state.moveFrom = pt
		} else if state.dragging {
			state.current = pt
		} else {
			state.hover, state.hasHover = targetAt(state.targets, pt)
		}
		state.cursor = pt
		invalidateRect(hwnd, false)

		return 0

	case wmLButtonUp:
		if !state.dragging {
			cancelSelection(state)
			return 0
		}

		pt, err := getCursorPosition()
		if err == nil && !state.moving {
			state.current = pt
		}

		procReleaseCapture.Call()
		state.dragging = false

		finalizeSelection(state)
		return 0

	case wmPaint:
		paintSelectorWindow(hwnd, state)
		return 0

	case wmDestroy:
		state.done = true
		selectorStates.Delete(hwnd)
		postQuitMessage(0)
		return 0
	}

	return defWindowProc(hwnd, message, wParam, lParam)
}

// selection is the selected area in screen coordinates, made square while
// Shift is held.
func (state *selectionState) selection() rect {
	end := state.current
	if keyDown(vkShift) {
		dx, dy := end.X-state.start.X, end.Y-state.start.Y
		side := max(abs32(dx), abs32(dy))
		end.X = state.start.X + side*sign32(dx)
		end.Y = state.start.Y + side*sign32(dy)
	}
	return normalizeSelectionRect(state.start, end)
}

func paintSelectorWindow(hwnd uintptr, state *selectionState) {
	var ps paintstruct

	hdc, err := beginPaint(hwnd, &ps)
	if err != nil {
		return
	}
	defer endPaint(hwnd, &ps)

	renderSelector(state)
	f := state.frozen
	bitBlt(hdc, 0, 0, f.width, f.height, f.backDC, 0, 0, srccopy)
}

// renderSelector draws the selector into the back buffer.
func renderSelector(state *selectionState) {
	f := state.frozen
	back := f.backDC

	// Everything starts dimmed...
	bitBlt(back, 0, 0, f.width, f.height, f.dimDC, 0, 0, srccopy)

	cursorX := int(state.cursor.X) - state.virtualX
	cursorY := int(state.cursor.Y) - state.virtualY

	if state.dragging {
		// ...and the selection shows at full brightness, with its size next to it
		sel := state.selection()
		left := int(sel.Left) - state.virtualX
		top := int(sel.Top) - state.virtualY
		width := int(sel.Right - sel.Left)
		height := int(sel.Bottom - sel.Top)

		if width > 0 && height > 0 {
			bitBlt(back, left, top, width, height, f.brightDC, left, top, srccopy)
			drawRectangle(back, f.selectionPen, left-1, top-1, left+width+1, top+height+1)
		}

		label := fmt.Sprintf("%d × %d", width, height)
		_, labelHeight := labelSize(back, f, label)
		labelY := top - labelHeight - 4
		if labelY < 0 {
			labelY = top + 4
		}
		drawLabel(back, f, label, left, labelY)
	} else if state.hasHover {
		// The window under the cursor lights up: a click takes it
		hover := clipRect(state.hover, state.virtualX, state.virtualY, f.width, f.height)
		left, top := int(hover.Left), int(hover.Top)
		width, height := int(hover.Right-hover.Left), int(hover.Bottom-hover.Top)
		if width > 0 && height > 0 {
			bitBlt(back, left, top, width, height, f.brightDC, left, top, srccopy)
			drawRectangle(back, f.selectionPen, left, top, left+width, top+height)
			drawLabel(back, f, i18n.T("%d × %d  ·  click to capture", width, height), left+4, top+4)
		}
	}

	drawMagnifier(back, state, cursorX, cursorY)
}

// drawMagnifier shows the pixels around the cursor enlarged, for picking
// exact edges, with the cursor's position under it.
func drawMagnifier(hdc uintptr, state *selectionState, cursorX, cursorY int) {
	f := state.frozen
	size := magnifierSource * magnifierZoom

	coordinates := fmt.Sprintf("%d, %d", state.cursor.X, state.cursor.Y)
	_, labelHeight := labelSize(hdc, f, coordinates)

	// Next to the cursor, flipped to the other side near the screen's edges
	x := cursorX + magnifierOffset
	if x+size > f.width {
		x = cursorX - magnifierOffset - size
	}
	y := cursorY + magnifierOffset
	if y+size+labelHeight+2 > f.height {
		y = cursorY - magnifierOffset - size - labelHeight - 2
	}

	half := magnifierSource / 2
	setStretchBltMode(hdc, colorOnColor)
	stretchBlt(hdc, x, y, size, size, f.brightDC, cursorX-half, cursorY-half, magnifierSource, magnifierSource)

	// Frame, and a box around the pixel under the cursor
	drawRectangle(hdc, f.whitePen, x-1, y-1, x+size+1, y+size+1)
	center := half * magnifierZoom
	drawRectangle(hdc, f.selectionPen, x+center-1, y+center-1, x+center+magnifierZoom+1, y+center+magnifierZoom+1)

	drawLabel(hdc, f, coordinates, x-1, y+size+2)
}

func drawRectangle(hdc, pen uintptr, left, top, right, bottom int) {
	oldPen, err := selectObject(hdc, pen)
	if err != nil {
		return
	}
	defer selectObject(hdc, oldPen)

	oldBrush, err := selectObject(hdc, getStockObject(hollowBrush))
	if err != nil {
		return
	}
	defer selectObject(hdc, oldBrush)

	procRectangle.Call(hdc, uintptr(int32(left)), uintptr(int32(top)), uintptr(int32(right)), uintptr(int32(bottom)))
}

func labelSize(hdc uintptr, f *frozenScreen, text string) (int, int) {
	oldFont, err := selectObject(hdc, f.font)
	width, height := textExtent(hdc, text)
	if err == nil {
		selectObject(hdc, oldFont)
	}
	return width + 2*f.labelPadding, height + f.labelPadding
}

// drawLabel draws white text on a dark box with its top-left corner at x, y.
func drawLabel(hdc uintptr, f *frozenScreen, text string, x, y int) {
	width, height := labelSize(hdc, f, text)
	box := rect{Left: int32(x), Top: int32(y), Right: int32(x + width), Bottom: int32(y + height)}
	fillRect(hdc, &box, f.labelBrush)

	oldFont, err := selectObject(hdc, f.font)
	setBkMode(hdc, transparent)
	setTextColor(hdc, colorRef(255, 255, 255))
	textOut(hdc, x+f.labelPadding, y+f.labelPadding/2, text)
	if err == nil {
		selectObject(hdc, oldFont)
	}
}

func finalizeSelection(state *selectionState) {
	if state.done {
		return
	}

	selection := state.selection()
	// A click (hardly any drag) takes the window or screen under the cursor
	if selection.Right-selection.Left < 4 && selection.Bottom-selection.Top < 4 && state.hasClick {
		selection = state.clickTarget
	}
	if selection.Right <= selection.Left || selection.Bottom <= selection.Top {
		cancelSelection(state)
		return
	}

	state.result = selection
	state.done = true
	destroyWindow(state.hwnd)
}

func cancelSelection(state *selectionState) {
	if state.done {
		return
	}

	procReleaseCapture.Call()

	state.cancelled = true
	state.done = true
	destroyWindow(state.hwnd)
}

func normalizeSelectionRect(a, b point) rect {
	left := a.X
	right := b.X
	if left > right {
		left, right = right, left
	}

	top := a.Y
	bottom := b.Y
	if top > bottom {
		top, bottom = bottom, top
	}

	return rect{
		Left:   left,
		Top:    top,
		Right:  right,
		Bottom: bottom,
	}
}

func abs32(v int32) int32 {
	if v < 0 {
		return -v
	}
	return v
}

func sign32(v int32) int32 {
	if v < 0 {
		return -1
	}
	return 1
}

// clipRect turns a screen rectangle into selector coordinates, cut to the
// selector's size.
func clipRect(r rect, virtualX, virtualY, width, height int) rect {
	r.Left -= int32(virtualX)
	r.Right -= int32(virtualX)
	r.Top -= int32(virtualY)
	r.Bottom -= int32(virtualY)
	r.Left = max(r.Left, 0)
	r.Top = max(r.Top, 0)
	r.Right = min(r.Right, int32(width))
	r.Bottom = min(r.Bottom, int32(height))
	return r
}
