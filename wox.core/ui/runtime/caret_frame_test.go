package woxui

import "testing"

// TestCaretPatchThroughWindowOutline distinguishes a hollow outline from a filled occluder.
func TestCaretPatchThroughWindowOutline(t *testing.T) {
	for _, test := range []struct {
		name   string
		caret  Rect
		radius float32
		patch  bool
	}{
		{"square window", Rect{X: 20, Y: 50, Width: 2, Height: 24}, 0, true},
		{"rounded window", Rect{X: 20, Y: 50, Width: 2, Height: 24}, 30, true},
		{"on edge", Rect{X: 0, Y: 50, Width: 2, Height: 24}, 0, false},
		{"rounded corner", Rect{X: 5, Y: 5, Width: 2, Height: 24}, 30, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			scene := func(visible bool) *DisplayList {
				d := &DisplayList{}
				d.FillRect(Rect{Width: 200, Height: 120}, Color{A: 255})
				d.DrawCaret(test.caret, Color{R: 255, A: 255}, visible)
				d.StrokeRoundedRect(Rect{Width: 200, Height: 120}, test.radius, 2, Color{G: 255, A: 255})
				return d
			}
			previous := scene(true).PrepareCaretPatch(nil, false)
			previous = scene(true).PrepareCaretPatch(previous, true)
			next := scene(false)
			next.PrepareCaretPatch(previous, true)
			if got := next.CaretPatch(); (got == test.caret) != test.patch {
				t.Fatalf("patch=%+v, want enabled=%v", got, test.patch)
			}
		})
	}
}

// TestCaretPatchRequiresAnUnchangedScene exercises invalidation and occlusion rather than timer timing.
func TestCaretPatchRequiresAnUnchangedScene(t *testing.T) {
	caret := Rect{X: 20.25, Y: 12.5, Width: 2, Height: 24}
	scene := func(visible bool) *DisplayList {
		d := &DisplayList{}
		d.FillRect(Rect{Width: 200, Height: 100}, Color{R: 40, A: 90})
		d.DrawCaret(caret, Color{R: 255, A: 180}, visible)
		return d
	}
	for _, test := range []struct {
		name   string
		change func(*DisplayList)
		patch  bool
	}{
		{"blink", func(*DisplayList) {}, true},
		{"background", func(d *DisplayList) { d.commands[0].color.R++ }, false},
		{"caret position", func(d *DisplayList) { d.commands[1].rect.X++ }, false},
		{"caret color", func(d *DisplayList) { d.commands[1].color.A++ }, false},
		{"clip", func(d *DisplayList) { d.PushClipRect(caret) }, false},
		{"occlusion", func(d *DisplayList) { d.FillRect(caret, Color{A: 255}) }, false},
		{"blur sampling", func(d *DisplayList) {
			d.FloatingMaterial(Rect{X: 24, Y: 12, Width: 50, Height: 40}, 3, Color{A: 80}, Color{})
		}, false},
		{"multiple carets", func(d *DisplayList) { d.DrawCaret(Rect{X: 80, Y: 12, Width: 2, Height: 24}, Color{A: 255}, false) }, false},
		{"culled scene", func(d *DisplayList) { d.SetDamage(caret) }, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			previous := scene(true).PrepareCaretPatch(nil, false)
			previous = scene(true).PrepareCaretPatch(previous, true)
			next := scene(false)
			test.change(next)
			next.PrepareCaretPatch(previous, true)
			if got := next.CaretPatch(); (got == caret) != test.patch {
				t.Fatalf("patch = %+v, expected patch %v", got, test.patch)
			}
		})
	}
	// A full/native exposure frame must repaint even when its scene is identical.
	next := scene(false)
	next.PrepareCaretPatch(scene(true).PrepareCaretPatch(nil, false), false)
	if next.CaretPatch() != (Rect{}) {
		t.Fatal("full frame was narrowed")
	}
	previous := scene(true).PrepareCaretPatch(nil, false)
	firstBlink := scene(false)
	previous = firstBlink.PrepareCaretPatch(previous, true)
	if firstBlink.CaretPatch() != (Rect{}) {
		t.Fatal("the second swap-chain buffer was not given a normal repaint")
	}
	secondBlink := scene(true)
	secondBlink.PrepareCaretPatch(previous, true)
	if secondBlink.CaretPatch() != caret {
		t.Fatal("stable swap-chain buffers did not enable the caret patch")
	}
}

// TestHiddenCaretHasNoSoftwarePixels verifies the portable fallback still hides the caret.
func TestHiddenCaretHasNoSoftwarePixels(t *testing.T) {
	for _, visible := range []bool{true, false} {
		var actual, expected DisplayList
		color := Color{R: 240, G: 100, A: 190}
		rect := Rect{X: 4, Y: 3, Width: 2, Height: 8}
		actual.DrawCaret(rect, color, visible)
		if visible {
			expected.FillRect(rect, color)
		}
		a, _ := NewSoftwareRenderer(16, 16)
		b, _ := NewSoftwareRenderer(16, 16)
		if err := a.Render(&actual); err != nil {
			t.Fatal(err)
		}
		if err := b.Render(&expected); err != nil {
			t.Fatal(err)
		}
		for index, pixel := range a.pixels.Pix {
			if pixel != b.pixels.Pix[index] {
				t.Fatalf("visible=%v pixel %d differs", visible, index)
			}
		}
	}
}
