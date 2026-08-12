//go:build linux

package screenshot

import (
	"time"
	"wox/util"
)

const linuxGnomePortalChooserCloseDelay = 500 * time.Millisecond

func init() {
	registerLinuxWaylandCaptureBackend(linuxWaylandCaptureBackend{
		name:     "gnome-portal",
		priority: 100,
		matches:  util.IsGnomeWayland,
		open:     newLinuxGnomeDesktopCapture,
	})
}

// newLinuxGnomeDesktopCapture always asks GNOME for one monitor. GNOME Wayland does not expose
// global pointer coordinates, and its chooser can outlive the Start response briefly, so this
// backend avoids silent restore and lets the chooser disappear before accepting the first frame.
func newLinuxGnomeDesktopCapture() (linuxDesktopCapture, error) {
	return newLinuxPortalDesktopCapture(linuxGnomePortalCaptureConfig())
}

func linuxGnomePortalCaptureConfig() linuxPortalCaptureConfig {
	return linuxPortalCaptureConfig{
		backend:               "gnome-portal",
		multiple:              false,
		cursorMode:            1,
		initialLatestFrameFor: linuxGnomePortalChooserCloseDelay,
		disableRestore:        true,
	}
}
