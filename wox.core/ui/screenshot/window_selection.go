package screenshot

// windowAtPoint resolves the first visible frame in the frozen desktop's stacking order.
// Candidates are clipped to the capture surface, so offscreen pixels cannot enter the selection.
func (state *screenshotEditorOverlayState) windowAtPoint(point Point) Rect {
	for _, candidate := range state.windowCandidates {
		if point.X >= candidate.X && point.Y >= candidate.Y && point.X < candidate.X+candidate.Width && point.Y < candidate.Y+candidate.Height {
			return candidate
		}
	}
	return Rect{}
}

// selectionFallbackAtPoint keeps window capture eligibility separate from a display selected over empty desktop.
func (state *screenshotEditorOverlayState) selectionFallbackAtPoint(point Point) Rect {
	if window := state.windowAtPoint(point); window.Width >= 2 && window.Height >= 2 {
		return window
	}
	for _, display := range state.displayCandidates {
		if screenshotEditorRectContains(display, point) {
			return display
		}
	}
	return Rect{}
}

// screenshotSelectionDisplayBounds maps each captured display into editor coordinates, preserving gaps and negative desktop origins.
// Windows uses physical pixels for its spanning surface; Linux uses the captured desktop's logical bounds.
func screenshotSelectionDisplayBounds(displays []screenshotDisplay, capture Rect, physical bool) []Rect {
	var candidates []Rect
	for _, display := range displays {
		bounds := display.Bounds
		if physical {
			bounds = display.PixelBounds
		}
		left, top := max(float32(bounds.X), capture.X), max(float32(bounds.Y), capture.Y)
		right := min(float32(bounds.X+bounds.Width), capture.X+capture.Width)
		bottom := min(float32(bounds.Y+bounds.Height), capture.Y+capture.Height)
		if right-left >= 2 && bottom-top >= 2 {
			candidates = append(candidates, Rect{X: left - capture.X, Y: top - capture.Y, Width: right - left, Height: bottom - top})
		}
	}
	return candidates
}
