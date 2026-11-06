package component

import (
	"runtime"
	"testing"

	ui "wox/ui/runtime"
	"wox/ui/widget"
)

// richTextServices makes font traits affect actual geometry rather than merely checking props.
type richTextServices struct{ hotkeyRecorderHostServices }

func (*richTextServices) MeasureText(text string, style ui.TextStyle) (ui.TextMetrics, error) {
	factor := float32(0.5)
	if style.Family == ui.FontFamilyMonospace {
		factor = 0.7
	}
	if style.Weight == ui.FontWeightSemibold {
		factor += 0.1
	}
	return ui.TextMetrics{Size: ui.Size{Width: float32(len([]rune(text))) * style.Size * factor, Height: style.Size}, Baseline: style.Size * 0.8}, nil
}

// TestRichTextLayoutPreservesGraphemesAndDensity checks mixed fonts, line breaks, and exact backing text.
func TestRichTextLayoutPreservesGraphemesAndDensity(t *testing.T) {
	measure := &richTextServices{}
	for i, density := range []float32{0.9, 1, 1.1} {
		p := RichTextProps{Style: ui.TextStyle{Size: 14}, Theme: ControlTheme{DensityScale: density}, Runs: []RichTextRun{
			{ID: "plain", Text: "Hello ", Style: ui.TextStyle{Size: 14}},
			{ID: "code", Text: "world", Style: ui.TextStyle{Size: 14, Family: ui.FontFamilyMonospace}},
			{ID: "unicode", Text: "\né 👩‍👩‍👧‍👦 中文\n", Style: ui.TextStyle{Size: 14, Weight: ui.FontWeightSemibold, Italic: true}},
		}}
		layout := layoutRichText(p, ui.Size{Width: 70, Height: 500}, measure)
		if layout.value != "Hello world\né 👩‍👩‍👧‍👦 中文\n" || len(layout.lines) < 4 {
			t.Fatalf("density %v: lost whitespace/wrapping: %+v", density, layout)
		}
		boundaries := map[int]bool{0: true}
		for _, span := range ui.GraphemeSpans(layout.value) {
			boundaries[span.Start], boundaries[span.End] = true, true
		}
		for _, line := range layout.lines {
			if !boundaries[line.start] || !boundaries[line.end] {
				t.Fatalf("line split a grapheme: %+v", line)
			}
		}
		wantSize := []float32{13, 14, 15}[i]
		if layout.runs[1].Style.Family != ui.FontFamilyMonospace || !layout.runs[2].Style.Italic || layout.runs[1].Style.Size != wantSize || layout.lineHeight < wantSize*1.49 {
			t.Fatalf("font traits/density: %+v", layout)
		}
	}
}

// TestTextFieldHitWithoutMeasurer preserves headless hover and inline-decoration behavior.
func TestTextFieldHitWithoutMeasurer(t *testing.T) {
	state := ui.TextEditingState{Text: "abc"}
	if got := textFieldOffsetAt(state, nil, ui.TextStyle{Size: 14}, nil, 8, 21, 0, 100, true, false, ui.Point{X: 5, Y: 5}); got != 3 {
		t.Fatalf("unmeasured text offset = %d", got)
	}
	if got := textFieldMeasureRange(nil, []rune("abc"), 0, 3, ui.TextStyle{Size: 14}, []TextFieldRichRun{{Start: 0, End: 1, Advance: 12}}); got != 12 {
		t.Fatalf("unmeasured inline advance = %v", got)
	}
}

// TestFixedReadOnlyViewportDoesNotFollowCaret protects externally positioned inline link regions.
func TestFixedReadOnlyViewportDoesNotFollowCaret(t *testing.T) {
	p := TextFieldProps{ID: "fixed", Value: "first line\nsecond line\nthird line", MaxLines: 8, Width: 90, Height: 22,
		LineHeight: 21, Style: ui.TextStyle{Size: 14}, Padding: widget.Insets{Bottom: 1}, ReadOnly: true, FixedViewport: true, Measurer: &richTextServices{}}
	s := &textFieldState{}
	s.InitState(widget.StateContext{}, p)
	defer s.Dispose()
	s.controller.SelectAll()
	s.Build(widget.StateContext{}, p)
	if s.verticalOffset != 0 {
		t.Fatal("read-only selection scrolled text away from overlays")
	}
	state := s.controller.State()
	caret := textFieldCursorRect(state, p.Style, nil, 8, 21, 0, ui.Rect{Width: 90, Height: 21}, true, p.Measurer)
	if caret.X < 0 || caret.X > 90 {
		t.Fatalf("wrapped one-line viewport scrolled horizontally: %+v", caret)
	}
}

