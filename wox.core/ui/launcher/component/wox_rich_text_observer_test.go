package component

import (
	"runtime"
	"testing"

	ui "wox/ui/runtime"
	"wox/ui/widget"
)

// TestRichTextObserversPreserveSelection starts dragging on the observed span,
// then crosses another span and copies the unchanged underlying characters.
func TestRichTextObserversPreserveSelection(t *testing.T) {
	clipboard := &memoryClipboard{}
	SetClipboardProvider(clipboard)
	defer SetClipboardProvider(nil)
	measure := &richTextServices{}
	hovered, pressed := false, false
	p := RichTextProps{ID: "rich", Selectable: true, Style: ui.TextStyle{Size: 14}, Runs: []RichTextRun{
		{ID: "first", Text: "ab ", Style: ui.TextStyle{Size: 14}, OnHoverChange: func(v bool) { hovered = v }, OnPressChange: func(v bool) { pressed = v }},
		{ID: "last", Text: "中文 xy", Style: ui.TextStyle{Size: 14, Family: ui.FontFamilyMonospace}},
	}}
	host := widget.NewHost(func(ui.FrameInfo) widget.Widget { return WoxRichText(p) })
	host.AttachServices(measure)
	defer host.Dispose()
	for _, scale := range []float32{1, 1.5, 2} {
		host.Frame(&ui.DisplayList{}, ui.FrameInfo{Size: ui.Size{Width: 180, Height: 100}, Scale: scale})
		layout := layoutRichText(p, ui.Size{Width: 180, Height: 100}, measure)
		last := layout.fragments[1][0].bounds
		from, to := ui.Point{X: 7, Y: 10}, ui.Point{X: last.X + last.Width, Y: 10}
		host.Pointer(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, Position: from})
		if !hovered || !pressed {
			t.Fatal("press without preceding move did not observe the run")
		}
		host.Pointer(ui.PointerEvent{Kind: ui.PointerMove, Position: to})
		if hovered {
			t.Fatal("hover did not leave the first run")
		}
		host.Pointer(ui.PointerEvent{Kind: ui.PointerUp, Button: ui.PointerButtonPrimary, Position: to})
		if pressed {
			t.Fatal("release retained press state")
		}
		modifier := ui.KeyModifierControl
		if runtime.GOOS == "darwin" {
			modifier = ui.KeyModifierMeta
		}
		host.Key(ui.KeyEvent{Key: "c", Modifiers: modifier, Down: true})
		if clipboard.text != "b 中文 xy" {
			t.Fatalf("observer intercepted selection: %q", clipboard.text)
		}
	}
	host.Pointer(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, Position: ui.Point{X: 7, Y: 10}})
	host.SetWindowFocused(false)
	if pressed {
		t.Fatal("window blur retained pressed run")
	}
}

// TestRichTextObserversFollowFragments ignores empty space inside a wrapped
// run's bounding box and refreshes against new wrapping under a still pointer.
func TestRichTextObserversFollowFragments(t *testing.T) {
	measure := &richTextServices{}
	hovered, pressed := false, false
	p := RichTextProps{ID: "rich", Style: ui.TextStyle{Size: 14}, Runs: []RichTextRun{
		{ID: "prefix", Text: "prefix ", Style: ui.TextStyle{Size: 14}},
		{ID: "wrapped", Text: "ab cdefghij", Style: ui.TextStyle{Size: 14}, OnHoverChange: func(v bool) { hovered = v }, OnPressChange: func(v bool) { pressed = v }},
	}}
	width := float32(80)
	host := widget.NewHost(func(ui.FrameInfo) widget.Widget { return WoxRichText(p) })
	host.AttachServices(measure)
	defer host.Dispose()
	frame := func() {
		host.Frame(&ui.DisplayList{}, ui.FrameInfo{Size: ui.Size{Width: width, Height: 100}, Scale: 1.5})
	}
	frame()
	l := layoutRichText(p, ui.Size{Width: width, Height: 100}, measure)
	fragments := l.fragments[1]
	if len(fragments) < 2 || fragments[0].bounds.X == 0 {
		t.Fatalf("fixture does not wrap: %+v", fragments)
	}
	host.Pointer(ui.PointerEvent{Kind: ui.PointerMove, Position: ui.Point{X: 2, Y: 2}})
	if hovered {
		t.Fatal("bounding-box gap acquired hover")
	}
	second := fragments[1].bounds
	point := ui.Point{X: second.X + 2, Y: second.Y + 2}
	host.Pointer(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, Position: point})
	if !hovered || !pressed {
		t.Fatal("second fragment did not share state")
	}
	host.Pointer(ui.PointerEvent{Kind: ui.PointerUp, Button: ui.PointerButtonPrimary, Position: point})
	width = 180
	frame()
	if hovered || pressed {
		t.Fatal("reflow left stale fragment state")
	}
	width = 80
	frame()
	if !hovered {
		t.Fatal("stationary pointer did not reacquire reflowed run")
	}
	host.Pointer(ui.PointerEvent{Kind: ui.PointerLeave})
	if hovered {
		t.Fatal("pointer leave retained hover")
	}
}
