//go:build linux

package screenshot

import (
	"image"
	"wox/util"
)

// linuxScreenshotObjectSelection selects a backend with a verifiable global coordinate space.
// Hyprland supplies window rectangles through its IPC. Other Wayland sessions keep the
// display fallback and a manual region until their compositor can do the same.
func linuxScreenshotObjectSelection(windows []linuxScreenshotWindow, bounds Rect, source image.Image) ([]Rect, screenshotObjectQuery) {
	if util.IsHyprlandSession() {
		return linuxHyprlandScreenshotObjectSelection(windows, bounds, source)
	}
	if util.IsLinuxWaylandSession() {
		return nil, nil
	}
	return linuxX11ScreenshotObjectSelection(windows, bounds, source)
}
