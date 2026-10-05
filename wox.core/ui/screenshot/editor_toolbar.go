package screenshot

// screenshotEditorToolbarLayout wraps complete button slots on narrow or scaled displays without shrinking hit targets.
func screenshotEditorToolbarLayout(frameWidth, scale float32, hideTools, recording, scrolling, background bool, extraCount int) (Size, []Rect) {
	steps := make([]float32, 0, int(screenshotEditorToolCount)+extraCount+8)
	width := float32(182)
	if !hideTools {
		for tool := screenshotEditorTool(1); tool < screenshotEditorToolCount; tool++ {
			steps = append(steps, 48)
		}
		steps[len(steps)-1] += 6
		steps = append(steps, 54, 54, 54, 54)
		if !scrolling {
			steps = steps[:len(steps)-1]
		}
		width = 398 + 48*float32(screenshotEditorToolCount-1)
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
	width = min(width, max(float32(80), frameWidth/scale-48))
	left, top := float32(16), float32(0)
	buttons := make([]Rect, 0, len(steps))
	for _, step := range steps {
		if left+44 > width-16 {
			left, top = 16, top+48
		}
		buttons = append(buttons, Rect{X: (left + 4) * scale, Y: (top + 4) * scale, Width: 40 * scale, Height: 40 * scale})
		left += step
	}
	return Size{Width: width * scale, Height: (top + 48) * scale}, buttons
}