// TestRichTextWrappedLinkHasOneFocusTarget checks each fragment, keyboard order, clipping, and scale transitions.
func TestRichTextWrappedLinkHasOneFocusTarget(t *testing.T) {
	measure := &richTextServices{}
	pressed := 0
	p := RichTextProps{ID: "rich", Style: ui.TextStyle{Size: 14}, Runs: []RichTextRun{
		{ID: "prefix", Text: "Hi ", Style: ui.TextStyle{Size: 14}},
		{ID: "first", Text: "very long link across many lines", Style: ui.TextStyle{Size: 14}, Underline: true, OnPress: func() { pressed++ }},
		{ID: "second", Text: "next", Style: ui.TextStyle{Size: 14}, OnPress: func() { pressed += 100 }},
	}}
	host := widget.NewHost(func(ui.FrameInfo) widget.Widget { return WoxRichText(p) })
	host.AttachServices(measure)
	defer host.Dispose()
	for _, scale := range []float32{1, 1.5, 2, 1} {
		host.Frame(&ui.DisplayList{}, ui.FrameInfo{Size: ui.Size{Width: 75, Height: 400}, PixelSize: ui.PixelSize{Width: int(75 * scale), Height: int(400 * scale)}, Scale: scale})
		layout := layoutRichText(p, ui.Size{Width: 75, Height: 400}, measure)
		if len(layout.fragments[1]) < 2 {
			t.Fatal("link did not wrap")
		}
		before := pressed
		for _, f := range layout.fragments[1] {
			point := ui.Point{X: f.bounds.X + f.bounds.Width/2, Y: f.bounds.Y + f.bounds.Height/2}
			host.Pointer(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, Position: point})
			host.Pointer(ui.PointerEvent{Kind: ui.PointerUp, Button: ui.PointerButtonPrimary, Position: point})
		}
		if pressed != before+len(layout.fragments[1]) {
			t.Fatalf("scale %v: wrapped fragments activated %d times", scale, pressed-before)
		}
		host.ClearFocus()
		host.Key(ui.KeyEvent{Key: ui.KeyTab, Down: true})
		if host.FocusedKey() != "first" {
			t.Fatalf("first inline focus = %s", host.FocusedKey())
		}
		host.Key(ui.KeyEvent{Key: ui.KeyEnter, Down: true})
		host.Key(ui.KeyEvent{Key: ui.KeyEnter, Down: true, Repeat: true})
		if pressed != before+len(layout.fragments[1])+1 {
			t.Fatal("Enter repeat activated link twice")
		}
		host.Key(ui.KeyEvent{Key: ui.KeyTab, Down: true})
		if host.FocusedKey() != "second" {
			t.Fatal("a wrapped link created multiple Tab stops")
		}
	}
	p.MaxLines = 1
	host.Frame(&ui.DisplayList{}, ui.FrameInfo{Size: ui.Size{Width: 75, Height: 400}, Scale: 1})
	if host.FocusAutomationID("second") {
		t.Fatal("truncated link remained focusable")
	}
}

