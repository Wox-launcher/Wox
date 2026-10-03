package screenshot

import "math"

// mosaicPointerCursorLocked replaces the native pointer only where the mosaic brush can paint.
func (state *screenshotEditorOverlayState) mosaicPointerCursorLocked(point Point) PointerCursor {
	if !state.pointerInside || !state.hasSelection || state.hideTools || state.scrolling || state.scrollingStarting ||
		!screenshotEditorRectContains(state.selection, point) ||
		screenshotEditorRectContains(state.toolbarRect, point) ||
		screenshotEditorRectContains(state.editBarRect, point) ||
		screenshotEditorRectContains(state.sizeLabelRect, point) {
		return PointerCursorDefault
	}
	return PointerCursorHidden
}

// drawScreenshotEditorMosaicCursor outlines the brush's visible height without adding it to exported annotations.
func drawScreenshotEditorMosaicCursor(displayList *DisplayList, point Point, radius, uiScale float32) {
	if radius <= 0 {
		radius = screenshotEditorMosaicRadius
	}
	// Preview blocks are centered on a fixed grid around the pointer. Include the outer half-block,
	// and keep this radius in the same frame coordinates as the brush instead of scaling it twice.
	radius = float32(math.Floor(float64(radius/screenshotEditorMosaicBlockSize)))*screenshotEditorMosaicBlockSize + screenshotEditorMosaicBlockSize/2
	bounds := Rect{X: point.X - radius, Y: point.Y - radius, Width: radius * 2, Height: radius * 2}
	stroke := max(float32(1), uiScale)
	displayList.StrokeRoundedRect(bounds, radius, stroke*2, Color{A: 255})
	displayList.StrokeRoundedRect(bounds, radius, stroke, Color{R: 255, G: 255, B: 255, A: 255})
}
