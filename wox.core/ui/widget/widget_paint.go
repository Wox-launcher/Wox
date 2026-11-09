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
	bounds := offsetRect(n.bounds, origin)
	if n.snapshotScope {
		if n.snapshotPaint != nil {
			n.snapshotPaint()
		}
		displayList.BeginSnapshotScope(woxui.Point{X: bounds.X, Y: bounds.Y})
		defer displayList.EndSnapshotScope()
	}
	if n.capturePaint != nil {
		capture := displayList.BeginSnapshot(n.opacityPaintBounds(origin))
		defer func() { n.capturePaint(displayList.EndSnapshot(capture)) }()
	}
	if n.transform != nil {
		displayList.PushTransform(n.drawingTransform(bounds))
		defer displayList.PopTransform()
	}
	if n.opacity != nil && *n.opacity < 1 {
		if *n.opacity <= 0 {
			return
		}
		bounds := n.opacityPaintBounds(origin)
		if bounds.Width <= 0 || bounds.Height <= 0 {
			return
		}
		displayList.BeginOpacity(bounds, *n.opacity)
		defer displayList.EndOpacity(bounds, *n.opacity)
	}
	if n.fadeRight > 0 {
		mask := bounds
		mask.Width -= n.fadeRightInset
		if mask.Width <= 0 || mask.Height <= 0 {
			return
		}
		displayList.PushClipRect(mask)
		displayList.BeginHorizontalEdgeFade(mask, 0, n.fadeRight)
		defer func() {
			displayList.EndHorizontalEdgeFade(mask, 0, n.fadeRight)
			displayList.PopClipRect()
		}()
	}
	if n.focus != nil {
		focusWithin = n.id == focused
		focusableWithin = true
	} else {
		focusWithin = focusWithin || n.id == focused
	}
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
	fading := n.fadeTop > 0 || n.fadeBottom > 0
	if fading {
		displayList.BeginEdgeFade(bounds, n.fadeTop, n.fadeBottom)
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
	if fading {
		displayList.EndEdgeFade(bounds, n.fadeTop, n.fadeBottom)
	}
	if n.clip {
		displayList.PopClipRect()
	}
}
