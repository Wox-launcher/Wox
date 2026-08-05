//go:build darwin

package woxui

import "testing"

func TestDarwinQueueFramePreservesReplacedNativeDamage(t *testing.T) {
	window := &platformWindow{}
	frame := func(damage Rect) *darwinRenderFrame {
		displayList := &DisplayList{}
		displayList.SetNativeDamage(damage)
		return &darwinRenderFrame{displayList: displayList}
	}

	window.queueFrame(frame(Rect{X: 10, Y: 10, Width: 20, Height: 20}))
	partial := frame(Rect{X: 30, Y: 10, Width: 20, Height: 20})
	window.queueFrame(partial)
	if got, want := partial.displayList.NativeDamage(), (Rect{X: 10, Y: 10, Width: 40, Height: 20}); got != want {
		t.Fatalf("coalesced native damage = %+v, want %+v", got, want)
	}

	full := frame(Rect{})
	window.queueFrame(full)
	partialAfterFull := frame(Rect{X: 40, Y: 40, Width: 10, Height: 10})
	window.queueFrame(partialAfterFull)
	if got := partialAfterFull.displayList.NativeDamage(); got != (Rect{}) {
		t.Fatalf("native damage after replaced full frame = %+v, want full frame", got)
	}
}
