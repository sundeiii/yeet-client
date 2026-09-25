//go:build windows

package screenshots

import (
	"errors"
	"runtime"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

var ErrAreaSelectionCancelled = errors.New("area selection cancelled")

const (
	selectorWindowClass = "PuushAreaSelector"
	selectorAlpha       = 72
)

var (
	selectorClassOnce sync.Once
	selectorClassErr  error
	selectorStates    sync.Map

	selectorWndProc = windows.NewCallback(selectorWindowProc)
)

type selectionState struct {
	hwnd uintptr

	virtualX int
	virtualY int
	width    int
	height   int

	dragging bool
	start    point
	current  point

	result    rect
	cancelled bool
	done      bool
}

func selectAreaRect() (rect, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := registerSelectorClass(); err != nil {
		return rect{}, err
	}

	virtualX := getSystemMetrics(smXVirtualScreen)
	virtualY := getSystemMetrics(smYVirtualScreen)
	width := getSystemMetrics(smCXVirtualScreen)
	height := getSystemMetrics(smCYVirtualScreen)

	if width <= 0 || height <= 0 {
		return rect{}, errors.New("virtual screen size is invalid")
	}

	state := &selectionState{
		virtualX: virtualX,
		virtualY: virtualY,
		width:    width,
		height:   height,
	}

	hwnd, err := createSelectorWindow(virtualX, virtualY, width, height)
	if err != nil {
		return rect{}, err
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
			return rect{}, syscallErr("GetMessageW", e1)
		case 0:
			if state.cancelled {
				return rect{}, ErrAreaSelectionCancelled
			}

			if state.result.Right <= state.result.Left ||
				state.result.Bottom <= state.result.Top {
				return rect{}, ErrAreaSelectionCancelled
			}

			return state.result, nil
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
		uintptr(wsExTopmost|wsExToolWindow|wsExLayered),
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

	hwnd := r1

	r1, _, e1 = procSetLayeredWindowAttributes.Call(
		hwnd,
		0,
		uintptr(byte(selectorAlpha)),
		uintptr(lwaAlpha),
	)
	if r1 == 0 {
		destroyWindow(hwnd)
		return 0, syscallErr("SetLayeredWindowAttributes", e1)
	}

	return hwnd, nil
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
		if wParam == vkEscape {
			cancelSelection(state)
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

		procSetCapture.Call(hwnd)
		invalidateRect(hwnd, false)

		return 0

	case wmMouseMove:
		if !state.dragging {
			return 0
		}

		pt, err := getCursorPosition()
		if err != nil {
			return 0
		}

		state.current = pt
		invalidateRect(hwnd, false)

		return 0

	case wmLButtonUp:
		if !state.dragging {
			cancelSelection(state)
			return 0
		}

		pt, err := getCursorPosition()
		if err == nil {
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

func paintSelectorWindow(hwnd uintptr, state *selectionState) {
	var ps paintstruct

	hdc, err := beginPaint(hwnd, &ps)
	if err != nil {
		return
	}
	defer endPaint(hwnd, &ps)

	client, err := getClientRect(hwnd)
	if err != nil {
		return
	}

	backgroundBrush, err := createSolidBrush(colorRef(0, 0, 0))
	if err != nil {
		return
	}
	defer deleteObject(backgroundBrush)

	if err := fillRect(hdc, &client, backgroundBrush); err != nil {
		return
	}

	if !state.dragging {
		return
	}

	selection := normalizeSelectionRect(state.start, state.current)
	if selection.Right <= selection.Left || selection.Bottom <= selection.Top {
		return
	}

	left := selection.Left - int32(state.virtualX)
	top := selection.Top - int32(state.virtualY)
	right := selection.Right - int32(state.virtualX)
	bottom := selection.Bottom - int32(state.virtualY)

	pen, err := createPen(psSolid, 2, colorRef(255, 255, 255))
	if err != nil {
		return
	}
	defer deleteObject(pen)

	brush := getStockObject(hollowBrush)
	if brush == 0 {
		return
	}

	oldPen, err := selectObject(hdc, pen)
	if err != nil {
		return
	}
	defer func() {
		selectObject(hdc, oldPen)
	}()

	oldBrush, err := selectObject(hdc, brush)
	if err != nil {
		return
	}
	defer func() {
		selectObject(hdc, oldBrush)
	}()

	procRectangle.Call(
		hdc,
		uintptr(left),
		uintptr(top),
		uintptr(right),
		uintptr(bottom),
	)
}

func finalizeSelection(state *selectionState) {
	if state.done {
		return
	}

	selection := normalizeSelectionRect(state.start, state.current)
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
