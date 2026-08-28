//go:build linux

package mouse

import "wox/util/screen"

func x11Pointer() (Point, bool) {
	x, y, err := screen.GetX11PointerPosition()
	if err != nil {
		return Point{}, false
	}
	return Point{X: float64(x), Y: float64(y)}, true
}
