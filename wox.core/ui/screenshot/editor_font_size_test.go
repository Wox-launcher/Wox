package screenshot

import (
	"fmt"
	"testing"

	woxui "wox/ui/runtime"
)

// TestScreenshotFontSizeValueUsesMeasuredWidth covers the clipped second digit and taller configured fonts.
func TestScreenshotFontSizeValueUsesMeasuredWidth(t *testing.T) {
	surface := &screenshotTooltipTestSurface{}
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		for _, size := range []float32{12, 20, 48} {
			bounds := Rect{X: 30, Y: 40, Width: 44 * scale, Height: 56 * scale}
			actual, expected := &DisplayList{}, &DisplayList{}
			drawScreenshotEditorFontSizeValue(actual, surface, bounds, size, scale)
			label, style := fmt.Sprintf("%.0f", size), TextStyle{Size: 13 * scale, Weight: FontWeightSemibold}
			metrics, _ := surface.MeasureText(label, style)
			expected.DrawText(label, Rect{X: bounds.X + (bounds.Width-metrics.Size.Width)/2, Y: bounds.Y + (bounds.Height-metrics.Size.Height)/2,
				Width: metrics.Size.Width, Height: metrics.Size.Height}, style, Color{R: 255, G: 255, B: 255, A: 255})
			if err := actual.Compare(expected); err != nil {
				t.Fatalf("size %v scale %v: value is clipped or not centered: %v", size, scale, err)
			}
		}
	}
}

// TestScreenshotFontSizeDragPreservesDraft exercises dragging beyond the rail and returning to text input.
func TestScreenshotFontSizeDragPreservesDraft(t *testing.T) {
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		for _, existing := range []bool{false, true} {
			state := &screenshotEditorOverlayState{hasSelection: true, activeTool: screenshotEditorToolText, textEditing: true,
				textFontSize: 20, uiScale: scale, editFontSizeRect: Rect{X: 80, Y: 100, Width: 112 * scale, Height: 42 * scale},
				hasEditingText: existing, hasSelectedMark: existing, annotations: []screenshotEditorAnnotation{{tool: screenshotEditorToolText, text: "原文", fontSize: 20}}}
			state.resetTextEditorLocked("中文abc")
			state.textEditor.SetSelection(2, 5)
			state.textInput(TextInputEvent{Kind: TextInputCompose, Text: "ni"})
			before := state.textEditor.State()
			slider := state.editFontSizeRect
			state.pointer(PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: Point{X: slider.X + slider.Width/2, Y: slider.Y + 10}})
			if state.textFontSize != 30 || !state.fontSizeDragging || !state.textEditing {
				t.Fatalf("scale %v: slider did not begin a live preview", scale)
			}
			state.pointer(PointerEvent{Kind: PointerMove, Position: Point{X: slider.X + slider.Width + 100, Y: slider.Y - 50}})
			if state.textFontSize != 48 || state.textEditor.State() != before || state.annotations[0].text != "原文" {
				t.Fatal("drag lost draft or failed to clamp to maximum")
			}
			state.pointer(PointerEvent{Kind: PointerUp, Button: PointerButtonPrimary})
			if state.fontSizeDragging {
				t.Fatal("slider kept dragging after release")
			}
			state.textInput(TextInputEvent{Kind: TextInputCommit, Text: "你"})
			if !state.textEditing || state.fontSizeFocused || state.textDraft != "中文你" {
				t.Fatalf("typing after resizing lost input: %q", state.textDraft)
			}
		}
	}
}

// TestScreenshotFontSizeKeyboardAndAccessibility shares limits while preserving the screenshot on Escape.
func TestScreenshotFontSizeKeyboardAndAccessibility(t *testing.T) {
	state := &screenshotEditorOverlayState{activeTool: screenshotEditorToolText, textFontSize: 20, editFontSizeRect: Rect{Width: 112, Height: 42}}
	if !state.key(KeyEvent{Key: Key("tab"), Down: true}) || !state.fontSizeFocused {
		t.Fatal("Tab did not focus size slider")
	}
	state.key(KeyEvent{Key: KeyArrowRight, Down: true})
	if state.textFontSize != 22 {
		t.Fatalf("keyboard size = %v", state.textFontSize)
	}
	state.key(KeyEvent{Key: KeyEnd, Down: true})
	state.key(KeyEvent{Key: KeyArrowRight, Down: true})
	if state.textFontSize != 48 {
		t.Fatal("keyboard exceeded size limit")
	}
	if !state.key(KeyEvent{Key: KeyEscape, Down: true}) || state.fontSizeFocused {
		t.Fatal("Escape did not leave slider focus")
	}
	if err := state.fontSizeAccessibilityAction(woxui.AccessibilityActionSetValue, "24"); err != nil || state.textFontSize != 24 {
		t.Fatalf("accessible size change: %v", err)
	}
	if err := state.fontSizeAccessibilityAction(woxui.AccessibilityActionSetValue, "NaN"); err == nil || state.textFontSize != 24 {
		t.Fatal("invalid accessible value changed size")
	}
}

// TestScreenshotFontSizeLayoutFitsDisplay keeps the value and delete control reachable after DPI or width changes.
func TestScreenshotFontSizeLayoutFitsDisplay(t *testing.T) {
	for _, tool := range []screenshotEditorTool{screenshotEditorToolText, screenshotEditorToolNumber} {
		for _, scale := range []float32{1, 1.25, 1.5, 2, 1} {
			for _, frameWidth := range []float32{480, 800, 1600} {
				for _, selected := range []bool{false, true} {
					state := &screenshotEditorOverlayState{}
					annotation := (*screenshotEditorAnnotation)(nil)
					if selected {
						annotation = &screenshotEditorAnnotation{tool: tool, fontSize: 20}
					}
					state.drawEditBar(&DisplayList{}, Size{Width: frameWidth, Height: 1200}, Rect{X: 24 * scale, Y: 300, Width: 300, Height: 60 * scale},
						Rect{X: 100, Y: 50, Width: 200, Height: 200}, tool, annotation, screenshotEditorAnnotationColor, 18, 20, scale)
					bar, slider := state.editBarRect, state.editFontSizeRect
					if bar.X < 0 || bar.X+bar.Width > frameWidth || slider.Width < 48*scale || slider.Y+slider.Height > bar.Y+bar.Height {
						t.Fatalf("scale=%v width=%v selected=%t: bar=%+v slider=%+v", scale, frameWidth, selected, bar, slider)
					}
					valueRight := slider.X + slider.Width + 52*scale
					if valueRight > bar.X+bar.Width-12*scale {
						t.Fatalf("font-size value escapes its property bar: right=%v bar=%+v", valueRight, bar)
					}
					if selected && state.editDeleteRect.X+state.editDeleteRect.Width > bar.X+bar.Width {
						t.Fatalf("delete control escapes its property bar: %+v", state.editDeleteRect)
					}
					if bar.Height == 104*scale && slider.Y < bar.Y+48*scale {
						t.Fatal("wrapped slider overlaps the color palette")
					}
				}
			}
		}
	}
}
