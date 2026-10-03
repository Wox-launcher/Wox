package screenshot

import (
	"fmt"
	"math"
	"strconv"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const (
	screenshotEditorFontSizeMin  = float32(12)
	screenshotEditorFontSizeMax  = float32(48)
	screenshotEditorFontSizeStep = float32(2)
)

// fontSizeLocked resolves the live draft before the selected annotation or creation default.
func (state *screenshotEditorOverlayState) fontSizeLocked() float32 {
	if state.textEditing {
		return state.textFontSize
	}
	if state.hasSelectedMark && state.selectedAnnotation >= 0 && state.selectedAnnotation < len(state.annotations) {
		annotation := state.annotations[state.selectedAnnotation]
		if annotation.tool == screenshotEditorToolText || annotation.tool == screenshotEditorToolNumber {
			return screenshotEditorAnnotationFontSize(annotation)
		}
	}
	if state.activeTool == screenshotEditorToolNumber {
		if state.numberFontSize <= 0 {
			return screenshotEditorNumberFontSize
		}
		return state.numberFontSize
	}
	return state.textFontSize
}

// setFontSizeLocked preserves the draft selection and composition during live size previews.
func (state *screenshotEditorOverlayState) setFontSizeLocked(value float32) {
	value = min(max(screenshotEditorFontSizeMin, value), screenshotEditorFontSizeMax)
	if state.textEditing {
		state.textFontSize = value
		return
	}
	if state.hasSelectedMark && state.selectedAnnotation >= 0 && state.selectedAnnotation < len(state.annotations) {
		annotation := &state.annotations[state.selectedAnnotation]
		if annotation.tool == screenshotEditorToolText || annotation.tool == screenshotEditorToolNumber {
			annotation.fontSize = value
		}
		// Consecutive numbering keeps the new size, while explicit selection edits affect only that marker.
		if annotation.tool == screenshotEditorToolNumber && state.activeTool == screenshotEditorToolNumber {
			state.numberFontSize = value
		}
		return
	}
	if state.activeTool == screenshotEditorToolText {
		state.textFontSize = value
	} else if state.activeTool == screenshotEditorToolNumber {
		state.numberFontSize = value
	}
}

// screenshotEditorFontSizeSliderProps keeps hit testing and painting on the same logical rail geometry.
func screenshotEditorFontSizeSliderProps(bounds Rect, scale, value float32) woxcomponent.SliderTrackProps {
	return woxcomponent.SliderTrackProps{
		Width: bounds.Width, Height: bounds.Height, Scale: scale, Value: value,
		Min: screenshotEditorFontSizeMin, Max: screenshotEditorFontSizeMax, Step: screenshotEditorFontSizeStep,
		Theme: woxcomponent.ControlTheme{
			Text: Color{R: 255, G: 255, B: 255, A: 255}, Border: Color{R: 255, G: 255, B: 255, A: 61},
			Accent: screenshotEditorPalette[2], Focus: screenshotEditorPalette[2],
		},
	}
}

// screenshotEditorEditBarSize wraps size properties below the palette when the display cannot fit a usable rail.
func screenshotEditorEditBarSize(frameWidth float32, tool screenshotEditorTool, selected bool, scale float32) (width, height float32, wrap bool) {
	if scale <= 0 {
		scale = 1
	}
	width, height = 192, 56
	hasFontSize := tool == screenshotEditorToolText || tool == screenshotEditorToolNumber
	if tool == screenshotEditorToolMosaic || tool == screenshotEditorToolEraser {
		width = 120
	} else if tool == screenshotEditorToolBrush {
		width = 288
	} else if hasFontSize {
		width = 376
	}
	deleteWidth := float32(0)
	if selected {
		deleteWidth = 54
		width += deleteWidth
	}
	available := max(float32(192), frameWidth/scale-48)
	width = min(width, available)
	if (hasFontSize && width < 328+deleteWidth) || (tool == screenshotEditorToolBrush && width < 288+deleteWidth) {
		height, wrap = 104, true
	}
	return width * scale, height * scale, wrap
}

// drawScreenshotEditorFontSizeValue measures and centers the complete label inside its reserved slot.
func drawScreenshotEditorFontSizeValue(list *DisplayList, window woxwidget.HostServices, bounds Rect, value, scale float32) {
	woxwidget.PaintStateless(window, woxwidget.TextBlock{
		Value: fmt.Sprintf("%.0f", value), Width: bounds.Width, Height: bounds.Height, LineHeight: bounds.Height,
		MaxLines: 1, Centered: true, AlignmentY: 0.5,
		Style: TextStyle{Size: woxcomponent.SettingsControlFontSize * scale, Weight: FontWeightSemibold},
		Color: Color{R: 255, G: 255, B: 255, A: 255},
	}, list, bounds)
}

// fontSizeKey lets keyboard users focus the slider with Tab and return to annotation input with Enter or Escape.
func (state *screenshotEditorOverlayState) fontSizeKey(event KeyEvent) bool {
	state.mu.Lock()
	if state.editFontSizeRect.Width <= 0 || event.Modifiers & ^KeyModifierShift != 0 {
		state.mu.Unlock()
		return false
	}
	handled := true
	if event.Key == Key("tab") {
		state.fontSizeFocused = !state.fontSizeFocused
	} else if !state.fontSizeFocused {
		handled = false
	} else {
		switch event.Key {
		case KeyArrowLeft, KeyArrowDown:
			state.setFontSizeLocked(state.fontSizeLocked() - screenshotEditorFontSizeStep)
		case KeyArrowRight, KeyArrowUp:
			state.setFontSizeLocked(state.fontSizeLocked() + screenshotEditorFontSizeStep)
		case KeyHome:
			state.setFontSizeLocked(screenshotEditorFontSizeMin)
		case KeyEnd:
			state.setFontSizeLocked(screenshotEditorFontSizeMax)
		case KeyEnter, KeyEscape:
			state.fontSizeFocused = false
		default:
			handled = false
		}
	}
	editing := state.textEditing
	state.mu.Unlock()
	if handled {
		state.setTextInputEnabled(editing)
		state.invalidate()
	}
	return handled
}

// fontSizeAccessibilityAction uses the same live value changes as pointer and keyboard input.
func (state *screenshotEditorOverlayState) fontSizeAccessibilityAction(action woxui.AccessibilityAction, input string) error {
	state.mu.Lock()
	if state.editFontSizeRect.Width <= 0 || state.sizeDialog != nil {
		state.mu.Unlock()
		return nil
	}
	value := state.fontSizeLocked()
	switch action {
	case woxui.AccessibilityActionFocus:
		state.fontSizeFocused = true
	case woxui.AccessibilityActionIncrement:
		value += screenshotEditorFontSizeStep
	case woxui.AccessibilityActionDecrement:
		value -= screenshotEditorFontSizeStep
	case woxui.AccessibilityActionSetValue:
		parsed, err := strconv.ParseFloat(input, 32)
		if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
			state.mu.Unlock()
			return fmt.Errorf("invalid font size %q", input)
		}
		value = float32(math.Round(parsed))
	default:
		state.mu.Unlock()
		return fmt.Errorf("unsupported font size action %q", action)
	}
	state.setFontSizeLocked(value)
	editing := state.textEditing
	state.mu.Unlock()
	state.setTextInputEnabled(editing)
	state.invalidate()
	return nil
}
