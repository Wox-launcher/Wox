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
		current := woxui.Rect{}
		if repaint.node != nil {
			current = globalRect(repaint.node)
		}
		if !repaint.always && repaint.oldBounds == current {
			continue
		}
		result = unionDamageRects(result, repaint.oldBounds)
		result = unionDamageRects(result, current)
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

// coverRenderedMaterials grows damage to include every renderer-blurred floating surface
// it touches. The blur reads its backdrop from the back buffer, so repainting only part of
// the content under a surface would leave the blur sampling last frame's tinted panel
// around the change. Covering one surface can reach another, so it repeats until stable.
func coverRenderedMaterials(damage woxui.Rect, materials []woxui.Rect) woxui.Rect {
	if damage.Width <= 0 || damage.Height <= 0 || len(materials) == 0 {
		return damage
	}
	// Each surface is merged at most once, so the loop ends even when float rounding keeps
	// a merged surface from testing as exactly contained.
	covered := make([]bool, len(materials))
	for grown := true; grown; {
		grown = false
		for index, material := range materials {
			if covered[index] || !damageRectsOverlap(damage, material) {
				continue
			}
			damage = unionDamageRects(damage, material)
			covered[index] = true
			grown = true
		}
	}
	return damage
}

func damageRectsOverlap(left, right woxui.Rect) bool {
	return left.Width > 0 && left.Height > 0 && right.Width > 0 && right.Height > 0 &&
		left.X < right.X+right.Width && right.X < left.X+left.Width &&
		left.Y < right.Y+right.Height && right.Y < left.Y+left.Height
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
	return activeCaretDamageAt(current, woxui.Point{}, focused, focusWithin, focusableWithin)
}

func activeCaretDamageAt(current *node, origin woxui.Point, focused woxui.AccessibilityNodeID, focusWithin, focusableWithin bool) woxui.Rect {
	if current == nil {
		return woxui.Rect{}
	}
	if current.focus != nil {
		focusWithin = current.id == focused
		focusableWithin = true
	} else {
		focusWithin = focusWithin || current.id == focused
	}
	bounds := offsetRect(current.bounds, origin)
	result := woxui.Rect{}
	if current.caretPaint != nil {
		caretActive := current.caret
		if focusableWithin {
			caretActive = focusWithin
		}
		if caretActive {
			result = bounds
		}
	}
	childOrigin := woxui.Point{X: bounds.X, Y: bounds.Y}
	for _, child := range current.children {
		result = unionDamageRects(result, activeCaretDamageAt(child, childOrigin, focused, focusWithin, focusableWithin))
	}
	return result
}
