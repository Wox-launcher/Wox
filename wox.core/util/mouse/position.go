package mouse

// Point is a desktop position in the coordinate system expected by overlay
// absolute positioning on the current platform.
type Point struct {
	X float64
	Y float64
}

// logicalDesktopPoint converts physical pixels into overlay DIP coordinates.
func logicalDesktopPoint(physicalX, physicalY int32, scale float64) Point {
	if scale <= 0 {
		scale = 1
	}
	return Point{X: float64(physicalX) / scale, Y: float64(physicalY) / scale}
}
