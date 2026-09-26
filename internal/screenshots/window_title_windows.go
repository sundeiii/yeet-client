//go:build windows

package screenshots

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var procGetWindowTextW = modUser32.NewProc("GetWindowTextW")

// ActiveWindowTitle is the title of the window in front, for screenshot names.
func ActiveWindowTitle() string {
	hwnd, err := getForegroundWindow()
	if err != nil {
		return ""
	}
	var title [256]uint16
	procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&title[0])), uintptr(len(title)))
	return windows.UTF16ToString(title[:])
}
