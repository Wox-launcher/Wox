package screenshot

import (
	"bytes"
	"errors"
	"image"
	"math"
	"strings"
	"testing"

	woxui "wox/ui/runtime"
)

// screenshotAnnotationTextSurface models proportional advances with an AV kerning pair.
type screenshotAnnotationTextSurface struct{ *screenshotTestSurface }

// MeasureText supplies shaped advances that deliberately differ from the editor's heuristic widths.
func (*screenshotAnnotationTextSurface) MeasureText(text string, style TextStyle) (woxui.TextMetrics, error) {
	if style.Weight != FontWeightSemibold {
		return woxui.TextMetrics{}, errors.New("annotation measurements must use the drawing weight")
	}
	width := float32(0)
	for _, character := range text {
		switch character {
		case 'W':
			width += 24
		case 'i':
			width += 3
		case 'A':
			width += 14
		case 'V':
			width += 16
		case '中', '文', '字':
			width += 18
		default:
			width += 10
		}
	}
	width -= float32(strings.Count(text, "AV")) * 6
	return woxui.TextMetrics{Size: Size{Width: width * style.Size / 20, Height: style.Size + 4}}, nil
}

// TestScreenshotEditorShiftEnterEditsMultilineText covers replacement, undo, and the separate finish action.
func TestScreenshotEditorShiftEnterEditsMultilineText(t *testing.T) {
	state := &screenshotEditorOverlayState{textEditing: true, textFontSize: 20, uiScale: 1}
	state.resetTextEditorLocked("第一行第二行")
	state.textEditor.SetCaret(3)
	newline := KeyEvent{Key: KeyEnter, Down: true, Modifiers: KeyModifierShift}
	if !state.key(newline) || !state.textEditing || state.textDraft != "第一行\n第二行" || state.textCaret != 4 || len(state.annotations) != 0 {
		t.Fatalf("Shift+Enter should insert a newline without finishing: text=%q caret=%d editing=%t", state.textDraft, state.textCaret, state.textEditing)
	}
	state.key(KeyEvent{Key: Key("z"), Down: true, Modifiers: screenshotEditorPrimaryModifier()})
	if state.textDraft != "第一行第二行" || state.textCaret != 3 {
		t.Fatalf("undo newline = %q, caret=%d", state.textDraft, state.textCaret)
	}
	state.key(KeyEvent{Key: Key("z"), Down: true, Modifiers: screenshotEditorPrimaryModifier() | KeyModifierShift})
	state.textEditor.SetSelection(2, 5)
	state.key(newline)
	if state.textDraft != "第一\n二行" || state.textCaret != 3 {
		t.Fatalf("newline replacement = %q, caret=%d", state.textDraft, state.textCaret)
	}
	state.textInput(TextInputEvent{Kind: TextInputCompose, Text: "zhong"})
	before := state.textEditor.State()
	newline.Composing = true
	if state.key(newline) || state.textEditor.State() != before {
		t.Fatal("Shift+Enter intercepted active IME composition")
	}
	state.textInput(TextInputEvent{Kind: TextInputCommit, Text: "中"})
	state.key(KeyEvent{Key: KeyEnter, Down: true})
	if state.textEditing || len(state.annotations) != 1 || state.annotations[0].text != "第一\n中二行" {
		t.Fatalf("Enter did not finish the multiline label: %+v", state.annotations)
	}
}

// TestScreenshotEditorMultilinePasteAndNavigation preserves newlines and moves within hard lines.
func TestScreenshotEditorMultilinePasteAndNavigation(t *testing.T) {
	state := &screenshotEditorOverlayState{textEditing: true, textFontSize: 20, uiScale: 1}
	state.resetTextEditorLocked("")
	state.pasteTextEditing("中文\r\n字\r中文\n")
	if state.textDraft != "中文\n字\n中文\n" {
		t.Fatalf("paste lost hard lines: %q", state.textDraft)
	}
	state.textInput(TextInputEvent{Kind: TextInputCommit, Text: "末\r\n行"})
	if state.textDraft != "中文\n字\n中文\n末\n行" {
		t.Fatalf("text input lost hard lines: %q", state.textDraft)
	}
	state.textEditor.SetCaret(2)
	for _, step := range []struct {
		key       Key
		modifiers KeyModifiers
		focus     int
		selected  string
	}{
		{KeyArrowDown, 0, 4, ""},
		{KeyArrowDown, 0, 7, ""},
		{KeyHome, 0, 5, ""},
		{KeyEnd, KeyModifierShift, 7, "中文"},
		{KeyArrowUp, KeyModifierShift, 4, "\n"},
	} {
		if !state.key(KeyEvent{Key: step.key, Down: true, Modifiers: step.modifiers}) || state.textCaret != step.focus || state.textEditor.SelectedText() != step.selected {
			t.Fatalf("%s: caret=%d selected=%q, want %d/%q", step.key, state.textCaret, state.textEditor.SelectedText(), step.focus, step.selected)
		}
	}
}

