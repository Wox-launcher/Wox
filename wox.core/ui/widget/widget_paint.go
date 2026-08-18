package widget

import woxui "wox/ui/runtime"

func (n *node) draw(displayList *woxui.DisplayList, focused, focusRingTarget woxui.AccessibilityNodeID, caretVisible, focusWithin, focusableWithin bool, work *frameWorkCounters) {
	n.drawAt(displayList, woxui.Point{}, focused, focusRingTarget, caretVisible, focusWithin, focusableWithin, work)
}

// drawAt paints this node in window space by adding origin to its local bounds.
func (n *node) drawAt(displayList *woxui.DisplayList, origin woxui.Point, focused, focusRingTarget woxui.AccessibilityNodeID, caretVisible, focusWithin, focusableWithin bool, work *frameWorkCounters) {
	if work != nil {
		work.paintVisits++
	}
	if n.focus != nil {
		focusWithin = n.id == focused
		focusableWithin = true
	} else {
		focusWithin = focusWithin || n.id == focused
	}
	bounds := offsetRect(n.bounds, origin)
	childOrigin := woxui.Point{X: bounds.X, Y: bounds.Y}
	if n.paint != nil {
		n.paint(displayList, bounds)
	}
	if n.caretPaint != nil {
		caretFocused := n.caret
		if focusableWithin {
			// Reconciliation runs after retained widgets build, so the Host focus is the
			// authoritative caret state for this frame rather than the captured FocusNode value.
			caretFocused = focusWithin
		}
		n.caretPaint(displayList, bounds, caretFocused, caretVisible)
	}
	if n.clip {
		displayList.PushClipRect(bounds)
	}
	for _, child := range n.children {
		child.drawAt(displayList, childOrigin, focused, focusRingTarget, caretVisible, focusWithin, focusableWithin, work)
	}
	if n.id == focusRingTarget && n.focus != nil && n.focus.focusRingColor.A != 0 {
		outsets := n.focus.focusRingOutsets
		ring := woxui.Rect{
			X: bounds.X - outsets.Left, Y: bounds.Y - outsets.Top,
			Width: bounds.Width + outsets.Left + outsets.Right, Height: bounds.Height + outsets.Top + outsets.Bottom,
		}
		displayList.StrokeRoundedRect(ring, n.focus.focusRingRadius, 2, n.focus.focusRingColor)
	}
	if n.clip {
		displayList.PopClipRect()
	}
}