// TestRichTextDragAcrossStyles exercises selection hit testing with different font advances.
func TestRichTextDragAcrossStyles(t *testing.T) {
	clipboard := &memoryClipboard{}
	SetClipboardProvider(clipboard)
	defer SetClipboardProvider(nil)
	measure := &richTextServices{}
	pressed := 0
	p := RichTextProps{ID: "rich", Style: ui.TextStyle{Size: 14}, Selectable: true, Runs: []RichTextRun{
		{ID: "plain", Text: "ab ", Style: ui.TextStyle{Size: 14}},
		{ID: "link", Text: "中文", Style: ui.TextStyle{Size: 14, Family: ui.FontFamilyMonospace}, OnPress: func() { pressed++ }},
		{ID: "tail", Text: " xy", Style: ui.TextStyle{Size: 14, Weight: ui.FontWeightSemibold}},
	}}
	host := widget.NewHost(func(ui.FrameInfo) widget.Widget { return WoxRichText(p) })
	host.AttachServices(measure)
	defer host.Dispose()
	for _, scale := range []float32{1, 1.5, 2} {
		host.Frame(&ui.DisplayList{}, ui.FrameInfo{Size: ui.Size{Width: 180, Height: 100}, PixelSize: ui.PixelSize{Width: int(180 * scale), Height: int(100 * scale)}, Scale: scale})
		layout := layoutRichText(p, ui.Size{Width: 180, Height: 100}, measure)
		end := layout.fragments[2][0].bounds
		from := ui.Point{X: 7, Y: 10}
		to := ui.Point{X: end.X + end.Width, Y: 10}
		host.Pointer(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, Position: from})
		host.Pointer(ui.PointerEvent{Kind: ui.PointerMove, Position: to})
		host.Pointer(ui.PointerEvent{Kind: ui.PointerUp, Button: ui.PointerButtonPrimary, Position: to})
		modifier := ui.KeyModifierControl
		if runtime.GOOS == "darwin" {
			modifier = ui.KeyModifierMeta
		}
		host.Key(ui.KeyEvent{Key: "c", Modifiers: modifier, Down: true})
		if clipboard.text != "b 中文 xy" || pressed != 0 {
			t.Fatalf("scale %v: drag copied %q and activated %d links", scale, clipboard.text, pressed)
		}
	}
}

// TestSelectableRichTextCopyAndFixedViewport keeps centered links aligned after selection and wheel input.
func TestSelectableRichTextCopyAndFixedViewport(t *testing.T) {
	clipboard := &memoryClipboard{}
	SetClipboardProvider(clipboard)
	defer SetClipboardProvider(nil)
	measure := &richTextServices{}
	pressed := 0
	p := RichTextProps{ID: "rich", Style: ui.TextStyle{Size: 14}, Selectable: true, Centered: true, MaxLines: 1, Runs: []RichTextRun{
		{ID: "prefix", Text: "A ", Style: ui.TextStyle{Size: 14}},
		{ID: "link", Text: "link", Style: ui.TextStyle{Size: 14}, Underline: true, OnPress: func() { pressed++ }},
		{ID: "tail", Text: "\nhidden 世界", Style: ui.TextStyle{Size: 14}},
	}}
	host := widget.NewHost(func(ui.FrameInfo) widget.Widget { return WoxRichText(p) })
	host.AttachServices(measure)
	defer host.Dispose()
	render := func() { host.Frame(&ui.DisplayList{}, ui.FrameInfo{Size: ui.Size{Width: 140, Height: 100}, Scale: 2}) }
	render()
	if err := host.SelectAllWithin("rich/selectable"); err != nil {
		t.Fatal(err)
	}
	render()
	modifier := ui.KeyModifierControl
	if runtime.GOOS == "darwin" {
		modifier = ui.KeyModifierMeta
	}
	host.Key(ui.KeyEvent{Key: "c", Modifiers: modifier, Down: true})
	if clipboard.text != "A link\nhidden 世界" {
		t.Fatalf("rich copy changed bytes: %q", clipboard.text)
	}
	layout := layoutRichText(p, ui.Size{Width: 140, Height: 100}, measure)
	lines := textFieldRichLines(layout.value, measure, p.Style, 140, true, layout.runs)
	if lines[0].indent <= 0 || lines[0].indent != layout.fragments[0][0].bounds.X {
		t.Fatal("selectable centering differs from link geometry")
	}
	host.Pointer(ui.PointerEvent{Kind: ui.PointerScroll, Position: ui.Point{X: 10, Y: 10}, Scroll: ui.Point{Y: -200}})
	render()
	f := layout.fragments[1][0].bounds
	point := ui.Point{X: f.X + f.Width/2, Y: f.Y + f.Height/2}
	host.Pointer(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, Position: point})
	host.Pointer(ui.PointerEvent{Kind: ui.PointerUp, Button: ui.PointerButtonPrimary, Position: point})
	if pressed != 1 {
		t.Fatal("link geometry drifted after selecting clipped text")
	}
}
