//go:build linux

package mouse

import "wox/util/screen"

func hyprlandPointer() (Point, bool) {
	x, y, err := screen.GetHyprlandCursorPosition()
	if err != nil {
		return Point{}, false
	}
	return Point{X: float64(x), Y: float64(y)}, true
}
