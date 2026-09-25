// Copyright 2021 The golang.design Initiative Authors.
// All rights reserved. Use of this source code is governed
// by a MIT license that can be found in the LICENSE file.
//
// Written by Changkun Ou <changkun.de>

//go:build linux

#include <stdint.h>
#include <stdio.h>
#include <X11/Xlib.h>
#include <X11/Xutil.h>
#include <sys/select.h>
#include <sys/time.h>
#include <unistd.h>

extern void hotkeyDown(uintptr_t hkhandle);
extern void hotkeyUp(uintptr_t hkhandle);

int displayTest() {
	Display* d = NULL;
	for (int i = 0; i < 42; i++) {
		d = XOpenDisplay(0);
		if (d == NULL) continue;
		break;
	}
	if (d == NULL) {
		return -1;
	}
	XCloseDisplay(d);
	return 0;
}

// waitHotkey blocks until the hotkey is triggered or the cancelled flag is set.
int waitHotkey(uintptr_t hkhandle, unsigned int mod, int key, uint8_t* cancelled) {
	Display* d = NULL;
	for (int i = 0; i < 42; i++) {
		d = XOpenDisplay(0);
		if (d == NULL) continue;
		break;
	}
	if (d == NULL) {
		return -1;
	}

	int keycode = XKeysymToKeycode(d, key);
	XGrabKey(d, keycode, mod, DefaultRootWindow(d), False, GrabModeAsync, GrabModeAsync);
	XSelectInput(d, DefaultRootWindow(d), KeyPressMask);

	int x11_fd = ConnectionNumber(d);
	fd_set in_fds;
	struct timeval tv;

	while(1) {
		if (cancelled != NULL && *cancelled) {
			break;
		}

		// Process all pending events
		while (XPending(d) > 0) {
			XEvent ev;
			XNextEvent(d, &ev);
			switch(ev.type) {
			case KeyPress:
				hotkeyDown(hkhandle);
				break;
			case KeyRelease:
				hotkeyUp(hkhandle);
				break;
			}
		}

		if (cancelled != NULL && *cancelled) {
			break;
		}

		FD_ZERO(&in_fds);
		FD_SET(x11_fd, &in_fds);
		tv.tv_sec = 0;
		tv.tv_usec = 50000; // 50ms timeout

		select(x11_fd + 1, &in_fds, NULL, NULL, &tv);
	}

	XUngrabKey(d, keycode, mod, DefaultRootWindow(d));
	XCloseDisplay(d);
	return 0;
}