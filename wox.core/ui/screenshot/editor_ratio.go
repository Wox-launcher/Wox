package screenshot

import (
	"image"
	"math"

	"wox/common/icons"
	woxwidget "wox/ui/widget"
)

// screenshotRatioIndex maps the unlocked state and exact preset ratios to menu rows.
func screenshotRatioIndex(ratio float64) int {
	if ratio <= 0 {
		return 0
	}
	for index, preset := range screenshotSizeRatioPresets {
		if math.Abs(ratio-preset.ratio) < 1e-9 {
			return index + 1
		}
	}
	return -1
}

func screenshotRatioLabel(labels ScreenshotSizeLabels, ratio float64) string {
	index := screenshotRatioIndex(ratio)
	if index == 0 {
		return labels.RatioFree
	}
	if index < 0 {
		return labels.RatioCustom
	}
	return screenshotSizeRatioPresets[index-1].label
}

// screenshotRatioTriggerRect keeps the dropdown beside the size chip on its active display.
func screenshotRatioTriggerRect(window woxwidget.HostServices, chip, selection, display Rect, labels ScreenshotSizeLabels, ratio float64, scale float32) Rect {
	style := TextStyle{Size: 14 * scale, Weight: FontWeightSemibold}
	width := min(screenshotEditorMeasuredTextWidth(window, screenshotRatioLabel(labels, ratio), style)+42*scale, display.Width)
	gap := 4 * scale
	x := chip.X + chip.Width + gap
	if x+width > display.X+display.Width {
		x = chip.X - gap - width
	}
	x = max(display.X, min(x, display.X+display.Width-width))
	height := min(chip.Height, display.Height)
	y := max(display.Y, min(chip.Y, display.Y+display.Height-height))
	if y < selection.Y+selection.Height && y+height > selection.Y &&
		((x < selection.X && x+width > selection.X) || (x < selection.X+selection.Width && x+width > selection.X+selection.Width)) {
		right := selection.X + selection.Width + gap
		left := selection.X - gap - width
		switch {
		case right >= display.X && right+width <= display.X+display.Width:
			x = right
		case left >= display.X && left+width <= display.X+display.Width:
			x = left
		case selection.Width >= width:
			x = min(max(display.X, selection.X+selection.Width-width), display.X+display.Width-width)
		}
	}
	return Rect{X: x, Y: y, Width: width, Height: height}
}

// screenshotRatioMenuRect flips the list above the trigger when it would leave the active display.
func screenshotRatioMenuRect(window woxwidget.HostServices, trigger, display Rect, labels ScreenshotSizeLabels, scale float32) Rect {
	margin := min(4*scale, display.Height/2)
	height := min(8*28*scale, max(float32(0), display.Height-2*margin))
	style := TextStyle{Size: 13 * scale}
	width := screenshotEditorMeasuredTextWidth(window, labels.RatioFree, style)
	for _, preset := range screenshotSizeRatioPresets {
		width = max(width, screenshotEditorMeasuredTextWidth(window, preset.label, style))
	}
	width = min(max(trigger.Width, width+20*scale), display.Width)
	x := min(max(trigger.X, display.X), display.X+display.Width-width)
	y := trigger.Y + trigger.Height + margin
	if y+height > display.Y+display.Height-margin {
		y = trigger.Y - height - margin
	}
	y = min(max(y, display.Y+margin), max(display.Y+margin, display.Y+display.Height-margin-height))
	return Rect{X: x, Y: y, Width: width, Height: height}
}

// drawScreenshotRatioTrigger shows the current ratio independently from the editable size chip.
func drawScreenshotRatioTrigger(list *DisplayList, window woxwidget.HostServices, bounds Rect, labels ScreenshotSizeLabels, ratio float64, open bool, scale float32) {
	background := Color{R: 23, G: 23, B: 23, A: 255}
	if open {
		background = Color{R: 45, G: 75, B: 56, A: 255}
	}
	white := Color{R: 255, G: 255, B: 255, A: 255}
	woxwidget.PaintStateless(window, woxwidget.Container{Width: bounds.Width, Height: bounds.Height, Radius: 10 * scale, Color: background,
		Padding: woxwidget.Insets{Left: 8 * scale, Right: 8 * scale}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 6 * scale, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
			woxwidget.Expanded{Child: woxwidget.TextBlock{Value: screenshotRatioLabel(labels, ratio), Height: bounds.Height, LineHeight: bounds.Height, MaxLines: 1, AlignmentY: 0.5, Style: TextStyle{Size: 14 * scale, Weight: FontWeightSemibold}, Color: white}},
			woxwidget.Painter{Width: 20 * scale, Height: 20 * scale, Paint: func(list *DisplayList, bounds Rect) {
				drawScreenshotEditorToolbarIconSized(list, icons.ControlChevronDown, bounds, white, scale, 20)
			}},
		}}}, list, bounds)
}

