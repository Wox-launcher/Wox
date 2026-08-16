//go:build darwin

package woxui

import "testing"

func TestDarwinQueueFramePreservesReplacedNativeDamage(t *testing.T) {
	window := &platformWindow{}
	frameID := uint64(0)
	frame := func(damage Rect) *darwinRenderFrame {
		frameID++
		displayList := &DisplayList{frameID: frameID}
		displayList.SetNativeDamage(damage)
		return &darwinRenderFrame{displayList: displayList}
	}

	window.queueFrame(frame(Rect{X: 10, Y: 10, Width: 20, Height: 20}))
	partial := frame(Rect{X: 30, Y: 10, Width: 20, Height: 20})
	window.queueFrame(partial)
	if got, want := partial.displayList.NativeDamage(), (Rect{X: 10, Y: 10, Width: 40, Height: 20}); got != want {
		t.Fatalf("coalesced native damage = %+v, want %+v", got, want)
	}
	if partial.coalescedFrameCount != 1 || partial.firstCoalescedFrameID != 1 || partial.lastCoalescedFrameID != 1 {
		t.Fatalf("coalesced frame metadata = %+v, want one replaced frame 1", partial)
	}

	full := frame(Rect{})
	window.queueFrame(full)
	partialAfterFull := frame(Rect{X: 40, Y: 40, Width: 10, Height: 10})
	window.queueFrame(partialAfterFull)
	if got := partialAfterFull.displayList.NativeDamage(); got != (Rect{}) {
		t.Fatalf("native damage after replaced full frame = %+v, want full frame", got)
	}
	if partialAfterFull.coalescedFrameCount != 3 || partialAfterFull.firstCoalescedFrameID != 1 || partialAfterFull.lastCoalescedFrameID != 3 {
		t.Fatalf("accumulated coalesced frame metadata = %+v, want three replacements from 1 through 3", partialAfterFull)
	}
}

func TestDarwinRestoreFrameDamage(t *testing.T) {
	window := &platformWindow{}
	window.restoreFrameDamage(Rect{X: 10, Y: 10, Width: 20, Height: 20})
	window.restoreFrameDamage(Rect{X: 30, Y: 10, Width: 20, Height: 20})
	if got, want := window.consumePendingDamage(), (Rect{X: 10, Y: 10, Width: 40, Height: 20}); got != want {
		t.Fatalf("restored native damage = %+v, want %+v", got, want)
	}

	window.restoreFrameDamage(Rect{})
	window.restoreFrameDamage(Rect{X: 40, Y: 40, Width: 10, Height: 10})
	if !window.damagePending || !window.fullDamage {
		t.Fatalf("restored full native damage was not retained")
	}
	if got := window.consumePendingDamage(); got != (Rect{}) {
		t.Fatalf("restored full native damage = %+v, want full frame", got)
	}
}

func TestDarwinRenderDiagnosticNames(t *testing.T) {
	tests := map[uint8]string{
		darwinRenderDiagnosticWindowUnavailable:  "presentation_window_unavailable",
		darwinRenderDiagnosticRendererReplaced:   "presentation_renderer_replaced",
		darwinRenderDiagnosticGenerationMismatch: "presentation_generation_mismatch",
		darwinRenderDiagnosticStaleSequence:      "presentation_stale_sequence",
		darwinRenderDiagnosticRecovered:          "presentation_recovered",
		99:                                       "presentation_unknown",
	}
	for event, want := range tests {
		if got := darwinRenderDiagnosticName(event); got != want {
			t.Fatalf("diagnostic event %d = %q, want %q", event, got, want)
		}
	}
}

func TestDarwinFrameStatusNames(t *testing.T) {
	if got := darwinFrameStatusName(1); got != "native_skipped" {
		t.Fatalf("skipped status = %q", got)
	}
	if got := darwinFrameStatusName(2); got != "surface_busy" {
		t.Fatalf("surface busy status = %q", got)
	}
	if got := darwinFrameStatusName(-1); got != "native_error" {
		t.Fatalf("error status = %q", got)
	}
}

func TestDarwinRecoverableFrameStatusDoesNotLatchRenderError(t *testing.T) {
	window := &platformWindow{}
	window.recordRenderError("skip macOS frame", 1)
	window.recordRenderError("present macOS frame", 2)
	if window.renderErr != nil {
		t.Fatalf("recoverable frame status latched render error: %v", window.renderErr)
	}
}
