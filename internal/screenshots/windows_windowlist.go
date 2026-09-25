//go:build windows

package screenshots

import (
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

// The area selector lets you click a window (or a whole screen) instead of
// dragging. It needs the windows on screen, frontmost first, from before the
// selector covers them.

var (
	modDwmapi = windows.NewLazySystemDLL("dwmapi.dll")

	procEnumWindows           = modUser32.NewProc("EnumWindows")
	procIsWindowVisible       = modUser32.NewProc("IsWindowVisible")
	procIsIconic              = modUser32.NewProc("IsIconic")
	procGetClassNameW         = modUser32.NewProc("GetClassNameW")
	procEnumDisplayMonitors   = modUser32.NewProc("EnumDisplayMonitors")
	procGetMonitorInfoW       = modUser32.NewProc("GetMonitorInfoW")
	procDwmGetWindowAttribute = modDwmapi.NewProc("DwmGetWindowAttribute")
)

const (
	dwmwaExtendedFrameBounds = 9
	dwmwaCloaked             = 14
)

type monitorInfo struct {
	CbSize    uint32
	RcMonitor rect
	RcWork    rect
	DwFlags   uint32
}

var (
	listMu       sync.Mutex
	listedRects  []rect
	enumWindowCb = windows.NewCallback(func(hwnd, _ uintptr) uintptr {
		if r, ok := windowBounds(hwnd); ok {
			listedRects = append(listedRects, r)
		}
		return 1 // keep going
	})
	enumMonitorCb = windows.NewCallback(func(monitor, _, _, _ uintptr) uintptr {
		info := monitorInfo{CbSize: uint32(unsafe.Sizeof(monitorInfo{}))}
		if r1, _, _ := procGetMonitorInfoW.Call(monitor, uintptr(unsafe.Pointer(&info))); r1 != 0 {
			listedRects = append(listedRects, info.RcMonitor)
		}
		return 1
	})
)

// windowBounds returns where a window is, if it's one worth clicking: shown,
// not minimized, not hidden by Windows (like suspended apps), and not the
// desktop itself.
func windowBounds(hwnd uintptr) (rect, bool) {
	if visible, _, _ := procIsWindowVisible.Call(hwnd); visible == 0 {
		return rect{}, false
	}
	if minimized, _, _ := procIsIconic.Call(hwnd); minimized != 0 {
		return rect{}, false
	}
	var cloaked uint32
	if hr, _, _ := procDwmGetWindowAttribute.Call(hwnd, dwmwaCloaked, uintptr(unsafe.Pointer(&cloaked)), 4); hr == 0 && cloaked != 0 {
		return rect{}, false
	}
	var className [64]uint16
	procGetClassNameW.Call(hwnd, uintptr(unsafe.Pointer(&className[0])), uintptr(len(className)))
	switch windows.UTF16ToString(className[:]) {
	case "Progman", "WorkerW":
		return rect{}, false
	}

	// Without the invisible resize border and shadow, where Windows can tell
	var bounds rect
	hr, _, _ := procDwmGetWindowAttribute.Call(hwnd, dwmwaExtendedFrameBounds, uintptr(unsafe.Pointer(&bounds)), unsafe.Sizeof(bounds))
	if hr != 0 {
		var err error
		if bounds, err = getWindowRect(hwnd); err != nil {
			return rect{}, false
		}
	}
	if bounds.Right-bounds.Left < 2 || bounds.Bottom-bounds.Top < 2 {
		return rect{}, false
	}
	return bounds, true
}

// clickTargets lists the windows on screen, frontmost first, and then the
// screens themselves for clicks on the desktop.
func clickTargets() []rect {
	listMu.Lock()
	defer listMu.Unlock()
	listedRects = nil
	procEnumWindows.Call(enumWindowCb, 0)
	procEnumDisplayMonitors.Call(0, 0, enumMonitorCb, 0)
	targets := listedRects
	listedRects = nil
	return targets
}

// targetAt returns the frontmost window or screen under a point.
func targetAt(targets []rect, pt point) (rect, bool) {
	for _, target := range targets {
		if pt.X >= target.Left && pt.X < target.Right && pt.Y >= target.Top && pt.Y < target.Bottom {
			return target, true
		}
	}
	return rect{}, false
}