// TestScreenshotEditorMultilineCaretAndHitBounds covers empty lines, IME replacement, display changes, and negative origins.
func TestScreenshotEditorMultilineCaretAndHitBounds(t *testing.T) {
	surface := &screenshotAnnotationTextSurface{}
	annotation := screenshotEditorAnnotation{tool: screenshotEditorToolText, text: "Wi\n\nAV中\n", fontSize: 20}
	for _, scale := range []float32{1, 1.25, 1.5, 2, 1} {
		for _, origin := range []Point{{X: 40, Y: 60}, {X: -800.5, Y: -300.25}} {
			annotation.start = origin
			if got := screenshotEditorAnnotationBounds(annotation, scale); got != (Rect{X: origin.X, Y: origin.Y, Width: screenshotEditorEstimatedTextWidth("AV中", 20*scale), Height: 112 * scale}) {
				t.Fatalf("scale %v: multiline bounds = %+v", scale, got)
			}
			for _, point := range []struct {
				prefix string
				x, y   float32
				index  int
			}{
				{"Wi", 27, 0, 2}, {"Wi\n", 0, 28, 3}, {"Wi\n\nA", 14, 56, 5}, {"Wi\n\nAV中\n", 0, 84, 8},
			} {
				caret := screenshotEditorTextCaretRect(surface, origin, point.prefix, 20, scale)
				if caret.X != origin.X+point.x*scale || caret.Y != origin.Y+point.y*scale {
					t.Fatalf("scale %v: caret for %q = %+v", scale, point.prefix, caret)
				}
				offset := Point{X: caret.X - origin.X, Y: caret.Y - origin.Y + 5*scale}
				if got := screenshotEditorTextCaretIndex(surface, annotation.text, offset, 20*scale); got != point.index {
					t.Fatalf("scale %v: click at %+v = %d, want %d", scale, offset, got, point.index)
				}
			}
		}
	}
	editor := NewTextEditor(annotation.text)
	editor.SetSelection(4, 6)
	editor.HandleTextInput(TextInputEvent{Kind: TextInputCompose, Text: "文字"})
	_, prefix := screenshotEditorTextEditingPreview(editor.State())
	if caret := screenshotEditorTextCaretRect(surface, Point{}, prefix, 20, 1.5); caret.X != 54 || caret.Y != 84 {
		t.Fatalf("multiline IME replacement anchor = %+v", caret)
	}
}

// TestScreenshotEditorMultilineSelectionHighlightsEachLine checks reversed selection through an empty line.
func TestScreenshotEditorMultilineSelectionHighlightsEachLine(t *testing.T) {
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		actual, expected := &DisplayList{}, &DisplayList{}
		drawScreenshotEditorTextSelection(actual, &screenshotAnnotationTextSurface{}, Point{X: 40, Y: 60}, "Wi\n\nAV中", TextSelection{Anchor: 6, Focus: 1}, 20*scale, screenshotEditorAnnotationColor, scale)
		color := screenshotEditorAnnotationColor
		color.A = 70
		for row, span := range []struct{ x, width float32 }{{24, 13}, {0, 10}, {0, 24}} {
			expected.FillRect(Rect{X: 40 + span.x*scale, Y: 60 + float32(row)*28*scale, Width: span.width * scale, Height: 24 * scale}, color)
		}
		if err := actual.Compare(expected); err != nil {
			t.Fatalf("scale %v: multiline selection: %v", scale, err)
		}
	}
}

// TestScreenshotEditorMultilinePreviewAndExport keeps blank lines and horizontal origins in both render paths.
func TestScreenshotEditorMultilinePreviewAndExport(t *testing.T) {
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		annotation := screenshotEditorAnnotation{tool: screenshotEditorToolText, text: "Wi\n\nAV中", start: Point{X: 20, Y: 20}, fontSize: 20}
		lines := []screenshotEditorAnnotation{annotation, annotation}
		lines[0].text = "Wi"
		lines[1].text = "AV中"
		lines[1].start.Y += 56 * scale
		actual, expected := &DisplayList{}, &DisplayList{}
		drawScreenshotEditorAnnotations(actual, nil, []screenshotEditorAnnotation{annotation}, nil, Size{}, scale)
		drawScreenshotEditorAnnotations(expected, nil, lines, nil, Size{}, scale)
		if err := actual.Compare(expected); err != nil {
			t.Fatalf("scale %v: preview does not draw separate lines: %v", scale, err)
		}
		// Captured pixels can have a different scale from editor chrome.
		source := image.NewRGBA(image.Rect(0, 0, 600, 400))
		frame, selection := Size{Width: 300, Height: 200}, Rect{Width: 300, Height: 200}
		output, err := renderScreenshotEditorAnnotations(source, []screenshotEditorAnnotation{annotation}, selection, frame, scale)
		if err != nil {
			t.Fatal(err)
		}
		want, err := renderScreenshotEditorAnnotations(source, lines, selection, frame, scale)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(output.Pix, want.Pix) {
			t.Fatalf("scale %v: exported lines overlap or differ from their preview positions", scale)
		}
	}
}