// drawScreenshotRatioMenu paints above selection chrome so every preset remains visible and actionable.
func drawScreenshotRatioMenu(list *DisplayList, window woxwidget.HostServices, bounds Rect, labels ScreenshotSizeLabels, ratio float64, highlighted int, scale float32) {
	list.FillRoundedRect(bounds, 8*scale, Color{R: 23, G: 23, B: 23, A: 255})
	rowHeight := bounds.Height / float32(len(screenshotSizeRatioPresets)+1)
	selected := screenshotRatioIndex(ratio)
	for index := 0; index <= len(screenshotSizeRatioPresets); index++ {
		row := Rect{X: bounds.X + 2*scale, Y: bounds.Y + float32(index)*rowHeight, Width: max(0, bounds.Width-4*scale), Height: rowHeight}
		if index == selected || index == highlighted {
			color := Color{R: 255, G: 255, B: 255, A: 24}
			if index == selected {
				color = Color{R: 45, G: 75, B: 56, A: 255}
			}
			list.FillRoundedRect(row, 4*scale, color)
		}
		label := labels.RatioFree
		if index > 0 {
			label = screenshotSizeRatioPresets[index-1].label
		}
		woxwidget.PaintStateless(window, woxwidget.Container{Width: row.Width, Height: row.Height, Padding: woxwidget.Insets{Left: 8 * scale, Right: 8 * scale}, Child: woxwidget.TextBlock{
			Value: label, Height: row.Height, LineHeight: row.Height, MaxLines: 1, AlignmentY: 0.5,
			Style: TextStyle{Size: 13 * scale}, Color: Color{R: 255, G: 255, B: 255, A: 255},
		}}, list, row)
	}
}

// chooseScreenshotRatioLocked applies a preset directly to the capture, or unlocks without changing its dimensions.
func (state *screenshotEditorOverlayState) chooseScreenshotRatioLocked(index int) {
	if index < 0 || index > len(screenshotSizeRatioPresets) || !state.hasSelection || state.image == nil {
		return
	}
	if index == 0 {
		state.selectionAspectRatio = 0
		return
	}
	ratio := screenshotSizeRatioPresets[index-1].ratio
	if screenshotRatioIndex(state.selectionAspectRatio) == index {
		return
	}
	pixels, err := state.selectionPixelBoundsLocked()
	if err != nil {
		return
	}
	maxWidth := max(pixels.Dx(), state.image.Width-max(0, pixels.Min.X))
	maxHeight := max(pixels.Dy(), state.image.Height-max(0, pixels.Min.Y))
	width, height := screenshotSizePresetDimensions(pixels.Dx(), maxWidth, maxHeight, ratio)
	if width != pixels.Dx() || height != pixels.Dy() {
		selection := screenshotEditorSelectionWithPixelSize(state.selectionBoundsLocked(), state.frameSize, image.Pt(state.image.Width, state.image.Height), width, height)
		state.setSelectionLocked(normalizeScreenshotEditorRect(selection, state.frameSize))
	}
	state.selectionAspectRatio = ratio
}

// handleScreenshotRatioPointer owns the dropdown's pointer events before annotation or selection gestures.
func (state *screenshotEditorOverlayState) handleScreenshotRatioPointer(event PointerEvent) bool {
	state.mu.Lock()
	if event.Kind == PointerDown && state.hasSelection && screenshotEditorRectContains(state.sizeRatioRect, event.Position) {
		state.commitTextLocked()
		state.sizeRatioMenuOpen = !state.sizeRatioMenuOpen
		state.sizeRatioMenuIndex = screenshotRatioIndex(state.selectionAspectRatio)
		state.mu.Unlock()
		state.setTextInputEnabled(false)
		state.invalidate()
		return true
	}
	if !state.sizeRatioMenuOpen {
		state.mu.Unlock()
		return false
	}
	switch event.Kind {
	case PointerDown:
		if screenshotEditorRectContains(state.sizeRatioMenuRect, event.Position) {
			index := int((event.Position.Y - state.sizeRatioMenuRect.Y) * float32(len(screenshotSizeRatioPresets)+1) / state.sizeRatioMenuRect.Height)
			state.chooseScreenshotRatioLocked(index)
		}
		state.sizeRatioMenuOpen = false
	case PointerMove:
		state.pointerPosition, state.pointerInside = event.Position, true
		state.sizeRatioMenuIndex = -1
		if screenshotEditorRectContains(state.sizeRatioMenuRect, event.Position) {
			state.sizeRatioMenuIndex = int((event.Position.Y - state.sizeRatioMenuRect.Y) * float32(len(screenshotSizeRatioPresets)+1) / state.sizeRatioMenuRect.Height)
		}
	case PointerLeave:
		state.pointerInside = false
		state.sizeRatioMenuIndex = -1
	}
	state.mu.Unlock()
	state.invalidate()
	return true
}

// handleScreenshotRatioKey keeps menu navigation and Escape from reaching screenshot shortcuts.
func (state *screenshotEditorOverlayState) handleScreenshotRatioKey(event KeyEvent) bool {
	state.mu.Lock()
	if !state.sizeRatioMenuOpen {
		state.mu.Unlock()
		return false
	}
	switch event.Key {
	case KeyEscape:
		state.sizeRatioMenuOpen = false
	case KeyArrowUp, KeyArrowDown:
		step := 1
		if event.Key == KeyArrowUp {
			step = -1
		}
		count := len(screenshotSizeRatioPresets) + 1
		if state.sizeRatioMenuIndex < 0 && step < 0 {
			state.sizeRatioMenuIndex = 0
		}
		state.sizeRatioMenuIndex = (state.sizeRatioMenuIndex + step + count) % count
	case KeyEnter:
		state.chooseScreenshotRatioLocked(state.sizeRatioMenuIndex)
		state.sizeRatioMenuOpen = false
	}
	state.mu.Unlock()
	state.invalidate()
	return true
}
