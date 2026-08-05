package widget

import woxui "wox/ui/runtime"

type boundaryDamage struct {
	oldBounds woxui.Rect
	node      *node
	always    bool
}

type frameDamageTracker struct {
	rebuilt []boundaryDamage
}

func (t *frameDamageTracker) add(oldBounds woxui.Rect, current *node, always bool) {
	if t == nil || current == nil {
		return
	}
	t.rebuilt = append(t.rebuilt, boundaryDamage{oldBounds: oldBounds, node: current, always: always})
}

func (t *frameDamageTracker) resolve(base woxui.Rect) woxui.Rect {
	if t == nil {
		return base
	}
	result := base
	for _, repaint := range t.rebuilt {
		if !repaint.always && repaint.node != nil && repaint.oldBounds == repaint.node.bounds {
			continue
		}
		result = unionDamageRects(result, repaint.oldBounds)
		if repaint.node != nil {
			result = unionDamageRects(result, repaint.node.bounds)
		}
	}
	return result
}

func expandDamageRect(rect woxui.Rect, outset float32) woxui.Rect {
	if rect.Width <= 0 || rect.Height <= 0 || outset <= 0 {
		return rect
	}
	return woxui.Rect{X: rect.X - outset, Y: rect.Y - outset, Width: rect.Width + 2*outset, Height: rect.Height + 2*outset}
}

func unionDamageRects(left, right woxui.Rect) woxui.Rect {
	if left.Width <= 0 || left.Height <= 0 {
		return right
	}
	if right.Width <= 0 || right.Height <= 0 {
		return left
	}
	x := min(left.X, right.X)
	y := min(left.Y, right.Y)
	rightEdge := max(left.X+left.Width, right.X+right.Width)
	bottomEdge := max(left.Y+left.Height, right.Y+right.Height)
	return woxui.Rect{X: x, Y: y, Width: rightEdge - x, Height: bottomEdge - y}
}

func clipDamageRect(rect woxui.Rect, size woxui.Size) woxui.Rect {
	if rect.Width <= 0 || rect.Height <= 0 || size.Width <= 0 || size.Height <= 0 {
		return woxui.Rect{}
	}
	x := max(float32(0), rect.X)
	y := max(float32(0), rect.Y)
	right := min(size.Width, rect.X+rect.Width)
	bottom := min(size.Height, rect.Y+rect.Height)
	if right <= x || bottom <= y {
		return woxui.Rect{}
	}
	return woxui.Rect{X: x, Y: y, Width: right - x, Height: bottom - y}
}

// activeCaretDamage returns only the focused editor paint bounds, independent of its enclosing Boundary.
func activeCaretDamage(current *node, focused woxui.AccessibilityNodeID, focusWithin, focusableWithin bool) woxui.Rect {
	if current == nil {
		return woxui.Rect{}
	}
	if current.focus != nil {
		focusWithin = current.id == focused
		focusableWithin = true
	} else {
		focusWithin = focusWithin || current.id == focused
	}
	result := woxui.Rect{}
	if current.caretPaint != nil {
		caretActive := current.caret
		if focusableWithin {
			caretActive = focusWithin
		}
		if caretActive {
			result = current.bounds
		}
	}
	for _, child := range current.children {
		result = unionDamageRects(result, activeCaretDamage(child, focused, focusWithin, focusableWithin))
	}
	return result
}
