package notifications

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var procPlaySoundW = windows.NewLazySystemDLL("winmm.dll").NewProc("PlaySoundW")

const (
	sndAsync    = 0x0001
	sndFilename = 0x00020000
)

func playFile(path string) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return
	}
	procPlaySoundW.Call(uintptr(unsafe.Pointer(name)), 0, sndFilename|sndAsync)
}
