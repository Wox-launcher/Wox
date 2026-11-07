package window

import "testing"

// TestWindowsCaretBackdropCacheHit locks the idle blink path. begin_frame clears resource
// counters, so restoring the captured caret backdrop has to record a cache hit itself.
func TestWindowsCaretBackdropCacheHit(t *testing.T) {
	renderer, err := newWindowsOffscreenRenderer(64, 64)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(renderer.destroy)
	caret := Rect{X: 20, Y: 12, Width: 2, Height: 24}
	scene := func(visible bool) *DisplayList {
		displayList := &DisplayList{}
		displayList.FillRect(Rect{Width: 64, Height: 64}, Color{R: 20, G: 40, B: 80, A: 255})
		displayList.DrawCaret(caret, Color{R: 255, A: 255}, visible)
		return displayList
	}
	var previous, patched *DisplayList
	for _, visible := range []bool{true, true, false} {
		patched = scene(visible)
		previous = patched.PrepareCaretPatch(previous, true)
		if err := renderer.render(patched, 1); err != nil {
			t.Fatal(err)
		}
	}
	if patched.CaretPatch() != caret {
		t.Fatalf("caret patch = %+v", patched.CaretPatch())
	}
	if hits := renderer.resourceStats().CacheHits; hits < 1 {
		t.Fatalf("caret backdrop cache hits = %d, want > 0", hits)
	}
}
