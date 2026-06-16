//go:build linux && cgo

package screen

import (
	"fmt"
	"os"
	"strings"

	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/gtk"
)

/*
#cgo LDFLAGS: -lX11 -lXrandr
#include <stdbool.h>
#include <X11/Xlib.h>
#include <X11/extensions/Xrandr.h>
#include <stdlib.h>

Display* openDisplay() {
    return XOpenDisplay(NULL);
}

void getScreenSize(Display* display, int* width, int* height) {
    Screen* screen = DefaultScreenOfDisplay(display);
    *width = WidthOfScreen(screen);
    *height = HeightOfScreen(screen);
}

bool getMouseMonitor(Display* display, int* x, int* y, int* width, int* height, int* pointerX, int* pointerY) {
    int screen = DefaultScreen(display);
    Window root = RootWindow(display, screen);
    Window rootReturn;
    Window childReturn;
    int rootX;
    int rootY;
    int winX;
    int winY;
    unsigned int mask;

    if (!XQueryPointer(display, root, &rootReturn, &childReturn, &rootX, &rootY, &winX, &winY, &mask)) {
        return false;
    }
    *pointerX = rootX;
    *pointerY = rootY;

    int monitorCount = 0;
    XRRMonitorInfo* monitors = XRRGetMonitors(display, root, true, &monitorCount);
    if (monitors != NULL && monitorCount > 0) {
        int selected = -1;
        int fallback = 0;
        for (int i = 0; i < monitorCount; i++) {
            XRRMonitorInfo monitor = monitors[i];
            if (monitor.primary) {
                fallback = i;
            }
            if (rootX >= monitor.x && rootX < monitor.x + monitor.width &&
                rootY >= monitor.y && rootY < monitor.y + monitor.height) {
                selected = i;
                break;
            }
        }

        if (selected < 0) {
            selected = fallback;
        }

        XRRMonitorInfo monitor = monitors[selected];
        *x = monitor.x;
        *y = monitor.y;
        *width = monitor.width;
        *height = monitor.height;
        XRRFreeMonitors(monitors);
        return true;
    }

    if (monitors != NULL) {
        XRRFreeMonitors(monitors);
    }

    Screen* defaultScreen = DefaultScreenOfDisplay(display);
    *x = 0;
    *y = 0;
    *width = WidthOfScreen(defaultScreen);
    *height = HeightOfScreen(defaultScreen);
    return true;
}

void closeDisplay(Display* display) {
    XCloseDisplay(display);
}
*/
import "C"

func getMouseScreenGtkPointer() (Size, int, int, error) {
	err := gtk.InitCheck(nil)
	if err != nil {
		return Size{}, 0, 0, err
	}

	defaultGDKDisplay, err := gdk.DisplayGetDefault()
	if err != nil {
		return Size{}, 0, 0, err
	}

	seat, err := defaultGDKDisplay.GetDefaultSeat()
	if err != nil {
		return Size{}, 0, 0, err
	}

	pointer, err := seat.GetPointer()
	if err != nil {
		return Size{}, 0, 0, err
	}

	var x, y int
	if err := pointer.GetPosition(nil, &x, &y); err != nil {
		return Size{}, 0, 0, err
	}

	monitor, err := defaultGDKDisplay.GetMonitorAtPoint(x, y)
	if err != nil {
		return Size{}, 0, 0, err
	}

	area := monitor.GetWorkarea()
	return Size{
		X:      int(area.GetX()),
		Y:      int(area.GetY()),
		Width:  int(area.GetWidth()),
		Height: int(area.GetHeight()),
	}, x, y, nil
}

func GetMouseScreenGtk() (Size, error) {
	if size, _, _, err := getMouseScreenGtkPointer(); err == nil {
		return size, nil
	}

	return getPrimaryScreenGtk()
}

// getPrimaryScreenGtk returns the GTK primary monitor workarea without querying the global pointer.
func getPrimaryScreenGtk() (Size, error) {
	err := gtk.InitCheck(nil)
	if err != nil {
		return Size{}, err
	}

	defaultGDKDisplay, err := gdk.DisplayGetDefault()
	if err != nil {
		return Size{}, err
	}

	monitor, err := defaultGDKDisplay.GetPrimaryMonitor()
	if err != nil {
		return Size{}, err
	}
	area := monitor.GetWorkarea()
	return Size{
		X:      int(area.GetX()),
		Y:      int(area.GetY()),
		Width:  int(area.GetWidth()),
		Height: int(area.GetHeight()),
	}, nil
}

func GetMouseScreenX11() (Size, error) {
	size, _, _, err := getMouseScreenX11WithPointer()
	return size, err
}

