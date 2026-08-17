package widget

import (
	"testing"

	woxui "wox/ui/runtime"
)

func TestPrepareNodeTreeStampsBoundaryBoundsFromOrigin(t *testing.T) {
	inner := &node{bounds: woxui.Rect{X: 8, Y: 4, Width: 10, Height: 12}, boundary: &boundaryCache{}}
	parent := &node{bounds: woxui.Rect{X: 20, Y: 30, Width: 40, Height: 40}, children: []*node{inner}, boundary: &boundaryCache{}}
	root := &node{bounds: woxui.Rect{Width: 100, Height: 80}, children: []*node{parent}}
	prepareNodeTree(root)
	if inner.parent != parent || parent.parent != root {
		t.Fatalf("parent pointers = inner %p parent %p, want wired tree", inner.parent, parent.parent)
	}
	if parent.boundary.globalBounds != (woxui.Rect{X: 20, Y: 30, Width: 40, Height: 40}) {
		t.Fatalf("parent global bounds = %+v", parent.boundary.globalBounds)
	}
	if inner.boundary.globalBounds != (woxui.Rect{X: 28, Y: 34, Width: 10, Height: 12}) {
		t.Fatalf("inner global bounds = %+v, want origin-passed 28,34", inner.boundary.globalBounds)
	}
	if got := globalRect(inner); got != inner.boundary.globalBounds {
		t.Fatalf("stamped bounds %+v != ancestor walk %+v", inner.boundary.globalBounds, got)
	}
}

func TestPrepareNodeTreeClearsStaleRootParent(t *testing.T) {
	child := &node{bounds: woxui.Rect{X: 5, Y: 7, Width: 10, Height: 10}}
	reused := &node{bounds: woxui.Rect{X: 2, Y: 3, Width: 30, Height: 30}, children: []*node{child}}
	wrapper := &node{bounds: woxui.Rect{X: 100, Y: 200, Width: 300, Height: 300}, children: []*node{reused}}
	prepareNodeTree(wrapper)
	if reused.parent != wrapper {
		t.Fatalf("setup parent = %p, want the wrapper from the previous frame", reused.parent)
	}

	// The cached subtree survives into a frame where it is the root, so its parent pointer
	// still references last frame's wrapper until prepareNodeTree clears it.
	prepareNodeTree(reused)
	if reused.parent != nil {
		t.Fatalf("root parent = %p, want nil", reused.parent)
	}
	if got := globalRect(child); got != (woxui.Rect{X: 7, Y: 10, Width: 10, Height: 10}) {
		t.Fatalf("descendant window bounds = %+v, want 7,10 without the stale wrapper offset", got)
	}
}

func TestPlaceDoesNotMoveDescendants(t *testing.T) {
	child := &node{bounds: woxui.Rect{X: 4, Y: 6, Width: 10, Height: 10}}
	parent := &node{bounds: woxui.Rect{Width: 40, Height: 40}, children: []*node{child}}
	parent.place(20, 30)
	if parent.bounds.X != 20 || parent.bounds.Y != 30 {
		t.Fatalf("parent bounds = %+v, want origin 20,30", parent.bounds)
	}
	if child.bounds.X != 4 || child.bounds.Y != 6 {
		t.Fatalf("child bounds = %+v, want unchanged local 4,6", child.bounds)
	}
}

func TestNestedHitTestUsesAccumulatedOrigin(t *testing.T) {
	target := &node{bounds: woxui.Rect{X: 8, Y: 4, Width: 10, Height: 10}, gesture: &gesture{id: "cell"}}
	parent := &node{bounds: woxui.Rect{X: 20, Y: 30, Width: 40, Height: 40}, children: []*node{target}}
	root := &node{bounds: woxui.Rect{Width: 100, Height: 100}, children: []*node{parent}}
	if hit := root.hitTest(woxui.Point{X: 29, Y: 35}); hit != target {
		t.Fatalf("hit = %v, want nested gesture at window 29,35", hit)
	}
	if hit := root.hitTest(woxui.Point{X: 8, Y: 4}); hit != nil {
		t.Fatalf("hit = %v, want miss at the child's local origin", hit)
	}
}

func TestBoundaryCacheHitKeepsDescendantLocalOrigins(t *testing.T) {
	builds := 0
	host := NewHost(func(woxui.FrameInfo) Widget {
		return boundaryOffset{left: 40, child: Boundary[boundaryTestProps]{
			Key: "cached", Props: boundaryTestProps{Value: 1},
			Build: func(boundaryTestProps) Widget {
				builds++
				return Container{Width: 20, Height: 20, Child: Container{Width: 10, Height: 10}}
			},
		}}
	})
	services := &fakeHostServices{}
	host.AttachServices(services)
	first := &woxui.DisplayList{}
	host.Frame(first, woxui.FrameInfo{Size: woxui.Size{Width: 80, Height: 40}})
	if builds != 1 {
		t.Fatalf("first builds = %d, want 1", builds)
	}
	cached := host.root.children[0]
	inner := cached.children[0]
	if cached.bounds.X != 40 || inner.bounds.X != 0 {
		t.Fatalf("first local origins = cached %+v inner %+v, want 40 and 0", cached.bounds, inner.bounds)
	}

	host.build = func(woxui.FrameInfo) Widget {
		return boundaryOffset{left: 12, child: Boundary[boundaryTestProps]{
			Key: "cached", Props: boundaryTestProps{Value: 1},
			Build: func(boundaryTestProps) Widget {
				builds++
				return Container{Width: 20, Height: 20, Child: Container{Width: 10, Height: 10}}
			},
		}}
	}
	second := &woxui.DisplayList{}
	host.Frame(second, woxui.FrameInfo{Size: woxui.Size{Width: 80, Height: 40}})
	if builds != 1 {
		t.Fatalf("moved cache hit rebuilt the subtree, builds = %d", builds)
	}
	cached = host.root.children[0]
	inner = cached.children[0]
	if cached.bounds.X != 12 {
		t.Fatalf("moved cached root X = %v, want 12", cached.bounds.X)
	}
	if inner.bounds.X != 0 || inner.bounds.Y != 0 {
		t.Fatalf("descendant local origin = %+v, want unchanged 0,0", inner.bounds)
	}
	if got := globalRect(inner); got.X != 12 {
		t.Fatalf("descendant window bounds = %+v, want x 12", got)
	}
}
