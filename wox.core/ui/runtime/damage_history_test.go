package woxui

import "testing"

func TestBufferDamageHistoryUsesPreviousFrameForAgeTwo(t *testing.T) {
	history := bufferDamageHistory{}
	first := Rect{X: 10, Y: 10, Width: 10, Height: 10}
	second := Rect{X: 30, Y: 30, Width: 10, Height: 10}
	third := Rect{X: 50, Y: 50, Width: 10, Height: 10}
	if got := history.accumulate(first, false); got != (Rect{}) {
		t.Fatalf("first damage = %+v, want full frame", got)
	}
	if got, want := history.accumulate(second, false), (Rect{X: 10, Y: 10, Width: 30, Height: 30}); got != want {
		t.Fatalf("second damage = %+v, want %+v", got, want)
	}
	if got, want := history.accumulate(third, false), (Rect{X: 30, Y: 30, Width: 30, Height: 30}); got != want {
		t.Fatalf("third damage = %+v, want %+v", got, want)
	}
}

func TestBufferDamageHistoryFallsBackAfterFullOrReset(t *testing.T) {
	history := bufferDamageHistory{}
	history.accumulate(Rect{X: 10, Y: 10, Width: 10, Height: 10}, false)
	if got := history.accumulate(Rect{}, true); got != (Rect{}) {
		t.Fatalf("full damage = %+v, want full frame", got)
	}
	if got := history.accumulate(Rect{X: 20, Y: 20, Width: 10, Height: 10}, false); got != (Rect{}) {
		t.Fatalf("damage after full frame = %+v, want full frame", got)
	}
	history.reset()
	if got := history.accumulate(Rect{X: 30, Y: 30, Width: 10, Height: 10}, false); got != (Rect{}) {
		t.Fatalf("damage after reset = %+v, want full frame", got)
	}
}
