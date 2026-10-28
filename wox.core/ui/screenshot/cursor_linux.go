//go:build linux

package screenshot

import "wox/util"

// linuxScreenshotCursorPosition converts the desktop pointer into capture-local logical coordinates.
func linuxScreenshotCursorPosition(bounds Rect) *Point {
	var point *Point
	if util.IsHyprlandSession() {
		point = linuxHyprlandScreenshotCursorPosition()
	} else {
		point = linuxX11ScreenshotCursorPosition()
	}
	if point == nil {
		return nil
	}
	return &Point{X: point.X - bounds.X, Y: point.Y - bounds.Y}
}
