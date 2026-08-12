//go:build linux

package screenshot

import (
	"fmt"
	"wox/util"
	utilscreen "wox/util/screen"
)

const linuxHyprlandPortalRestoreTokenFile = "screenshot-portal-restore-token-hyprland"

func init() {
	registerLinuxWaylandCaptureBackend(linuxWaylandCaptureBackend{
		name:     "hyprland-portal",
		priority: 100,
		matches:  util.IsHyprlandSession,
		open:     newLinuxHyprlandDesktopCapture,
	})
}

// newLinuxHyprlandDesktopCapture keeps compositor-driven monitor matching isolated from other portals.
func newLinuxHyprlandDesktopCapture() (linuxDesktopCapture, error) {
	return newLinuxPortalDesktopCapture(linuxHyprlandPortalCaptureConfig())
}

func linuxHyprlandPortalCaptureConfig() linuxPortalCaptureConfig {
	return linuxPortalCaptureConfig{
		backend:               "hyprland-portal",
		multiple:              true,
		cursorMode:            1,
		screenSpecificRestore: true,
		restoreTokenFile:      linuxHyprlandPortalRestoreTokenFile,
		displayGeometries:     linuxHyprlandPortalDisplayGeometries,
	}
}

func linuxHyprlandPortalDisplayGeometries() []linuxPortalDisplayGeometry {
	monitors, err := utilscreen.GetHyprlandMonitors()
	if err != nil {
		util.GetLogger().Warn(util.NewTraceContext(), fmt.Sprintf("[screenshot] failed to query Hyprland monitor geometry, using GTK fallback: %v", err))
		return nil
	}
	displays := make([]linuxPortalDisplayGeometry, 0, len(monitors))
	for _, monitor := range monitors {
		logical := monitor.LogicalSize()
		pixels := monitor.PixelSize()
		displays = append(displays, linuxPortalDisplayGeometry{
			Name:        monitor.Name,
			Logical:     utilscreen.Rect{X: logical.X, Y: logical.Y, Width: logical.Width, Height: logical.Height},
			PixelWidth:  pixels.Width,
			PixelHeight: pixels.Height,
		})
	}
	return displays
}
