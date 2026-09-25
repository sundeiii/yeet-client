//go:build windows

package screenshots

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	// Window messages
	wmDestroy     = 0x0002
	wmPaint       = 0x000F
	wmEraseBkgnd  = 0x0014
	wmKeyDown     = 0x0100
	wmMouseMove   = 0x0200
	wmLButtonDown = 0x0201
	wmLButtonUp   = 0x0202
	wmRButtonUp   = 0x0205

	vkEscape = 0x1B

	// Window styles
	wsPopup        = 0x80000000
	wsExTopmost    = 0x00000008
	wsExToolWindow = 0x00000080
	wsExLayered    = 0x00080000

	swShow   = 5
	lwaAlpha = 0x00000002
	idcCross = 32515

	// GDI
	psSolid     = 0
	hollowBrush = 5

	// System metrics
	smXVirtualScreen  = 76
	smYVirtualScreen  = 77
	smCXVirtualScreen = 78
	smCYVirtualScreen = 79

	srccopy      = 0x00CC0020
	captureBlt   = 0x40000000
	biRGB        = 0
	dibRGBColors = 0

	pwRenderFullContent = 0x00000002
)

type point struct {
	X int32
	Y int32
}

type rect struct {
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
}

type msg struct {
	Hwnd     uintptr
	Message  uint32
	WParam   uintptr
	LParam   uintptr
	Time     uint32
	Pt       point
	LPrivate uint32
}

type paintstruct struct {
	Hdc         uintptr
	FErase      int32
	RcPaint     rect
	FRestore    int32
	FIncUpdate  int32
	RgbReserved [32]byte
}

type wndclassex struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     uintptr
	HIcon         uintptr
	HCursor       uintptr
	HbrBackground uintptr
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       uintptr
}

type bitmapInfoHeader struct {
	BiSize          uint32
	BiWidth         int32
	BiHeight        int32
	BiPlanes        uint16
	BiBitCount      uint16
	BiCompression   uint32
	BiSizeImage     uint32
	BiXPelsPerMeter int32
	BiYPelsPerMeter int32
	BiClrUsed       uint32
	BiClrImportant  uint32
}

type rgbQuad struct {
	Blue     byte
	Green    byte
	Red      byte
	Reserved byte
}

type bitmapInfo struct {
	BmiHeader bitmapInfoHeader
	BmiColors [1]rgbQuad
}

var (
	modKernel32 = windows.NewLazySystemDLL("kernel32.dll")
	modUser32   = windows.NewLazySystemDLL("user32.dll")
	modGdi32    = windows.NewLazySystemDLL("gdi32.dll")

	procGetModuleHandleW           = modKernel32.NewProc("GetModuleHandleW")
	procRegisterClassExW           = modUser32.NewProc("RegisterClassExW")
	procCreateWindowExW            = modUser32.NewProc("CreateWindowExW")
	procDefWindowProcW             = modUser32.NewProc("DefWindowProcW")
	procDestroyWindow              = modUser32.NewProc("DestroyWindow")
	procShowWindow                 = modUser32.NewProc("ShowWindow")
	procUpdateWindow               = modUser32.NewProc("UpdateWindow")
	procGetMessageW                = modUser32.NewProc("GetMessageW")
	procTranslateMessage           = modUser32.NewProc("TranslateMessage")
	procDispatchMessageW           = modUser32.NewProc("DispatchMessageW")
	procPostQuitMessage            = modUser32.NewProc("PostQuitMessage")
	procInvalidateRect             = modUser32.NewProc("InvalidateRect")
	procLoadCursorW                = modUser32.NewProc("LoadCursorW")
	procSetCapture                 = modUser32.NewProc("SetCapture")
	procReleaseCapture             = modUser32.NewProc("ReleaseCapture")
	procSetForegroundWindow        = modUser32.NewProc("SetForegroundWindow")
	procSetFocus                   = modUser32.NewProc("SetFocus")
	procSetLayeredWindowAttributes = modUser32.NewProc("SetLayeredWindowAttributes")
	procBeginPaint                 = modUser32.NewProc("BeginPaint")
	procEndPaint                   = modUser32.NewProc("EndPaint")
	procGetClientRect              = modUser32.NewProc("GetClientRect")
	procFillRect                   = modUser32.NewProc("FillRect")
	procGetCursorPos               = modUser32.NewProc("GetCursorPos")
	procGetDC                      = modUser32.NewProc("GetDC")
	procReleaseDC                  = modUser32.NewProc("ReleaseDC")
	procGetSystemMetrics           = modUser32.NewProc("GetSystemMetrics")
	procGetForegroundWindow        = modUser32.NewProc("GetForegroundWindow")
	procGetWindowRect              = modUser32.NewProc("GetWindowRect")
	procGetWindowDC                = modUser32.NewProc("GetWindowDC")
	procPrintWindow                = modUser32.NewProc("PrintWindow")
	procSetProcessDPIAware         = modUser32.NewProc("SetProcessDPIAware")

	procCreateSolidBrush       = modGdi32.NewProc("CreateSolidBrush")
	procCreatePen              = modGdi32.NewProc("CreatePen")
	procRectangle              = modGdi32.NewProc("Rectangle")
	procGetStockObject         = modGdi32.NewProc("GetStockObject")
	procCreateCompatibleDC     = modGdi32.NewProc("CreateCompatibleDC")
	procDeleteDC               = modGdi32.NewProc("DeleteDC")
	procCreateCompatibleBitmap = modGdi32.NewProc("CreateCompatibleBitmap")
	procSelectObject           = modGdi32.NewProc("SelectObject")
	procDeleteObject           = modGdi32.NewProc("DeleteObject")
	procBitBlt                 = modGdi32.NewProc("BitBlt")
	procGetDIBits              = modGdi32.NewProc("GetDIBits")
)

