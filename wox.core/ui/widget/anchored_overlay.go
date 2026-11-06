package widget

import ui "wox/ui/runtime"

// AnchoredOverlay positions window-level popup content below its logical anchor,
// flips above when needed, and clamps it within the current logical viewport.
// Install it with SetHostOverlay so ancestor clipping and scrolling do not apply.
type AnchoredOverlay struct {
	Anchor ui.Rect
	Gap    float32
	Margin float32
	Child  Widget
}

func (w AnchoredOverlay) layout(ctx context, available constraints) *node {
	margin := max(float32(0), w.Margin)
	width, height := max(float32(0), available.width-2*margin), max(float32(0), available.height-2*margin)
	child := layoutBehaviorChild(ctx, constraints{width: width, height: height}, w.Child)
	x := min(max(w.Anchor.X, margin), max(margin, available.width-margin-child.bounds.Width))
	y := w.Anchor.Y + w.Anchor.Height + w.Gap
	if y+child.bounds.Height > available.height-margin {
		y = w.Anchor.Y - w.Gap - child.bounds.Height
	}
	y = min(max(y, margin), max(margin, available.height-margin-child.bounds.Height))
	child.place(x, y)
	return &node{bounds: ui.Rect{Width: available.width, Height: available.height}, children: []*node{child}}
}
