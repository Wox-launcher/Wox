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
	bounds := AnchoredOverlayBounds(w.Anchor, ui.Size{Width: child.bounds.Width, Height: child.bounds.Height},
		ui.Size{Width: available.width, Height: available.height}, w.Gap, margin)
	child.place(bounds.X, bounds.Y)
	return &node{bounds: ui.Rect{Width: available.width, Height: available.height}, children: []*node{child}}
}

// AnchoredOverlayBounds resolves a popup of known size, also for callers clipping native surfaces underneath it.
func AnchoredOverlayBounds(anchor ui.Rect, popup, viewport ui.Size, gap, margin float32) ui.Rect {
	margin = max(0, margin)
	x := min(max(anchor.X, margin), max(margin, viewport.Width-margin-popup.Width))
	y := anchor.Y + anchor.Height + gap
	if y+popup.Height > viewport.Height-margin {
		y = anchor.Y - gap - popup.Height
	}
	y = min(max(y, margin), max(margin, viewport.Height-margin-popup.Height))
	return ui.Rect{X: x, Y: y, Width: popup.Width, Height: popup.Height}
}