// TestScreenshotEditorCaretHitUsesRenderedAdvances covers narrow/wide glyphs and kerning in a proportional font.
func TestScreenshotEditorCaretHitUsesRenderedAdvances(t *testing.T) {
	surface := &screenshotAnnotationTextSurface{}
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		for index, advance := range []float32{0, 24, 27, 41, 51, 69} {
			if got := screenshotEditorTextCaretIndex(surface, "WiAV中", Point{X: advance * scale}, 20*scale); got != index {
				t.Fatalf("scale %v: caret at rendered advance %v = %d, want %d", scale, advance*scale, got, index)
			}
		}
	}
}

// TestScreenshotEditorCaretDoesNotAccumulateWidthError covers long labels at fractional display scales and logical origins.
func TestScreenshotEditorCaretDoesNotAccumulateWidthError(t *testing.T) {
	surface := &screenshotAnnotationTextSurface{}
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		for _, repetitions := range []int{1, 20, 100} {
			text := strings.Repeat("WiAV中", repetitions)
			for _, origin := range []Point{{X: 40.5, Y: 60.25}, {X: -800.5, Y: -300.25}} {
				caret := screenshotEditorTextCaretRect(surface, origin, text, 20, scale)
				wantX := origin.X + 69*float32(repetitions)*scale
				if math.Abs(float64(caret.X-wantX)) > 0.01 || caret.Y != origin.Y || caret.Width != 1.5*scale || caret.Height != 24*scale {
					t.Fatalf("scale %v, length %d: caret = %+v, want x=%v with logical origin %+v", scale, len([]rune(text)), caret, wantX, origin)
				}
			}
		}
	}
}

// TestScreenshotEditorSelectionUsesRenderedAdvances checks reversed selections against the rendered prefix positions.
func TestScreenshotEditorSelectionUsesRenderedAdvances(t *testing.T) {
	surface := &screenshotAnnotationTextSurface{}
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		actual, expected := &DisplayList{}, &DisplayList{}
		drawScreenshotEditorTextSelection(actual, surface, Point{X: 40, Y: 60}, "WiAV中", TextSelection{Anchor: 4, Focus: 1}, 20*scale, screenshotEditorAnnotationColor, scale)
		highlight := screenshotEditorAnnotationColor
		highlight.A = 70
		expected.FillRect(Rect{X: 40 + 24*scale, Y: 60, Width: 27 * scale, Height: 24 * scale}, highlight)
		if err := actual.Compare(expected); err != nil {
			t.Fatalf("scale %v: selection does not match glyph positions: %v", scale, err)
		}
	}
}

// TestScreenshotEditorCompositionCaretUsesReplacementPrefix keeps IME anchoring aligned when composition replaces selected text.
func TestScreenshotEditorCompositionCaretUsesReplacementPrefix(t *testing.T) {
	editor := NewTextEditor("WiAV中")
	editor.SetSelection(1, 4)
	editor.HandleTextInput(TextInputEvent{Kind: TextInputCompose, Text: "文字"})
	preview, prefix := screenshotEditorTextEditingPreview(editor.State())
	if preview != "W文字中" || prefix != "W文字" {
		t.Fatalf("IME preview = %q, caret prefix = %q", preview, prefix)
	}
	caret := screenshotEditorTextCaretRect(&screenshotAnnotationTextSurface{}, Point{X: 40, Y: 60}, prefix, 20, 1.5)
	if caret.X != 130 {
		t.Fatalf("IME caret x = %v, want 130", caret.X)
	}
}

// TestScreenshotEditorCaretFallsBackWithoutNativeMetrics preserves headless editing and empty labels.
func TestScreenshotEditorCaretFallsBackWithoutNativeMetrics(t *testing.T) {
	text := "WiAV中"
	if got, want := screenshotEditorTextWidth(&Window{}, text, 20), screenshotEditorEstimatedTextWidth(text, 20); got != want {
		t.Fatalf("unavailable native metrics: width = %v, want fallback %v", got, want)
	}
	if got := screenshotEditorTextWidth(&screenshotAnnotationTextSurface{}, "", 20); got != 0 {
		t.Fatalf("empty prefix width = %v, want 0", got)
	}
}

