package widget

import woxui "wox/ui/runtime"

// prepareNodeTree assigns parent pointers and stamps Boundary window bounds in one origin-passing walk.
// The root's own parent is cleared first: a cached subtree that was nested last frame keeps its old
// parent pointer, and damage resolution calls globalRect before assignIdentities rebinds it, which
// would otherwise fold a stale ancestor's offset into every damage rect this frame.
func prepareNodeTree(current *node) {
	if current != nil {
		current.parent = nil
	}
	wireTree(current, woxui.Point{})
}

// wireTree walks once, wiring parents and stamping each Boundary from the accumulated origin.
func wireTree(current *node, origin woxui.Point) {
	if current == nil {
		return
	}
	if current.boundary != nil {
		current.boundary.globalBounds = offsetRect(current.bounds, origin)
	}
	childOrigin := woxui.Point{X: origin.X + current.bounds.X, Y: origin.Y + current.bounds.Y}
	for _, child := range current.children {
		child.parent = current
		wireTree(child, childOrigin)
	}
}

// globalRect maps a local node box into window coordinates by walking ancestors.
func globalRect(current *node) woxui.Rect {
	if current == nil {
		return woxui.Rect{}
	}
	x, y := current.bounds.X, current.bounds.Y
	for parent := current.parent; parent != nil; parent = parent.parent {
		x += parent.bounds.X
		y += parent.bounds.Y
	}
	return woxui.Rect{X: x, Y: y, Width: current.bounds.Width, Height: current.bounds.Height}
}

// localPoint converts a window-space point into this node's local coordinates.
func (n *node) localPoint(point woxui.Point) woxui.Point {
	bounds := globalRect(n)
	return woxui.Point{X: point.X - bounds.X, Y: point.Y - bounds.Y}
}

func boundaryWindowBounds(cache *boundaryCache) woxui.Rect {
	if cache == nil {
		return woxui.Rect{}
	}
	if cache.globalBounds.Width > 0 && cache.globalBounds.Height > 0 {
		return cache.globalBounds
	}
	if cache.node != nil {
		return globalRect(cache.node)
	}
	return woxui.Rect{}
}

func containsPoint(bounds woxui.Rect, point woxui.Point) bool {
	return point.X >= bounds.X && point.Y >= bounds.Y && point.X < bounds.X+bounds.Width && point.Y < bounds.Y+bounds.Height
}

func offsetRect(bounds woxui.Rect, origin woxui.Point) woxui.Rect {
	return woxui.Rect{X: origin.X + bounds.X, Y: origin.Y + bounds.Y, Width: bounds.Width, Height: bounds.Height}
}

// offsetInAncestor sums local offsets from current up to, but not including, ancestor.
func offsetInAncestor(current, ancestor *node) (float32, float32) {
	var x, y float32
	for n := current; n != nil && n != ancestor; n = n.parent {
		x += n.bounds.X
		y += n.bounds.Y
	}
	return x, y
}
