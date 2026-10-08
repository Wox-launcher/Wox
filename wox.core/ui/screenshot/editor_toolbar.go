package screenshot

// screenshotEditorToolbarGripSlot is the narrow leading strip reserved for the drag handle.
// It is narrower than a tool slot so the affordance stays secondary to the actions.
const screenshotEditorToolbarGripSlot = 12

// screenshotEditorToolbarLayout wraps complete button slots on narrow or scaled displays without shrinking hit targets.
func screenshotEditorToolbarLayout(frameWidth, scale float32, hideTools, recording, scrolling, background bool, extraCount int) (Size, []Rect) {
	steps := make([]float32, 0, int(screenshotEditorToolCount)+extraCount+8)
	width := float32(182) + screenshotEditorToolbarGripSlot
	if !hideTools {
		for tool := screenshotEditorTool(1); tool < screenshotEditorToolCount; tool++ {
			steps = append(steps, 48)
		}
		steps[len(steps)-1] += 6
		steps = append(steps, 54, 54, 54, 54)
		if !scrolling {
			steps = steps[:len(steps)-1]
		}
		width = 398 + screenshotEditorToolbarGripSlot + 48*float32(screenshotEditorToolCount-1)
		if !scrolling {
			width -= 54
		}
		if background {
			steps = append(steps, 54)
			width += 54
		}
		if recording {
			steps = append(steps, 48)
			width += 54
		}
	}
	for index := 0; index < extraCount; index++ {
		steps = append(steps, 54)
		width += 54
	}
	steps = append(steps, 48, 48, 48)
	scale = max(float32(1), scale)
	width = min(width, max(float32(80)+screenshotEditorToolbarGripSlot, frameWidth/scale-48))
	contentLeft := float32(16) + screenshotEditorToolbarGripSlot
	left, top := contentLeft, float32(0)
	buttons := make([]Rect, 0, len(steps))
	for _, step := range steps {
		if left+44 > width-16 {
			left, top = contentLeft, top+48
		}
		buttons = append(buttons, Rect{X: (left + 4) * scale, Y: (top + 4) * scale, Width: 40 * scale, Height: 40 * scale})
		left += step
	}
	return Size{Width: width * scale, Height: (top + 48) * scale}, buttons
}

// screenshotEditorToolbarGripRect is the leading drag target, ending where the first tool begins.
func screenshotEditorToolbarGripRect(toolbar Rect, scale float32) Rect {
	if toolbar.Width <= 0 || toolbar.Height <= 0 {
		return Rect{}
	}
	scale = max(float32(1), scale)
	return Rect{
		X: toolbar.X, Y: toolbar.Y,
		Width: (16 + screenshotEditorToolbarGripSlot) * scale, Height: min(48*scale, toolbar.Height),
	}
}

// clampScreenshotEditorToolbarOrigin keeps a user-dragged toolbar inside the frame.
func clampScreenshotEditorToolbarOrigin(origin Point, width, height float32, frame Size, uiScale float32) Point {
	inset := 8 * max(float32(1), uiScale)
	return Point{
		X: min(max(origin.X, inset), max(inset, frame.Width-width-inset)),
		Y: min(max(origin.Y, inset), max(inset, frame.Height-height-inset)),
	}
}
