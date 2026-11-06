package graphics

// PolygonBounds returns the conservative envelope of finite vertices. An empty
// or nonfinite polygon has no drawable bounds.
func PolygonBounds(points []Point) Rect {
	if len(points) == 0 {
		return Rect{}
	}
	left, right, top, bottom := points[0].X, points[0].X, points[0].Y, points[0].Y
	for _, p := range points {
		if !finitePoint(p) {
			return Rect{}
		}
		left, right, top, bottom = min(left, p.X), max(right, p.X), min(top, p.Y), max(bottom, p.Y)
	}
	width, height := right-left, bottom-top
	if !finitePoint(Point{X: width, Y: height}) {
		return Rect{}
	}
	return Rect{X: left, Y: top, Width: width, Height: height}
}

// ClipConvexPolygon intersects finite convex polygons of either winding without
// mutating either input. The Sutherland-Hodgman result preserves subject winding;
// disjoint, touching-only and degenerate polygons produce no drawable region.
func ClipConvexPolygon(subject, clip []Point) []Point {
	if len(subject) < 3 || len(clip) < 3 {
		return nil
	}
	for _, polygon := range [][]Point{subject, clip} {
		for _, p := range polygon {
			if !finitePoint(p) {
				return nil
			}
		}
	}
	orientation := polygonArea(clip)
	if orientation == 0 || polygonArea(subject) == 0 {
		return nil
	}
	if orientation > 0 {
		orientation = 1
	} else {
		orientation = -1
	}
	result := append([]Point(nil), subject...)
	for i, a := range clip {
		b := clip[(i+1)%len(clip)]
		if a == b {
			continue
		}
		distance := func(p Point) float64 {
			return orientation * ((float64(b.X)-float64(a.X))*(float64(p.Y)-float64(a.Y)) - (float64(b.Y)-float64(a.Y))*(float64(p.X)-float64(a.X)))
		}
		input := result
		result = make([]Point, 0, len(input)+1)
		previous := input[len(input)-1]
		previousDistance := distance(previous)
		for _, point := range input {
			currentDistance := distance(point)
			if (currentDistance >= 0) != (previousDistance >= 0) {
				weight := previousDistance / (previousDistance - currentDistance)
				intersection := Point{X: float32(float64(previous.X) + (float64(point.X)-float64(previous.X))*weight), Y: float32(float64(previous.Y) + (float64(point.Y)-float64(previous.Y))*weight)}
				result = appendDistinctPoint(result, intersection)
			}
			if currentDistance >= 0 {
				result = appendDistinctPoint(result, point)
			}
			previous, previousDistance = point, currentDistance
		}
		if len(result) > 1 && result[0] == result[len(result)-1] {
			result = result[:len(result)-1]
		}
		if len(result) < 3 {
			return nil
		}
	}
	if polygonArea(result) == 0 {
		return nil
	}
	return result
}

func appendDistinctPoint(points []Point, p Point) []Point {
	if len(points) == 0 || points[len(points)-1] != p {
		return append(points, p)
	}
	return points
}

// polygonArea uses a local origin to avoid cancellation on negative/large desktops.
func polygonArea(points []Point) float64 {
	if len(points) < 3 {
		return 0
	}
	origin := points[0]
	area := float64(0)
	for i := 1; i+1 < len(points); i++ {
		a, b := points[i], points[i+1]
		area += (float64(a.X)-float64(origin.X))*(float64(b.Y)-float64(origin.Y)) - (float64(a.Y)-float64(origin.Y))*(float64(b.X)-float64(origin.X))
	}
	return area / 2
}
