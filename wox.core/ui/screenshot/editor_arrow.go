package screenshot

import "math"

// screenshotEditorArrowMiddle is the on-curve handle, offset from the straight chord's midpoint.
func screenshotEditorArrowMiddle(annotation screenshotEditorAnnotation) Point {
	return Point{X: (annotation.start.X+annotation.end.X)/2 + annotation.arrowBend.X, Y: (annotation.start.Y+annotation.end.Y)/2 + annotation.arrowBend.Y}
}

// screenshotEditorArrowPoint interpolates a quadratic through the two ends and the draggable midpoint.
func screenshotEditorArrowPoint(annotation screenshotEditorAnnotation, t float32) Point {
	bend := 4 * t * (1 - t)
	return Point{
		X: annotation.start.X + (annotation.end.X-annotation.start.X)*t + annotation.arrowBend.X*bend,
		Y: annotation.start.Y + (annotation.end.Y-annotation.start.Y)*t + annotation.arrowBend.Y*bend,
	}
}

// screenshotEditorArrowPoints bounds chord approximation error in logical units; export supplies a pixel-scaled tolerance.
func screenshotEditorArrowPoints(annotation screenshotEditorAnnotation, tolerance float32) []Point {
	if annotation.arrowBend == (Point{}) {
		return []Point{annotation.start, annotation.end}
	}
	bend := math.Hypot(float64(annotation.arrowBend.X), float64(annotation.arrowBend.Y))
	segments := max(2, int(math.Ceil(math.Sqrt(bend/float64(max(tolerance, 0.01))))))
	// An even count includes the draggable midpoint exactly.
	segments += segments % 2
	points := make([]Point, segments+1)
	for index := range points {
		points[index] = screenshotEditorArrowPoint(annotation, float32(index)/float32(segments))
	}
	return points
}

// screenshotEditorArrowGeometry shares the curved shaft and tangent-aligned arrowhead between preview and export.
func screenshotEditorArrowGeometry(annotation screenshotEditorAnnotation, width, tolerance float32) ([]Point, [3]Point) {
	points := screenshotEditorArrowPoints(annotation, tolerance)
	length := float32(0)
	for index := 1; index < len(points); index++ {
		length += float32(math.Hypot(float64(points[index].X-points[index-1].X), float64(points[index].Y-points[index-1].Y)))
	}
	// The quadratic's end tangent is half the chord minus twice the midpoint offset.
	dx := (annotation.end.X-annotation.start.X)/2 - 2*annotation.arrowBend.X
	dy := (annotation.end.Y-annotation.start.Y)/2 - 2*annotation.arrowBend.Y
	if math.Hypot(float64(dx), float64(dy)) < 0.001 {
		dx, dy = annotation.end.X-annotation.start.X, annotation.end.Y-annotation.start.Y
	}
	angle := math.Atan2(float64(dy), float64(dx))
	headLength := min(14*width/screenshotEditorAnnotationStroke, length/2)
	left := Point{X: annotation.end.X - headLength*float32(math.Cos(angle-math.Pi/6)), Y: annotation.end.Y - headLength*float32(math.Sin(angle-math.Pi/6))}
	right := Point{X: annotation.end.X - headLength*float32(math.Cos(angle+math.Pi/6)), Y: annotation.end.Y - headLength*float32(math.Sin(angle+math.Pi/6))}
	base := Point{X: (left.X + right.X) / 2, Y: (left.Y + right.Y) / 2}
	head := [3]Point{annotation.end, left, right}
	if annotation.arrowBend == (Point{}) {
		return []Point{annotation.start, base}, head
	}
	// Stop the rounded shaft inside the head so its cap cannot protrude beyond the tip.
	remaining := length - headLength*float32(math.Cos(math.Pi/6))
	shaft := []Point{annotation.start}
	for index := 1; index < len(points); index++ {
		previous, next := points[index-1], points[index]
		segment := float32(math.Hypot(float64(next.X-previous.X), float64(next.Y-previous.Y)))
		if segment >= remaining && segment > 0 {
			ratio := remaining / segment
			shaft = append(shaft, Point{X: previous.X + (next.X-previous.X)*ratio, Y: previous.Y + (next.Y-previous.Y)*ratio})
			break
		}
		shaft = append(shaft, next)
		remaining -= segment
	}
	return append(shaft, base), head
}

// screenshotEditorArrowDistance follows the visible curve instead of its endpoint chord.
func screenshotEditorArrowDistance(annotation screenshotEditorAnnotation, point Point) float32 {
	points := screenshotEditorArrowPoints(annotation, 0.25)
	distance := float32(math.MaxFloat32)
	for index := 1; index < len(points); index++ {
		distance = min(distance, screenshotEditorDistanceToSegment(point, points[index-1], points[index]))
	}
	return distance
}

// screenshotEditorArrowBounds includes quadratic extrema without clamping negative logical origins.
func screenshotEditorArrowBounds(annotation screenshotEditorAnnotation) Rect {
	left, right := min(annotation.start.X, annotation.end.X), max(annotation.start.X, annotation.end.X)
	top, bottom := min(annotation.start.Y, annotation.end.Y), max(annotation.start.Y, annotation.end.Y)
	for _, axis := range []struct{ start, end, bend float32 }{
		{annotation.start.X, annotation.end.X, annotation.arrowBend.X},
		{annotation.start.Y, annotation.end.Y, annotation.arrowBend.Y},
	} {
		if axis.bend == 0 {
			continue
		}
		t := 0.5 + (axis.end-axis.start)/(8*axis.bend)
		if t > 0 && t < 1 {
			point := screenshotEditorArrowPoint(annotation, t)
			left, right = min(left, point.X), max(right, point.X)
			top, bottom = min(top, point.Y), max(bottom, point.Y)
		}
	}
	return Rect{X: left, Y: top, Width: right - left, Height: bottom - top}
}
