//go:build wox_ui_smoke && linux

package smoke

/*
#cgo pkg-config: x11 xtst
#include <stdlib.h>
#include <X11/Xlib.h>
#include <X11/extensions/XTest.h>

static int smoke_x11_chord(char **names, int count) {
	Display *display = XOpenDisplay(NULL);
	if (!display) {
		return 0;
	}
	KeyCode keys[8];
	for (int i = 0; i < count; i++) {
		keys[i] = XKeysymToKeycode(display, XStringToKeysym(names[i]));
		if (!keys[i]) {
			XCloseDisplay(display);
			return 0;
		}
	}
	int ok = 1;
	for (int i = 0; i < count; i++) {
		ok &= XTestFakeKeyEvent(display, keys[i], True, CurrentTime);
	}
	for (int i = count - 1; i >= 0; i--) {
		ok &= XTestFakeKeyEvent(display, keys[i], False, CurrentTime);
	}
	XSync(display, False);
	XCloseDisplay(display);
	return ok;
}
*/
import "C"

import (
	"fmt"
	"unsafe"
)

// SendNativeKeyChord drives X11 keysyms into the focused window.
func SendNativeKeyChord(keys ...string) error {
	if len(keys) < 1 || len(keys) > 8 {
		return fmt.Errorf("invalid X11 chord %v", keys)
	}
	names := make([]*C.char, len(keys))
	for i, key := range keys {
		names[i] = C.CString(key)
		defer C.free(unsafe.Pointer(names[i]))
	}
	if C.smoke_x11_chord(&names[0], C.int(len(names))) == 0 {
		return fmt.Errorf("send X11 chord %v", keys)
	}
	return nil
}