func syscallErr(name string, err error) error {
	if err != nil && err != windows.Errno(0) {
		return fmt.Errorf("%s: %w", name, err)
	}
	return fmt.Errorf("%s failed", name)
}

func getSystemMetrics(index int) int {
	r1, _, _ := procGetSystemMetrics.Call(uintptr(index))
	return int(r1)
}

func getForegroundWindow() (uintptr, error) {
	r1, _, e1 := procGetForegroundWindow.Call()
	if r1 == 0 {
		return 0, syscallErr("GetForegroundWindow", e1)
	}
	return r1, nil
}

func getWindowRect(hwnd uintptr) (rect, error) {
	var r rect
	r1, _, e1 := procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
	if r1 == 0 {
		return rect{}, syscallErr("GetWindowRect", e1)
	}
	return r, nil
}

func getDC(hwnd uintptr) (uintptr, error) {
	r1, _, e1 := procGetDC.Call(hwnd)
	if r1 == 0 {
		return 0, syscallErr("GetDC", e1)
	}
	return r1, nil
}

func getWindowDC(hwnd uintptr) (uintptr, error) {
	r1, _, e1 := procGetWindowDC.Call(hwnd)
	if r1 == 0 {
		return 0, syscallErr("GetWindowDC", e1)
	}
	return r1, nil
}

func releaseDC(hwnd, hdc uintptr) {
	procReleaseDC.Call(hwnd, hdc)
}

func createCompatibleDC(hdc uintptr) (uintptr, error) {
	r1, _, e1 := procCreateCompatibleDC.Call(hdc)
	if r1 == 0 {
		return 0, syscallErr("CreateCompatibleDC", e1)
	}
	return r1, nil
}

func deleteDC(hdc uintptr) {
	procDeleteDC.Call(hdc)
}

func createCompatibleBitmap(hdc uintptr, width, height int) (uintptr, error) {
	r1, _, e1 := procCreateCompatibleBitmap.Call(hdc, uintptr(width), uintptr(height))
	if r1 == 0 {
		return 0, syscallErr("CreateCompatibleBitmap", e1)
	}
	return r1, nil
}

func selectObject(hdc, obj uintptr) (uintptr, error) {
	const invalidHandle = ^uintptr(0)
	r1, _, e1 := procSelectObject.Call(hdc, obj)
	if r1 == 0 || r1 == invalidHandle {
		return 0, syscallErr("SelectObject", e1)
	}
	return r1, nil
}

