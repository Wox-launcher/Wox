package widget

import ui "wox/ui/runtime"

// PaintOverlay draws noninteractive foreground decoration inside the child's
// measured box. It preserves child layout, input, focus, and visible overflow.
type PaintOverlay struct {
	Child Widget
	Paint func(*ui.DisplayList, ui.Rect)
}

// layout measures the retained child exactly once; the paint-only sibling neither
// takes input nor contributes extra dimensions to the parent's layout.
func (w PaintOverlay) layout(ctx context, available constraints) *node {
	result := &node{}
	if w.Child != nil {
		child := available.constrainNode(w.Child.layout(ctx, available))
		result.bounds.Width, result.bounds.Height = child.bounds.Width, child.bounds.Height
		result.children = []*node{child}
	}
	result = available.constrainNode(result)
	if w.Paint != nil {
		result.children = append(result.children, &node{bounds: result.bounds, paint: func(list *ui.DisplayList, bounds ui.Rect) {
			list.PushClipRect(bounds)
			defer list.PopClipRect()
			w.Paint(list, bounds)
		}})
	}
	return result
}
