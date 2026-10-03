package screenshot

import (
	"cmp"
	"image"
	"image/color"
	"image/draw"
	"math"
	"slices"
)

var screenshotEditorBrushRadii = [...]float32{2, 4, 8}

// screenshotEditorEraserPreview retains only the source patch covered by one stroke, not another desktop image.
// Erasers cannot be selected or moved, so appended points and render scale completely identify the patch.
type screenshotEditorEraserPreview struct {
	source     *Image
	frame      Size
	pointCount int
	radius     float32
	image      *Image
	raster     *image.RGBA
	bounds     Rect
}

// nextAnnotationPaintOrderLocked places new marks above every existing mark, including restored annotations.
func (state *screenshotEditorOverlayState) nextAnnotationPaintOrderLocked() uint64 {
	var order uint64
	for _, annotation := range state.annotations {
		order = max(order, annotation.paintOrder)
	}
	return order + 1
}

// screenshotEditorPaintOrder leaves creation order intact for selection indexes and undo.
func screenshotEditorPaintOrder(annotations []screenshotEditorAnnotation) []screenshotEditorAnnotation {
	compare := func(a, b screenshotEditorAnnotation) int { return cmp.Compare(a.paintOrder, b.paintOrder) }
	if slices.IsSortedFunc(annotations, compare) {
		return annotations
	}
	ordered := slices.Clone(annotations)
	slices.SortStableFunc(ordered, compare)
	return ordered
}

// strokeRadiusLocked resolves independent brush and eraser creation sizes in logical units.
func (state *screenshotEditorOverlayState) strokeRadiusLocked(tool screenshotEditorTool) float32 {
	radius := state.brushRadius
	if tool == screenshotEditorToolEraser {
		radius = state.eraserRadius
	}
	return screenshotEditorStrokeRadius(screenshotEditorAnnotation{tool: tool, strokeRadius: radius}, 1)
}

// screenshotEditorStrokeRadius scales chrome units once before mapping the stroke into capture pixels.
func screenshotEditorStrokeRadius(annotation screenshotEditorAnnotation, uiScale float32) float32 {
	radius := annotation.strokeRadius
	if radius <= 0 {
		radius = screenshotEditorBrushRadii[0]
		if annotation.tool == screenshotEditorToolEraser {
			radius = screenshotEditorMosaicRadius
		}
	}
	return radius * max(float32(1), uiScale)
}

// appendStrokePointLocked keeps release coordinates and sparse pointer events connected without dropping short strokes.
func (state *screenshotEditorOverlayState) appendStrokePointLocked(point Point) {
	if state.draft == nil || (state.draft.tool != screenshotEditorToolBrush && state.draft.tool != screenshotEditorToolEraser) {
		return
	}
	points := state.draft.points
	if len(points) == 0 || points[len(points)-1] != point {
		state.draft.points = append(points, point)
	}
}

// screenshotEditorStrokeContains tests capsules between samples, including a single-click dot.
func screenshotEditorStrokeContains(points []Point, point Point, radius float32) bool {
	for index, end := range points {
		start := points[max(0, index-1)]
		if screenshotEditorDistanceToSegment(point, start, end) <= radius {
			return true
		}
	}
	return false
}

// drawScreenshotEditorBrush joins freehand segments with round caps so fast drags stay continuous.
func drawScreenshotEditorBrush(list *DisplayList, annotation screenshotEditorAnnotation, uiScale float32) {
	width := screenshotEditorStrokeRadius(annotation, uiScale) * 2
	color := screenshotEditorAnnotationDrawColor(annotation)
	for index, point := range annotation.points {
		if index > 0 {
			drawScreenshotEditorLine(list, annotation.points[index-1], point, width, color)
		}
		drawScreenshotEditorLine(list, point, point, width, color)
	}
}

// drawScreenshotEditorStrokeCursor shows the exact logical brush radius independently of desktop origin or capture scale.
func drawScreenshotEditorStrokeCursor(list *DisplayList, point Point, radius, uiScale float32) {
	bounds := Rect{X: point.X - radius, Y: point.Y - radius, Width: radius * 2, Height: radius * 2}
	width := max(float32(1), uiScale)
	list.StrokeRoundedRect(bounds, radius, width*2, Color{A: 255})
	list.StrokeRoundedRect(bounds, radius, width, Color{R: 255, G: 255, B: 255, A: 255})
}

