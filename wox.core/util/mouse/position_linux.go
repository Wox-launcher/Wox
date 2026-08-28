//go:build linux

package mouse

import (
	"sync"

	"wox/util"
)

type observedPointer struct {
	mu     sync.Mutex
	window uintptr
	point  Point
	inside bool
}

var linuxObserved observedPointer

// ObserveWindowPointer records the last pointer location over a Wox window.
// Wayland compositors other than Hyprland often cannot expose a global cursor,
// so tooltip tracking falls back to this observation.
func ObserveWindowPointer(window uintptr, point Point, inside bool) {
	linuxObserved.mu.Lock()
	defer linuxObserved.mu.Unlock()
	if !inside {
		if linuxObserved.window == window {
			linuxObserved.inside = false
		}
		return
	}
	linuxObserved.window = window
	linuxObserved.point = point
	linuxObserved.inside = true
}

func observedWindowPointer() (Point, bool) {
	linuxObserved.mu.Lock()
	defer linuxObserved.mu.Unlock()
	if !linuxObserved.inside {
		return Point{}, false
	}
	return linuxObserved.point, true
}

// CurrentPosition returns the pointer in overlay logical desktop coordinates.
// Hyprland and X11 can read a global cursor. Other Wayland sessions use the
// last pointer event observed on a Wox window.
func CurrentPosition() (Point, bool) {
	if point, ok := compositorPointer(); ok {
		return point, true
	}
	return observedWindowPointer()
}

func compositorPointer() (Point, bool) {
	if util.IsHyprlandSession() {
		return hyprlandPointer()
	}
	if !util.IsLinuxWaylandSession() {
		return x11Pointer()
	}
	return Point{}, false
}
