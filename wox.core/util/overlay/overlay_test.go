package overlay

import (
	"testing"

	woxui "wox/ui/runtime"
)

func TestBoundsMovesUpWhenGrowthReachesWorkAreaBottom(t *testing.T) {
	workArea := woxui.Rect{Width: 1200, Height: 900}
	bounds := Bounds(WindowOptions{AbsolutePosition: true, Anchor: AnchorTopLeft, OffsetX: 300, OffsetY: 700}, woxui.Rect{}, workArea, woxui.Size{Width: 420, Height: 600})
	if bounds.X != 300 || bounds.Y != 300 {
		t.Fatalf("clamped origin = (%v, %v), want (300, 300)", bounds.X, bounds.Y)
	}
}

func TestBoundsPreservesPositionUntilClampingIsNeeded(t *testing.T) {
	workArea := woxui.Rect{X: -1000, Width: 1000, Height: 800}
	current := woxui.Rect{X: -900, Y: 120, Width: 300, Height: 100}
	bounds := Bounds(WindowOptions{PreservePosition: true}, current, workArea, woxui.Size{Width: 420, Height: 760})
	if bounds.X != -900 || bounds.Y != 40 {
		t.Fatalf("preserved and clamped origin = (%v, %v), want (-900, 40)", bounds.X, bounds.Y)
	}
}

func TestRequestCloseFiresCallbackOnce(t *testing.T) {
	called := 0
	RegisterCloseCallback("test", func() { called++ })
	if callback := takeCloseCallback("test"); callback != nil {
		callback()
	}
	if callback := takeCloseCallback("test"); callback != nil {
		callback()
	}
	if called != 1 {
		t.Fatalf("close callback count = %d, want 1", called)
	}
}

func TestScaledBoundsPreservesCenterAndAspectRatio(t *testing.T) {
	current := woxui.Rect{X: 300, Y: 200, Width: 400, Height: 250}
	target := scaledBounds(current, woxui.Rect{Width: 1200, Height: 900}, 1.25, 1.6, woxui.Size{Width: 180, Height: 120})
	if target != (woxui.Rect{X: 250, Y: 168.75, Width: 500, Height: 312.5}) {
		t.Fatalf("scaled bounds = %+v", target)
	}
}
