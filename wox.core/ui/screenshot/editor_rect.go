package screenshot

import (
	"image"
	"image/color"
	"math"

	"golang.org/x/image/vector"
)

// A circular corner crosses its diagonal this far from the bounding-box corner per unit radius.
const screenshotEditorRectRadiusHandleRatio = 1 - 1/math.Sqrt2

func screenshotEditorRectCornerRadius(annotation screenshotEditorAnnotation) float32 {
	return min(max(float32(0), annotation.cornerRadius), max(float32(0), min(annotation.rect.Width, annotation.rect.Height)/2))
}

// screenshotEditorRectRadiusHandlePoints keeps controls inside the curved outline, separated even at maximum rounding.
func screenshotEditorRectRadiusHandlePoints(annotation screenshotEditorAnnotation, uiScale float32) [4]Point {
	rect := annotation.rect
	gap := min(16*max(float32(1), uiScale), min(rect.Width, rect.Height)/8)
	inset := gap + screenshotEditorRectCornerRadius(annotation)*screenshotEditorRectRadiusHandleRatio
	return [4]Point{
		{X: rect.X + inset, Y: rect.Y + inset},
		{X: rect.X + rect.Width - inset, Y: rect.Y + inset},
		{X: rect.X + rect.Width - inset, Y: rect.Y + rect.Height - inset},
		{X: rect.X + inset, Y: rect.Y + rect.Height - inset},
	}
}

// drawScreenshotEditorRectRadiusHandles distinguishes rounding controls from the hollow resize handles.
func drawScreenshotEditorRectRadiusHandles(list *DisplayList, annotation screenshotEditorAnnotation, uiScale float32) {
	scale := max(float32(1), uiScale)
	for _, point := range screenshotEditorRectRadiusHandlePoints(annotation, scale) {
		bounds := Rect{X: point.X - 5*scale, Y: point.Y - 5*scale, Width: 10 * scale, Height: 10 * scale}
		list.FillRoundedRect(bounds, 5*scale, screenshotEditorAnnotationDrawColor(annotation))
		list.StrokeRoundedRect(bounds, 5*scale, 2*scale, Color{R: 255, G: 255, B: 255, A: 255})
	}
}

// screenshotEditorRectControlAt uses distance so overlapping hit areas cannot hide resize or rounding controls on small rectangles.
func screenshotEditorRectControlAt(annotation screenshotEditorAnnotation, point Point, uiScale float32) (screenshotEditorHandle, screenshotEditorEditMode, bool) {
	closest := float64(12 * max(float32(1), uiScale))
	handle, mode := screenshotEditorHandleTopLeft, screenshotEditorEditNone
	for index, control := range screenshotEditorRectHandlePoints(annotation.rect) {
		distance := math.Hypot(float64(control.X-point.X), float64(control.Y-point.Y))
		if distance <= closest {
			closest, handle, mode = distance, screenshotEditorHandle(index), screenshotEditorEditResizeAnnotation
		}
	}
	for index, control := range screenshotEditorRectRadiusHandlePoints(annotation, uiScale) {
		distance := math.Hypot(float64(control.X-point.X), float64(control.Y-point.Y))
		if distance < closest {
			closest, handle, mode = distance, screenshotEditorHandle(index*2), screenshotEditorEditRectRadius
		}
	}
	return handle, mode, mode != screenshotEditorEditNone
}

// screenshotEditorDraggedRectRadius projects the drag onto the corner diagonal, preserving off-center grabs without a jump.
func screenshotEditorDraggedRectRadius(annotation screenshotEditorAnnotation, handle screenshotEditorHandle, delta Point) float32 {
	if handle == screenshotEditorHandleTopRight || handle == screenshotEditorHandleBottomRight {
		delta.X = -delta.X
	}
	if handle == screenshotEditorHandleBottomLeft || handle == screenshotEditorHandleBottomRight {
		delta.Y = -delta.Y
	}
	annotation.cornerRadius = screenshotEditorRectCornerRadius(annotation) + (delta.X+delta.Y)/(2*screenshotEditorRectRadiusHandleRatio)
	return screenshotEditorRectCornerRadius(annotation)
}

