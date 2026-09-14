package widget

import (
	"math"

	woxui "wox/ui/runtime"
)

type boundaryDamage struct {
	oldBounds woxui.Rect
	node      *node
	always    bool
}

// currentMaterialBounds includes newly grown surfaces before damage classification.
// Previous display lists only describe the smaller panel after a filter has settled.
func currentMaterialBounds(current *node, materials []woxui.Rect) []woxui.Rect {
	if current == nil {
		return materials
	}
	if current.floating {
		materials = append(materials, globalRect(current))
	}
	for _, child := range current.children {
		materials = currentMaterialBounds(child, materials)
	}
	return materials
}

// keyedNodeDamage resolves new geometry after layout; old geometry was invalidated before the frame.
func keyedNodeDamage(current *node, keys map[Key]bool) woxui.Rect {
	if current == nil {
		return woxui.Rect{}
	}
	if keys[current.key] {
		return globalRect(current)
	}
	var damage woxui.Rect
	for _, child := range current.children {
		damage = unionDamageRects(damage, keyedNodeDamage(child, keys))
	}
	return damage
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

// coverRenderedMaterials grows damage so a renderer-blurred surface can resample its
// backdrop. The blur reads the back buffer, so a change under a surface (or in its
// sample halo) must repaint that whole surface plus sampleMargin.
//
// Adjacent cards such as the action panel and launcher toolbar often sit within one
// blur kernel of each other. Unioning both into one axis-aligned rect would swallow
// the result list between them. Damage contained by a surface is a content update:
// only that surface and overlapping surfaces are covered. Material bounds include
// both retained frames because native buffer repair can still reference the larger
// panel after filtering shrinks it. Halo overlap handles damage outside every surface.
func coverRenderedMaterials(damage woxui.Rect, materials []woxui.Rect, sampleMargin, scale float32) woxui.Rect {
	if damage.Width <= 0 || damage.Height <= 0 || len(materials) == 0 {
		return damage
	}
	covered := make([]bool, len(materials))
	containedCount := 0
	if scale <= 0 {
		scale = 1
	}
	for index, material := range materials {
		// Win32 rounds invalidation outward to physical pixels. Compare on that
		// same grid so fractional panel edges do not pull in an adjacent toolbar.
		left := float32(math.Floor(float64(material.X*scale))) / scale
		top := float32(math.Floor(float64(material.Y*scale))) / scale
		right := float32(math.Ceil(float64((material.X+material.Width)*scale))) / scale
		bottom := float32(math.Ceil(float64((material.Y+material.Height)*scale))) / scale
		if damageRectContains(woxui.Rect{X: left, Y: top, Width: right - left, Height: bottom - top}, damage) {
			covered[index] = true
			containedCount++
		}
	}
	if containedCount == 0 {
		for index, material := range materials {
			if damageRectsOverlap(damage, expandDamageRect(material, sampleMargin)) {
				covered[index] = true
			}
		}
	}
	// A tooltip stacked on a panel shares the panel's unexpanded bounds, so covering
	// one reaches the other. Adjacent cards with only a halo overlap stay separate.
	for grown := true; grown; {
		grown = false
		for index, material := range materials {
			if covered[index] {
				continue
			}
			for other, isCovered := range covered {
				if !isCovered || !damageRectsOverlap(material, materials[other]) {
					continue
				}
				covered[index] = true
				grown = true
				break
			}
		}
	}
	for index, material := range materials {
		if covered[index] {
			damage = unionDamageRects(damage, expandDamageRect(material, sampleMargin))
		}
	}
	return damage
}

func damageRectsOverlap(left, right woxui.Rect) bool {
	return left.Width > 0 && left.Height > 0 && right.Width > 0 && right.Height > 0 &&
		left.X < right.X+right.Width && right.X < left.X+left.Width &&
		left.Y < right.Y+right.Height && right.Y < left.Y+left.Height
}

func damageRectContains(outer, inner woxui.Rect) bool {
	return inner.Width > 0 && inner.Height > 0 && outer.Width > 0 && outer.Height > 0 &&
		inner.X >= outer.X && inner.Y >= outer.Y &&
		inner.X+inner.Width <= outer.X+outer.Width &&
		inner.Y+inner.Height <= outer.Y+outer.Height
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
