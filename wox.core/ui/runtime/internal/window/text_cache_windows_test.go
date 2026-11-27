//go:build windows

package window

import (
	"bytes"
	"fmt"
	"testing"
)

// renderTextFrame renders the list and returns the frame's resource counters and pixels.
func renderTextFrame(t *testing.T, r *nativeRenderer, d *DisplayList) (FrameRendererResourceMetrics, []byte) {
	t.Helper()
	if err := r.render(d, 1.5); err != nil {
		t.Fatal(err)
	}
	stats := r.resourceStats()
	pixels, err := readWindowsOffscreenPixels(r, 256, 96)
	if err != nil {
		t.Fatal(err)
	}
	return stats, pixels
}

// TestWindowsTextLayoutCache proves repeated labels skip shaping without changing pixels, and that
// the cache is released by image-cache clearing and font changes.
func TestWindowsTextLayoutCache(t *testing.T) {
	r, err := newWindowsOffscreenRenderer(256, 96)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(r.destroy)

	d := &DisplayList{}
	d.Clear(Color{R: 255, G: 255, B: 255, A: 255})
	const label = "Hello 世界 日本語 123"
	d.DrawText(label, Rect{X: 4, Y: 4, Width: 240, Height: 24}, TextStyle{Size: 16}, Color{A: 255})
	d.DrawText(label, Rect{X: 4, Y: 36, Width: 240, Height: 24}, TextStyle{Size: 16, Weight: FontWeightSemibold}, Color{A: 255})

	first, firstPixels := renderTextFrame(t, r, d)
	if first.TextRasterizations != 2 {
		t.Fatalf("first frame text rasterizations = %d, want 2", first.TextRasterizations)
	}
	second, secondPixels := renderTextFrame(t, r, d)
	if second.TextRasterizations != 0 {
		t.Fatalf("second frame text rasterizations = %d, want 0 (layout cache hit)", second.TextRasterizations)
	}
	if !bytes.Equal(firstPixels, secondPixels) {
		t.Fatal("cached text layouts rendered different pixels than fresh layouts")
	}

	if err := r.clearImageCache(); err != nil {
		t.Fatal(err)
	}
	if cleared, _ := renderTextFrame(t, r, d); cleared.TextRasterizations != 2 {
		t.Fatalf("frame after clearImageCache rasterizations = %d, want 2", cleared.TextRasterizations)
	}
	if rewarmed, _ := renderTextFrame(t, r, d); rewarmed.TextRasterizations != 0 {
		t.Fatalf("rewarmed frame rasterizations = %d, want 0", rewarmed.TextRasterizations)
	}

	if err := r.setFontFamily("Consolas"); err != nil {
		t.Fatal(err)
	}
	if changed, _ := renderTextFrame(t, r, d); changed.TextRasterizations != 2 {
		t.Fatalf("frame after setFontFamily rasterizations = %d, want 2", changed.TextRasterizations)
	}
}

// TestWindowsTextLayoutCacheEvictsBeyondBound renders more distinct labels than the cache holds in
// one frame, covering eviction while the frame is open and release when it ends.
func TestWindowsTextLayoutCacheEvictsBeyondBound(t *testing.T) {
	r, err := newWindowsOffscreenRenderer(256, 96)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(r.destroy)

	d := &DisplayList{}
	d.Clear(Color{R: 255, G: 255, B: 255, A: 255})
	for index := 0; index < 300; index++ {
		d.DrawText(fmt.Sprintf("label %03d", index), Rect{X: 4, Y: float32(index%4) * 20, Width: 200, Height: 18}, TextStyle{Size: 12}, Color{A: 255})
	}
	stats, _ := renderTextFrame(t, r, d)
	if stats.TextRasterizations != 300 {
		t.Fatalf("text rasterizations = %d, want 300 distinct layouts", stats.TextRasterizations)
	}
	for frame := 0; frame < 3; frame++ {
		if err := r.render(d, 1.5); err != nil {
			t.Fatalf("frame %d after eviction: %v", frame, err)
		}
	}
}

// TestWindowsClearRendererCacheCommandChecksFocusEpoch keeps a delayed clear from an earlier hide
// from wiping the warm cache of a later hide.
func TestWindowsClearRendererCacheCommandChecksFocusEpoch(t *testing.T) {
	r, err := newWindowsOffscreenRenderer(256, 96)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(r.destroy)

	d := &DisplayList{}
	d.Clear(Color{R: 255, G: 255, B: 255, A: 255})
	d.DrawText("epoch label", Rect{X: 4, Y: 4, Width: 240, Height: 24}, TextStyle{Size: 16}, Color{A: 255})
	if stats, _ := renderTextFrame(t, r, d); stats.TextRasterizations != 1 {
		t.Fatalf("warmup rasterizations = %d, want 1", stats.TextRasterizations)
	}

	w := &platformWindow{renderer: r, focus: focusRuntime{epoch: 2}}
	steps := []struct {
		name    string
		visible bool
		epoch   FocusEpoch
		want    int
	}{
		{"stale epoch keeps cache", false, 1, 0},
		{"visible window keeps cache", true, 2, 0},
		{"matching hidden epoch clears cache", false, 2, 1},
	}
	for _, step := range steps {
		w.focus.visible = step.visible
		if result := w.executeCommand(windowCommand{kind: windowCommandClearRendererCache, epoch: step.epoch}); result.err != nil {
			t.Fatalf("%s: %v", step.name, result.err)
		}
		if stats, _ := renderTextFrame(t, r, d); stats.TextRasterizations != step.want {
			t.Fatalf("%s: rasterizations = %d, want %d", step.name, stats.TextRasterizations, step.want)
		}
	}
}
