package widget

import (
	"testing"

	woxui "wox/ui/runtime"
)

func TestFrameDamageTrackerIncludesOldAndPlacedBoundaryBounds(t *testing.T) {
	current := &node{bounds: woxui.Rect{Width: 10, Height: 10}}
	tracker := &frameDamageTracker{}
	tracker.add(woxui.Rect{X: 20, Y: 20, Width: 10, Height: 10}, current, true)
	current.place(40, 40)

	got := tracker.resolve(woxui.Rect{X: 25, Y: 25, Width: 1, Height: 1})
	want := woxui.Rect{X: 20, Y: 20, Width: 30, Height: 30}
	if got != want {
		t.Fatalf("resolved damage = %+v, want %+v", got, want)
	}
}

func TestFrameDamageTrackerSkipsStationaryCacheHit(t *testing.T) {
	current := &node{bounds: woxui.Rect{X: 20, Y: 20, Width: 10, Height: 10}}
	tracker := &frameDamageTracker{}
	tracker.add(current.bounds, current, false)
	base := woxui.Rect{X: 2, Y: 2, Width: 4, Height: 4}
	if got := tracker.resolve(base); got != base {
		t.Fatalf("stationary cache hit damage = %+v, want %+v", got, base)
	}
}

func TestFrameDamageTrackerIncludesMovedCacheHit(t *testing.T) {
	current := &node{bounds: woxui.Rect{Width: 10, Height: 10}}
	tracker := &frameDamageTracker{}
	tracker.add(woxui.Rect{X: 20, Y: 20, Width: 10, Height: 10}, current, false)
	current.place(40, 40)
	if got, want := tracker.resolve(woxui.Rect{X: 25, Y: 25, Width: 1, Height: 1}), (woxui.Rect{X: 20, Y: 20, Width: 30, Height: 30}); got != want {
		t.Fatalf("moved cache hit damage = %+v, want %+v", got, want)
	}
}

func TestStateInvalidateUsesNearestBoundaryBounds(t *testing.T) {
	services := &fakeHostServices{}
	host := NewHost(nil)
	host.AttachServices(services)
	tree := newElementTree(host)
	boundaryElement := &stateElement{tree: tree, parent: tree.root, boundary: &boundaryCache{node: &node{bounds: woxui.Rect{X: 10, Y: 20, Width: 30, Height: 40}}}}
	boundaryElement.mounted.Store(true)
	child := &stateElement{tree: tree, parent: boundaryElement}
	child.mounted.Store(true)

	StateContext{element: child}.Invalidate()

	if services.invalidatedRect != boundaryElement.boundary.node.bounds {
		t.Fatalf("invalidated rect = %+v, want nearest boundary %+v", services.invalidatedRect, boundaryElement.boundary.node.bounds)
	}
	if !child.dirty.Load() || !boundaryElement.dirty.Load() {
		t.Fatal("state invalidation did not mark the retained ancestor chain dirty")
	}
}

func TestHostConsumesPendingDamageOnlyWithNativeDamage(t *testing.T) {
	host := NewHost(nil)
	host.invalidateRect(woxui.Rect{X: 10, Y: 10, Width: 10, Height: 10})
	if got := host.consumeFrameDamage(woxui.Rect{}, woxui.Size{Width: 100, Height: 100}); got != (woxui.Rect{}) {
		t.Fatalf("fallback platform damage = %+v, want full frame", got)
	}

	host.invalidateRect(woxui.Rect{X: 10, Y: 10, Width: 10, Height: 10})
	got := host.consumeFrameDamage(woxui.Rect{X: 30, Y: 30, Width: 10, Height: 10}, woxui.Size{Width: 100, Height: 100})
	want := woxui.Rect{X: 10, Y: 10, Width: 30, Height: 30}
	if got != want {
		t.Fatalf("combined platform damage = %+v, want %+v", got, want)
	}
}