func getMouseScreenX11WithPointer() (Size, int, int, error) {
	display := C.openDisplay()
	if display == nil {
		return Size{}, 0, 0, fmt.Errorf("could not open X11 display")
	}
	defer C.closeDisplay(display)

	var x, y, width, height, pointerX, pointerY C.int
	if !bool(C.getMouseMonitor(display, &x, &y, &width, &height, &pointerX, &pointerY)) {
		return Size{}, 0, 0, fmt.Errorf("could not get X11 mouse monitor")
	}

	return Size{
		X:      int(x),
		Y:      int(y),
		Width:  int(width),
		Height: int(height),
	}, int(pointerX), int(pointerY), nil
}

func GetMouseScreen() Size {
	if isWaylandSession() {
		// Wayland does not expose a trusted global pointer to regular clients.
		// Do not fall back to X11/XRandR while the session is Wayland: DISPLAY may
		// only describe the XWayland compatibility server, whose pointer and monitor
		// state can disagree with the compositor that actually places the window.
		// Return a neutral monitor only for sizing/logging; Flutter skips absolute
		// placement on native Wayland and lets the compositor choose the screen.
		size, err := getPrimaryScreenGtk()
		if err == nil {
			setLastMouseScreenDebug(fmt.Sprintf("source=gtk-wayland-primary screen=%d,%d %dx%d reason=wayland-no-global-pointer", size.X, size.Y, size.Width, size.Height))
			return size
		}
		setLastMouseScreenDebug(fmt.Sprintf("source=wayland-primary-failed err=%v", err))
	} else {
		size, pointerX, pointerY, err := getMouseScreenX11WithPointer()
		if err == nil {
			setLastMouseScreenDebug(fmt.Sprintf("source=x11 pointer=%d,%d screen=%d,%d %dx%d", pointerX, pointerY, size.X, size.Y, size.Width, size.Height))
			return size
		}
		setLastMouseScreenDebug(fmt.Sprintf("source=x11-failed err=%v", err))
	}

	// Give gtk a try, as it considers DPI and scaling of the screen
	size, err := GetMouseScreenGtk()
	if err == nil {
		setLastMouseScreenDebug(fmt.Sprintf("source=gtk-fallback screen=%d,%d %dx%d", size.X, size.Y, size.Width, size.Height))
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
	setLastMouseScreenDebug(fmt.Sprintf("source=x11-size-fallback width=%d height=%d gtkErr=%v", int(width), int(height), err))

	return Size{
		Width:  int(width),
		Height: int(height),
	}
}

func isWaylandSession() bool {
	return strings.EqualFold(os.Getenv("XDG_SESSION_TYPE"), "wayland") || os.Getenv("WAYLAND_DISPLAY") != ""
}

func GetActiveScreen() Size {
	// For Linux, we'll use the mouse screen info
	// Note: Getting the truly active screen in Linux is complex and requires window manager integration
	return GetMouseScreen()
}

func listDisplays() ([]Display, error) {
	err := gtk.InitCheck(nil)
	if err != nil {
		return nil, err
	}

	display, err := gdk.DisplayGetDefault()
	if err != nil {
		return nil, err
	}

	count := display.GetNMonitors()
	displays := make([]Display, 0, count)
	for i := 0; i < count; i++ {
		monitor, monitorErr := display.GetMonitor(i)
		if monitorErr != nil {
			return nil, monitorErr
		}

		geometry := monitor.GetGeometry()
		workarea := monitor.GetWorkarea()
		scale := float64(monitor.GetScaleFactor())
		if scale <= 0 {
			scale = 1
		}

		displays = append(displays, Display{
			ID:   fmt.Sprintf("%d", i),
			Name: fmt.Sprintf("Display %d", i+1),
			Bounds: Rect{
				X:      int(geometry.GetX()),
				Y:      int(geometry.GetY()),
				Width:  int(geometry.GetWidth()),
				Height: int(geometry.GetHeight()),
			},
			WorkArea: Rect{
				X:      int(workarea.GetX()),
				Y:      int(workarea.GetY()),
				Width:  int(workarea.GetWidth()),
				Height: int(workarea.GetHeight()),
			},
			PixelBounds: Rect{
				X:      int(float64(geometry.GetX()) * scale),
				Y:      int(float64(geometry.GetY()) * scale),
				Width:  int(float64(geometry.GetWidth()) * scale),
				Height: int(float64(geometry.GetHeight()) * scale),
			},
			PixelWorkArea: Rect{
				X:      int(float64(workarea.GetX()) * scale),
				Y:      int(float64(workarea.GetY()) * scale),
				Width:  int(float64(workarea.GetWidth()) * scale),
				Height: int(float64(workarea.GetHeight()) * scale),
			},
			Scale:   scale,
			Primary: i == 0,
		})
	}

	return displays, nil
}
