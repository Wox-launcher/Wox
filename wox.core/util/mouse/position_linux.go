//go:build linux

package mouse

// CurrentPosition is not implemented on Linux yet. The Linux overlay backend is
// currently a stub, so callers can simply skip pointer-anchored progress UI.
func CurrentPosition() (Point, bool) {
	return Point{}, false
}
