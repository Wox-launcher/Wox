//go:build linux

package screenshot

import utilscreen "wox/util/screen"

// linuxHyprlandScreenshotCursorPosition reads compositor-owned logical coordinates.
// Xwayland can report a stale (0, 0) over native Wayland windows, leaving the recording border pass-through.
func linuxHyprlandScreenshotCursorPosition() *Point {
	x, y, err := utilscreen.GetHyprlandCursorPosition()
	if err != nil {
		return nil
	}
	return &Point{X: float32(x), Y: float32(y)}
}
