package widget

import (
	"math"

	ui "wox/ui/runtime"
)

// Opacity composites its complete child subtree once. It preserves layout,
// focus, and input; use disabled controls when interaction should also stop.
type Opacity struct {
	Value float32
	Child Widget
}

func (w Opacity) layout(ctx context, available constraints) *node {
	value := w.Value
	if math.IsNaN(float64(value)) {
		value = 1
	}
	value = min(float32(1), max(float32(0), value))
	result := &node{opacity: &value}
	if w.Child != nil {
		child := available.constrainNode(w.Child.layout(ctx, available))
		result.bounds.Width, result.bounds.Height = child.bounds.Width, child.bounds.Height
		result.children = []*node{child}
	}
	return available.constrainNode(result)
}

// opacityPaintBounds includes visible overflow and focus rings so applying a
// group alpha does not accidentally introduce another layout-sized clip.
func (n *node) opacityPaintBounds(origin ui.Point) ui.Rect {
	bounds := offsetRect(n.bounds, origin)
	if n.clip {
		return bounds
	}
	result := ui.Rect{X: bounds.X - n.paintOutsets.Left, Y: bounds.Y - n.paintOutsets.Top,
		Width: bounds.Width + n.paintOutsets.Left + n.paintOutsets.Right, Height: bounds.Height + n.paintOutsets.Top + n.paintOutsets.Bottom}
	if n.focus != nil {
		outsets := n.focus.focusRingOutsets
		result = unionDamageRects(result, ui.Rect{X: bounds.X - outsets.Left, Y: bounds.Y - outsets.Top,
			Width: bounds.Width + outsets.Left + outsets.Right, Height: bounds.Height + outsets.Top + outsets.Bottom})
	}
	for _, child := range n.children {
		origin := ui.Point{X: bounds.X, Y: bounds.Y}
		childBounds := child.opacityPaintBounds(origin)
		if child.transform != nil {
			childBounds = child.drawingTransform(offsetRect(child.bounds, origin)).Bounds(childBounds)
		}
		result = unionDamageRects(result, childBounds)
	}
	return result
}
