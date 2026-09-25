/*
 * Portions of this file are derived from work by Martin Lindhe
 * licensed under the MIT License, see
 * https://github.com/martinlindhe/notify
 */

package notifications

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation -framework AppKit -framework UserNotifications

#include <stdbool.h>
#include <stdlib.h>

void puushConfigureNotificationDelegate(void);
bool puushPostNotification(const char *title, const char *subtitle, const char *message,
	const char *soundName, const char *actionURL);
*/
import "C"

import (
	"errors"

	"unsafe"
)

func init() {
	C.puushConfigureNotificationDelegate()
}

func (n *Notification) Push() error {
	title := C.CString(n.Application)
	defer C.free(unsafe.Pointer(title))

	subtitle := n.Title
	message := n.Text
	if message == "" {
		message = subtitle
		subtitle = ""
	}

	cSubtitle := C.CString(subtitle)
	defer C.free(unsafe.Pointer(cSubtitle))
	cMessage := C.CString(message)
	defer C.free(unsafe.Pointer(cMessage))
	soundName := ""
	if n.soundPath != "" {
		soundName = "success.wav"
	}
	cSoundName := C.CString(soundName)
	defer C.free(unsafe.Pointer(cSoundName))
	cActionURL := C.CString(n.actionUrl)
	defer C.free(unsafe.Pointer(cActionURL))

	if !C.puushPostNotification(title, cSubtitle, cMessage, cSoundName, cActionURL) {
		return errors.New("native macOS notification backend unavailable")
	}
	return nil
}
