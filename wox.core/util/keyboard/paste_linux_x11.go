//go:build linux

package keyboard

/*
#cgo pkg-config: x11 xtst
#include <X11/Xlib.h>
#include <X11/keysym.h>
#include <X11/extensions/XTest.h>

static int wox_x11_paste(Display *display) {
	KeyCode ctrl = XKeysymToKeycode(display, XK_Control_L);
	KeyCode v = XKeysymToKeycode(display, XK_v);
	int event, error, major, minor;
	if (!ctrl || !v || !XTestQueryExtension(display, &event, &error, &major, &minor)) {
		return 0;
	}
	int ok = XTestFakeKeyEvent(display, ctrl, True, CurrentTime);
	ok &= XTestFakeKeyEvent(display, v, True, CurrentTime);
	ok &= XTestFakeKeyEvent(display, v, False, CurrentTime);
	ok &= XTestFakeKeyEvent(display, ctrl, False, CurrentTime);
	XSync(display, False);
	return ok;
}
*/
import "C"

import "errors"

// SimulateX11PasteAfterFocus uses the current X session without a uinput device.
func SimulateX11PasteAfterFocus(focus func() error) error {
	display := C.XOpenDisplay(nil)
	if display == nil {
		return errors.New("open X11 display for paste")
	}
	defer C.XCloseDisplay(display)
	if err := focus(); err != nil {
		return err
	}
	if C.wox_x11_paste(display) == 0 {
		return errors.New("send X11 paste with XTest")
	}
	return nil
}
