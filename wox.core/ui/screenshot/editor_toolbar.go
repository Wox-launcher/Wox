package screenshot

// screenshotEditorToolbarGripSlot is the narrow leading strip reserved for the drag handle.
// It is narrower than a tool slot so the affordance stays secondary to the actions.
const screenshotEditorToolbarGripSlot = 12

// screenshotEditorToolbarFrame avoids monitor gaps by using the display containing most of the selection.
// Display candidates already use the same editor coordinates as the selection.
func screenshotEditorToolbarFrame(selection, fallback Rect, displays []Rect) Rect {
	frame := fallback
	var largestArea float32
	for _, display := range displays {
		width := max(float32(0), min(selection.X+selection.Width, display.X+display.Width)-max(selection.X, display.X))
		height := max(float32(0), min(selection.Y+selection.Height, display.Y+display.Height)-max(selection.Y, display.Y))
		if area := width * height; area > largestArea {
			frame, largestArea = display, area
		}
	}
	return frame
}

// screenshotEditorToolbarLayout wraps complete button slots on narrow or scaled displays without shrinking hit targets.
func screenshotEditorToolbarLayout(frameWidth, scale float32, hideTools, recording, scrolling, background bool, extraCount int) (Size, []Rect) {
	const (
		buttonSize  = 40
		buttonInset = 4
		edgePadding = 16
		rowHeight   = buttonSize + 2*buttonInset
	)
	buttonCount := extraCount + 3 // Cancel, save, and confirm.
	if !hideTools {
		buttonCount += int(screenshotEditorToolCount-1) + 3 // Annotation tools, undo, cursor, and pin.
		if scrolling {
			buttonCount++
		}
		if background {
			buttonCount++
		}
		if recording {
			buttonCount++
		}
	}
	contentLeft := float32(edgePadding + screenshotEditorToolbarGripSlot + buttonInset)
	width := contentLeft + float32(buttonCount*buttonSize) + edgePadding + buttonInset
	scale = max(float32(1), scale)
	width = min(width, max(float32(80)+screenshotEditorToolbarGripSlot, frameWidth/scale-48))
	left, top := contentLeft, float32(0)
	buttons := make([]Rect, 0, buttonCount)
	for index := 0; index < buttonCount; index++ {
		if left+buttonSize > width-edgePadding {
			left, top = contentLeft, top+rowHeight
		}
		buttons = append(buttons, Rect{X: left * scale, Y: (top + buttonInset) * scale, Width: buttonSize * scale, Height: buttonSize * scale})
		left += buttonSize
	}
	return Size{Width: width * scale, Height: (top + rowHeight) * scale}, buttons
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