// paintScreenshotEditorStroke evaluates capsules in logical coordinates, preserving circular strokes with unequal capture scales.
func paintScreenshotEditorStroke(target *image.RGBA, clip image.Rectangle, points []Point, radius, scaleX, scaleY float32, pixel func(int, int) color.RGBA) {
	if radius <= 0 || scaleX <= 0 || scaleY <= 0 {
		return
	}
	clip = clip.Intersect(target.Bounds())
	for index, end := range points {
		start := points[max(0, index-1)]
		dx, dy := end.X-start.X, end.Y-start.Y
		lengthSquared := dx*dx + dy*dy
		bounds := image.Rect(
			int(math.Floor(float64((min(start.X, end.X)-radius)*scaleX))),
			int(math.Floor(float64((min(start.Y, end.Y)-radius)*scaleY))),
			int(math.Ceil(float64((max(start.X, end.X)+radius)*scaleX))),
			int(math.Ceil(float64((max(start.Y, end.Y)+radius)*scaleY))),
		).Intersect(clip)
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				px, py := (float32(x)+0.5)/scaleX-start.X, (float32(y)+0.5)/scaleY-start.Y
				ratio := float32(0)
				if lengthSquared > 0 {
					ratio = min(max(float32(0), (px*dx+py*dy)/lengthSquared), 1)
				}
				ex, ey := px-ratio*dx, py-ratio*dy
				if ex*ex+ey*ey <= radius*radius {
					target.SetRGBA(x, y, pixel(x, y))
				}
			}
		}
	}
}

// drawScreenshotEditorEraser overlays original capture pixels; all underlying annotation geometry remains editable.
func drawScreenshotEditorEraser(list *DisplayList, annotation screenshotEditorAnnotation, source *Image, frame Size, uiScale float32) {
	if source == nil || frame.Width <= 0 || frame.Height <= 0 || len(annotation.points) == 0 {
		return
	}
	cache := annotation.eraserPreview
	if cache == nil {
		cache = &screenshotEditorEraserPreview{}
	}
	radius := screenshotEditorStrokeRadius(annotation, uiScale)
	if cache.source != source || cache.frame != frame || cache.pointCount != len(annotation.points) || cache.radius != radius {
		scaleX, scaleY := float32(source.Width)/frame.Width, float32(source.Height)/frame.Height
		bounds := screenshotEditorAnnotationBounds(annotation, uiScale)
		clip := image.Rect(int(math.Floor(float64(bounds.X*scaleX))), int(math.Floor(float64(bounds.Y*scaleY))),
			int(math.Ceil(float64((bounds.X+bounds.Width)*scaleX))), int(math.Ceil(float64((bounds.Y+bounds.Height)*scaleY)))).Intersect(image.Rect(0, 0, source.Width, source.Height))
		cache.image = nil
		if clip.Empty() {
			cache.raster = nil
		}
		if !clip.Empty() {
			patch := image.NewRGBA(clip)
			points := annotation.points
			if cache.raster != nil && cache.source == source && cache.frame == frame && cache.radius == radius && cache.pointCount > 0 && cache.pointCount < len(points) {
				// Published renderer images are immutable. Copy the previous patch, then rasterize only the newly appended segments.
				draw.Draw(patch, cache.raster.Bounds(), cache.raster, cache.raster.Bounds().Min, draw.Src)
				points = points[cache.pointCount-1:]
			}
			paintScreenshotEditorStroke(patch, clip, points, radius, scaleX, scaleY, source.RGBAAt)
			cache.raster = patch
			cache.image, _ = NewImageFromPackedRGBA(patch)
			cache.bounds = Rect{X: float32(clip.Min.X) / scaleX, Y: float32(clip.Min.Y) / scaleY, Width: float32(clip.Dx()) / scaleX, Height: float32(clip.Dy()) / scaleY}
		}
		cache.source, cache.frame, cache.pointCount, cache.radius = source, frame, len(annotation.points), radius
	}
	if cache.image != nil {
		list.DrawImage(cache.image, cache.bounds)
	}
}
