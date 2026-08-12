//go:build linux

package screenshot

import (
	"fmt"
	"image"
	"sync"
	"wox/util"
)

type linuxDesktopCapture interface {
	capture() (image.Image, error)
	logicalBounds() Rect
	close()
}

// linuxWaylandCaptureBackend describes one compositor-specific capture implementation.
type linuxWaylandCaptureBackend struct {
	name     string
	priority int
	matches  func() bool
	open     func() (linuxDesktopCapture, error)
}

var linuxWaylandCaptureBackends struct {
	sync.Mutex
	items []linuxWaylandCaptureBackend
}

// registerLinuxWaylandCaptureBackend lets a desktop-specific file add a backend without changing shared dispatch.
func registerLinuxWaylandCaptureBackend(backend linuxWaylandCaptureBackend) {
	if backend.name == "" || backend.matches == nil || backend.open == nil {
		panic("invalid Linux Wayland screenshot backend")
	}
	linuxWaylandCaptureBackends.Lock()
	defer linuxWaylandCaptureBackends.Unlock()
	linuxWaylandCaptureBackends.items = append(linuxWaylandCaptureBackends.items, backend)
}

// newLinuxWaylandDesktopCapture selects the highest-priority matching desktop backend.
func newLinuxWaylandDesktopCapture() (linuxDesktopCapture, error) {
	linuxWaylandCaptureBackends.Lock()
	backends := append([]linuxWaylandCaptureBackend(nil), linuxWaylandCaptureBackends.items...)
	linuxWaylandCaptureBackends.Unlock()

	if backend := linuxMatchingWaylandCaptureBackend(backends); backend != nil {
		util.GetLogger().Info(util.NewTraceContext(), fmt.Sprintf("[screenshot] selected Linux Wayland backend=%s", backend.name))
		return backend.open()
	}
	return nil, fmt.Errorf("no Linux Wayland screenshot backend is registered")
}

func linuxMatchingWaylandCaptureBackend(backends []linuxWaylandCaptureBackend) *linuxWaylandCaptureBackend {
	selected := -1
	for index, backend := range backends {
		if backend.matches() && (selected < 0 || backend.priority > backends[selected].priority) {
			selected = index
		}
	}
	if selected < 0 {
		return nil
	}
	return &backends[selected]
}
