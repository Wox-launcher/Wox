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
