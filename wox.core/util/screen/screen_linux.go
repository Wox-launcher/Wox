package screen

/*
#cgo LDFLAGS: -lX11
#include <X11/Xlib.h>
#include <stdlib.h>

Display* openDisplay() {
    return XOpenDisplay(NULL);
}

void getScreenSize(Display* display, int* width, int* height) {
    Screen* screen = DefaultScreenOfDisplay(display);
    *width = WidthOfScreen(screen);
    *height = HeightOfScreen(screen);
}

void closeDisplay(Display* display) {
    XCloseDisplay(display);
}
*/
import "C"

func GetMouseScreen() Size {
	display := C.openDisplay()
	if display == nil {
		panic("Could not open X11 display")
	}
	defer C.closeDisplay(display)

	var width, height C.int
	C.getScreenSize(display, &width, &height)

	return Size{
		Width:  int(width),
		Height: int(height),
	}
}

func GetActiveScreen() Size {
	// For Linux, we'll use the mouse screen info
	// Note: Getting the truly active screen in Linux is complex and requires window manager integration
	return GetMouseScreen()
}
