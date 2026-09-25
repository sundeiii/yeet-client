// Copyright 2021 The golang.design Initiative Authors.
// All rights reserved. Use of this source code is governed
// by a MIT license that can be found in the LICENSE file.
//
// Written by Changkun Ou <changkun.de>

//go:build darwin

#include <stdint.h>
#import <Cocoa/Cocoa.h>
#import <Carbon/Carbon.h>
#import <Dispatch/Dispatch.h>

extern void keydownCallback(uintptr_t id);
extern void keyupCallback(uintptr_t id);

static EventHandlerRef keydownEventHandler;
static EventHandlerRef keyupEventHandler;
static bool eventHandlersInstalled;

static OSStatus
keydownHandler(EventHandlerCallRef nextHandler, EventRef theEvent, void *userData) {
	EventHotKeyID k;
	if (GetEventParameter(theEvent, kEventParamDirectObject, typeEventHotKeyID, NULL, sizeof(k), NULL, &k) != noErr) {
		return eventNotHandledErr;
	}
	keydownCallback((uintptr_t)k.id);
	return noErr;
}

static OSStatus
keyupHandler(EventHandlerCallRef nextHandler, EventRef theEvent, void *userData) {
	EventHotKeyID k;
	if (GetEventParameter(theEvent, kEventParamDirectObject, typeEventHotKeyID, NULL, sizeof(k), NULL, &k) != noErr) {
		return eventNotHandledErr;
	}
	keyupCallback((uintptr_t)k.id);
	return noErr;
}

static OSStatus installEventHandlers(void) {
	if (eventHandlersInstalled) {
		return noErr;
	}

	EventTypeSpec keydownEvent;
	keydownEvent.eventClass = kEventClassKeyboard;
	keydownEvent.eventKind = kEventHotKeyPressed;
	EventTypeSpec keyupEvent;
	keyupEvent.eventClass = kEventClassKeyboard;
	keyupEvent.eventKind = kEventHotKeyReleased;

	OSStatus s = InstallApplicationEventHandler(
		&keydownHandler, 1, &keydownEvent, NULL, &keydownEventHandler
	);
	if (s != noErr) {
		return s;
	}

	s = InstallApplicationEventHandler(
		&keyupHandler, 1, &keyupEvent, NULL, &keyupEventHandler
	);
	if (s != noErr) {
		RemoveEventHandler(keydownEventHandler);
		keydownEventHandler = NULL;
		return s;
	}

	eventHandlersInstalled = true;
	return noErr;
}

static OSStatus registerHotKeyOnMainThread(int mod, int key, uint32_t id, EventHotKeyRef* ref) {
	OSStatus s = installEventHandlers();
	if (s != noErr) {
		return s;
	}

	EventHotKeyID hkid = {.signature = 'ghky', .id = id};
	return RegisterEventHotKey(
		key, mod, hkid, GetApplicationEventTarget(), 0, ref
	);
}

// registerHotKey registers a global system hotkey for callbacks.
int registerHotKey(int mod, int key, uint32_t id, EventHotKeyRef* ref) {
	__block OSStatus s = noErr;
	if ([NSThread isMainThread]) {
		s = registerHotKeyOnMainThread(mod, key, id, ref);
	} else {
		dispatch_sync(dispatch_get_main_queue(), ^{
			s = registerHotKeyOnMainThread(mod, key, id, ref);
		});
	}
	return (int)s;
}

static OSStatus unregisterHotKeyOnMainThread(EventHotKeyRef ref) {
	return UnregisterEventHotKey(ref);
}

int unregisterHotKey(EventHotKeyRef ref) {
	__block OSStatus s = noErr;
	if ([NSThread isMainThread]) {
		s = unregisterHotKeyOnMainThread(ref);
	} else {
		dispatch_sync(dispatch_get_main_queue(), ^{
			s = unregisterHotKeyOnMainThread(ref);
		});
	}
	return (int)s;
}