// screenshotEditorRoundedRectContains excludes the empty corners from annotation selection.
func screenshotEditorRoundedRectContains(rect Rect, radius float32, point Point) bool {
	if !screenshotEditorRectContains(rect, point) {
		return false
	}
	centerX := min(max(point.X, rect.X+radius), rect.X+rect.Width-radius)
	centerY := min(max(point.Y, rect.Y+radius), rect.Y+rect.Height-radius)
	dx, dy := point.X-centerX, point.Y-centerY
	return dx*dx+dy*dy <= radius*radius
}

// drawScreenshotEditorPixelRect matches the preview's inset stroke and maps each axis to the actual capture pixels.
func drawScreenshotEditorPixelRect(target *image.RGBA, clip image.Rectangle, annotation screenshotEditorAnnotation, width, scaleX, scaleY float32, ink color.RGBA) {
	rect := annotation.rect
	bounds := image.Rect(int(math.Floor(float64(rect.X*scaleX))), int(math.Floor(float64(rect.Y*scaleY))),
		int(math.Ceil(float64((rect.X+rect.Width)*scaleX))), int(math.Ceil(float64((rect.Y+rect.Height)*scaleY)))).Intersect(clip).Intersect(target.Bounds())
	if bounds.Empty() {
		return
	}
	radius := screenshotEditorRectCornerRadius(annotation)
	width = min(width, min(rect.Width, rect.Height)/2)
	raster := vector.NewRasterizer(bounds.Dx(), bounds.Dy())
	screenshotEditorRectRasterPath(raster, rect, radius, scaleX, scaleY, bounds.Min, false)
	inner := Rect{X: rect.X + width, Y: rect.Y + width, Width: rect.Width - 2*width, Height: rect.Height - 2*width}
	if inner.Width > 0 && inner.Height > 0 {
		// Opposite winding removes the interior without painting over the captured image or earlier marks.
		screenshotEditorRectRasterPath(raster, inner, max(float32(0), radius-width), scaleX, scaleY, bounds.Min, true)
	}
	raster.Draw(target, bounds, image.NewUniform(ink), image.Point{})
}

// screenshotEditorRectRasterPath adds a rounded contour; reflecting its Y coordinates reverses the inner contour's winding.
func screenshotEditorRectRasterPath(raster *vector.Rasterizer, rect Rect, radius, scaleX, scaleY float32, origin image.Point, reverse bool) {
	left, top := rect.X*scaleX-float32(origin.X), rect.Y*scaleY-float32(origin.Y)
	right, bottom := (rect.X+rect.Width)*scaleX-float32(origin.X), (rect.Y+rect.Height)*scaleY-float32(origin.Y)
	rx, ry := radius*scaleX, radius*scaleY
	if reverse {
		top, bottom = bottom, top
		ry = -ry
	}
	// Cubic quarter circles keep exported corners smooth at fractional capture scales.
	const kappa = 0.5522847498
	cx, cy := rx*kappa, ry*kappa
	raster.MoveTo(left+rx, top)
	raster.LineTo(right-rx, top)
	raster.CubeTo(right-rx+cx, top, right, top+ry-cy, right, top+ry)
	raster.LineTo(right, bottom-ry)
	raster.CubeTo(right, bottom-ry+cy, right-rx+cx, bottom, right-rx, bottom)
	raster.LineTo(left+rx, bottom)
	raster.CubeTo(left+rx-cx, bottom, left, bottom-ry+cy, left, bottom-ry)
	raster.LineTo(left, top+ry)
	raster.CubeTo(left, top+ry-cy, left+rx-cx, top, left+rx, top)
	raster.ClosePath()
}