func deleteObject(obj uintptr) {
	procDeleteObject.Call(obj)
}

func bitBlt(dst uintptr, x, y, width, height int, src uintptr, srcX, srcY int, rop uintptr) error {
	r1, _, e1 := procBitBlt.Call(
		dst, uintptr(x), uintptr(y), uintptr(width), uintptr(height),
		src, uintptr(srcX), uintptr(srcY), rop,
	)
	if r1 == 0 {
		return syscallErr("BitBlt", e1)
	}
	return nil
}

func printWindow(hwnd, hdc uintptr, flags uint32) bool {
	r1, _, _ := procPrintWindow.Call(hwnd, hdc, uintptr(flags))
	return r1 != 0
}

func getModuleHandle() (uintptr, error) {
	r1, _, e1 := procGetModuleHandleW.Call(0)
	if r1 == 0 {
		return 0, syscallErr("GetModuleHandleW", e1)
	}
	return r1, nil
}

func loadCursor(cursorID uintptr) (uintptr, error) {
	r1, _, e1 := procLoadCursorW.Call(0, cursorID)
	if r1 == 0 {
		return 0, syscallErr("LoadCursorW", e1)
	}
	return r1, nil
}

func beginPaint(hwnd uintptr, ps *paintstruct) (uintptr, error) {
	r1, _, e1 := procBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(ps)))
	if r1 == 0 {
		return 0, syscallErr("BeginPaint", e1)
	}
	return r1, nil
}

func endPaint(hwnd uintptr, ps *paintstruct) {
	procEndPaint.Call(hwnd, uintptr(unsafe.Pointer(ps)))
}

func getClientRect(hwnd uintptr) (rect, error) {
	var r rect
	r1, _, e1 := procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
	if r1 == 0 {
		return rect{}, syscallErr("GetClientRect", e1)
	}
	return r, nil
}

func fillRect(hdc uintptr, r *rect, brush uintptr) error {
	r1, _, e1 := procFillRect.Call(hdc, uintptr(unsafe.Pointer(r)), brush)
	if r1 == 0 {
		return syscallErr("FillRect", e1)
	}
	return nil
}

func createSolidBrush(color uint32) (uintptr, error) {
	r1, _, e1 := procCreateSolidBrush.Call(uintptr(color))
	if r1 == 0 {
		return 0, syscallErr("CreateSolidBrush", e1)
	}
	return r1, nil
}

func createPen(style, width int, color uint32) (uintptr, error) {
	r1, _, e1 := procCreatePen.Call(uintptr(style), uintptr(width), uintptr(color))
	if r1 == 0 {
		return 0, syscallErr("CreatePen", e1)
	}
	return r1, nil
}

func getStockObject(index int) uintptr {
	r1, _, _ := procGetStockObject.Call(uintptr(index))
	return r1
}

func getCursorPosition() (point, error) {
	var pt point
	r1, _, e1 := procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	if r1 == 0 {
		return point{}, syscallErr("GetCursorPos", e1)
	}
	return pt, nil
}

func showWindow(hwnd uintptr) {
	procShowWindow.Call(hwnd, swShow)
}

func updateWindow(hwnd uintptr) {
	procUpdateWindow.Call(hwnd)
}

func invalidateRect(hwnd uintptr, erase bool) {
	var eraseFlag uintptr
	if erase {
		eraseFlag = 1
	}
	procInvalidateRect.Call(hwnd, 0, eraseFlag)
}

func destroyWindow(hwnd uintptr) error {
	r1, _, e1 := procDestroyWindow.Call(hwnd)
	if r1 == 0 {
		return syscallErr("DestroyWindow", e1)
	}
	return nil
}

func postQuitMessage(code int32) {
	procPostQuitMessage.Call(uintptr(code))
}

func defWindowProc(hwnd uintptr, message uint32, wParam uintptr, lParam uintptr) uintptr {
	r1, _, _ := procDefWindowProcW.Call(hwnd, uintptr(message), wParam, lParam)
	return r1
}

func colorRef(r, g, b byte) uint32 {
	return uint32(r) | uint32(g)<<8 | uint32(b)<<16
}

func init() {
	procSetProcessDPIAware.Call()
}