// TestScreenshotEditorLeavesCompositionKeysToIME prevents candidate navigation from editing or committing the annotation.
func TestScreenshotEditorLeavesCompositionKeysToIME(t *testing.T) {
	for _, key := range []Key{KeyEnter, KeyEscape, KeyBackspace, KeyDelete, KeyArrowLeft, KeyArrowRight, KeyArrowUp, KeyArrowDown, KeyHome, KeyEnd, Key(" "), Key("1")} {
		t.Run(string(key), func(t *testing.T) {
			state := &screenshotEditorOverlayState{textEditing: true, textFontSize: 20, uiScale: 1}
			state.resetTextEditorLocked("文字")
			state.textInput(TextInputEvent{Kind: TextInputCompose, Text: "ni"})
			before := state.textEditor.State()
			if state.key(KeyEvent{Key: key, Down: true, Composing: true}) {
				t.Fatalf("composition key %q was consumed before reaching the native input method", key)
			}
			if !state.textEditing || state.textEditor == nil || state.textEditor.State() != before || state.textDraft != before.Text || state.textMarked != before.Composition || state.textCaret != before.Selection.Focus || len(state.annotations) != 0 {
				t.Fatalf("composition key %q changed the annotation editor", key)
			}
			state.textInput(TextInputEvent{Kind: TextInputCommit, Text: "你"})
			if state.textDraft != "文字你" || state.textMarked != "" || !state.textEditing {
				t.Fatalf("candidate commit failed: text=%q marked=%q editing=%t", state.textDraft, state.textMarked, state.textEditing)
			}
			if !state.key(KeyEvent{Key: KeyEnter, Down: true}) || state.textEditing || len(state.annotations) != 1 || state.annotations[0].text != "文字你" {
				t.Fatal("Enter did not finish the annotation after composition ended")
			}
		})
	}
}

// screenshotMixedScriptTextSurface models the different CoreText baselines for Chinese, Latin, and emoji fallback runs.
type screenshotMixedScriptTextSurface struct {
	*screenshotAnnotationTextSurface
}

// MeasureText varies ascent and descent with the fallback fonts used by the line.
func (surface *screenshotMixedScriptTextSurface) MeasureText(text string, style TextStyle) (woxui.TextMetrics, error) {
	metrics, err := surface.screenshotAnnotationTextSurface.MeasureText(text, style)
	metrics.Baseline = style.Size * 0.86
	metrics.Size.Height = style.Size
	if strings.ContainsAny(text, "ani") {
		metrics.Baseline = style.Size * 0.967
		metrics.Size.Height = style.Size * 1.178
	} else if strings.Contains(text, "🙂") {
		metrics.Baseline = style.Size * 0.966
		metrics.Size.Height = style.Size * 1.177
	}
	return metrics, err
}

// TestScreenshotEditorMixedScriptKeepsBaseline fixes the native baseline in logical coordinates through typing and IME commits.
func TestScreenshotEditorMixedScriptKeepsBaseline(t *testing.T) {
	surface := &screenshotMixedScriptTextSurface{screenshotAnnotationTextSurface: &screenshotAnnotationTextSurface{}}
	for _, scale := range []float32{1, 1.25, 1.5, 2, 1} {
		for _, origin := range []Point{{X: 40, Y: 60}, {X: -800.5, Y: -300.25}} {
			for _, text := range []string{"中文", "中文a", "中文ni", "中文你", "中文🙂", "a中文", "中文\n中文a\n中文你"} {
				annotation := screenshotEditorAnnotation{tool: screenshotEditorToolText, text: text, start: origin, fontSize: 20}
				actual, expected := &DisplayList{}, &DisplayList{}
				drawScreenshotEditorAnnotations(actual, surface, []screenshotEditorAnnotation{annotation}, nil, Size{}, scale)
				style := TextStyle{Size: 20 * scale, Weight: FontWeightSemibold}
				for row, line := range strings.Split(text, "\n") {
					metrics, err := surface.MeasureText(line, style)
					if err != nil {
						t.Fatal(err)
					}
					// Native DrawText adds this line's measured baseline to rect.Y.
					baselineY := origin.Y + float32(row)*28*scale + 20*scale
					expected.DrawText(line, Rect{
						X: origin.X, Y: baselineY - metrics.Baseline, Width: 480, Height: max(style.Size+12, 28*scale),
					}, style, screenshotEditorAnnotationColor)
				}
				if err := actual.Compare(expected); err != nil {
					t.Fatalf("scale %v, origin %+v, text %q: changed the anchored baseline: %v", scale, origin, text, err)
				}
			}
		}
	}
}
