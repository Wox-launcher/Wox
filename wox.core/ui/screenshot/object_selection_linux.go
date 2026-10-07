//go:build linux

package screenshot

import (
	"image"
	"wox/util"
)

// linuxScreenshotObjectSelection selects a backend with a verifiable global coordinate space.
// Wayland uses display fallback and manual regions until its compositor supplies window identities and matching AT-SPI geometry.
func linuxScreenshotObjectSelection(windows []linuxScreenshotWindow, bounds Rect, source image.Image) ([]Rect, screenshotObjectQuery) {
	if util.IsLinuxWaylandSession() {
		return nil, nil
	}
	return linuxX11ScreenshotObjectSelection(windows, bounds, source)
}
