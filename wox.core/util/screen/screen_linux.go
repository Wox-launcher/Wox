package screen

import (
	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/gtk"
)

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

func GetMouseScreenGtk() (Size, error) {
	err := gtk.InitCheck(nil)
	if err != nil {
		return Size{}, err
	}

	default_gdk_display, err := gdk.DisplayGetDefault()
	if err != nil {
		return Size{}, err
	}

	monitor, err := default_gdk_display.GetPrimaryMonitor()
	if err != nil {
		return Size{}, err
	}

	area := monitor.GetWorkarea()
	return Size{
		Width:  int(area.GetWidth()),
		Height: int(area.GetHeight()),
	}, nil
}

func GetMouseScreen() Size {
	// Give gtk a try, as it considers DPI and scaling of the screen
	size, err := GetMouseScreenGtk()
	if err == nil {
		return size
	}
	// Fallback to X11
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
