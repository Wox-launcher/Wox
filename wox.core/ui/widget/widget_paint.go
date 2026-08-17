package widget

import (
	"os"
	"strings"

	woxui "wox/ui/runtime"
)

// DisableRetainedPaintEnvironment flattens every Boundary into immediate drawing commands.
const DisableRetainedPaintEnvironment = "WOX_DISABLE_RETAINED_PAINT"

func retainedPaintDisabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(DisableRetainedPaintEnvironment))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func (n *node) draw(displayList *woxui.DisplayList, focused, focusRingTarget woxui.AccessibilityNodeID, caretVisible, focusWithin, focusableWithin bool, work *frameWorkCounters) {
	n.drawAt(displayList, woxui.Point{}, focused, focusRingTarget, caretVisible, focusWithin, focusableWithin, work, true, nil)
}

// drawUnretained paints the tree without creating or reusing retained paint segments.
func (n *node) drawUnretained(displayList *woxui.DisplayList, focused, focusRingTarget woxui.AccessibilityNodeID, caretVisible, focusWithin, focusableWithin bool) {
	n.drawAt(displayList, woxui.Point{}, focused, focusRingTarget, caretVisible, focusWithin, focusableWithin, nil, false, nil)
}

// drawAt paints this node in window space by adding origin to its local bounds.
func (n *node) drawAt(displayList *woxui.DisplayList, origin woxui.Point, focused, focusRingTarget woxui.AccessibilityNodeID, caretVisible, focusWithin, focusableWithin bool, work *frameWorkCounters, retain bool, nested *[]*node) {
	if retain && n.boundary != nil && !retainedPaintDisabled() {
		n.drawBoundary(displayList, origin, focused, focusRingTarget, caretVisible, focusWithin, focusableWithin, work, nested)
		return
	}
	n.drawImmediate(displayList, origin, focused, focusRingTarget, caretVisible, focusWithin, focusableWithin, work, retain, nil, nested)
}

func (n *node) drawBoundary(displayList *woxui.DisplayList, origin woxui.Point, focused, focusRingTarget woxui.AccessibilityNodeID, caretVisible, focusWithin, focusableWithin bool, work *frameWorkCounters, nested *[]*node) {
	if nested != nil {
		*nested = append(*nested, n)
	}
	segmentOrigin := woxui.Point{X: origin.X + n.bounds.X, Y: origin.Y + n.bounds.Y}
	if n.boundary.paint != nil && n.boundary.paint.Fingerprint.Matches(focused, focusRingTarget, caretVisible) {
		if work != nil {
			work.paintVisits++
			work.paintSegmentReuses++
		}
		if replacements := n.refreshNestedPaint(focused, focusRingTarget, caretVisible, focusWithin, focusableWithin, work); len(replacements) > 0 {
			n.boundary.paint = woxui.RebasePaintSegment(n.boundary.paint, replacements)
		}
		displayList.AppendPaintSegment(n.boundary.paint, segmentOrigin)
		return
	}
	n.recordPaintSegment(displayList, origin, focused, focusRingTarget, caretVisible, focusWithin, focusableWithin, work, true)
}

func (n *node) recordPaintSegment(displayList *woxui.DisplayList, origin woxui.Point, focused, focusRingTarget woxui.AccessibilityNodeID, caretVisible, focusWithin, focusableWithin bool, work *frameWorkCounters, appendToParent bool) {
	local := &woxui.DisplayList{}
	var nested []*node
	var fingerprint woxui.PaintFingerprint
	n.drawImmediate(local, woxui.Point{X: -n.bounds.X, Y: -n.bounds.Y}, focused, focusRingTarget, caretVisible, focusWithin, focusableWithin, work, true, &fingerprint, &nested)
	n.boundary.paint = woxui.CapturePaintSegment(woxui.Rect{Width: n.bounds.Width, Height: n.bounds.Height}, local, fingerprint)
	n.boundary.nestedPaint = nested
	if appendToParent {
		displayList.AppendPaintSegment(n.boundary.paint, woxui.Point{X: origin.X + n.bounds.X, Y: origin.Y + n.bounds.Y})
	}
}

func (n *node) refreshNestedPaint(focused, focusRingTarget woxui.AccessibilityNodeID, caretVisible, focusWithin, focusableWithin bool, work *frameWorkCounters) map[*woxui.PaintSegment]*woxui.PaintSegment {
	if n.boundary == nil {
		return nil
	}
	var replacements map[*woxui.PaintSegment]*woxui.PaintSegment
	add := func(old, next *woxui.PaintSegment) {
		if old == nil || next == nil || old == next {
			return
		}
		if replacements == nil {
			replacements = map[*woxui.PaintSegment]*woxui.PaintSegment{}
		}
		replacements[old] = next
	}
	for _, child := range n.boundary.nestedPaint {
		if child == nil || child.boundary == nil {
			continue
		}
		old := child.boundary.paint
		if old != nil && old.Fingerprint.Matches(focused, focusRingTarget, caretVisible) {
			if childReplacements := child.refreshNestedPaint(focused, focusRingTarget, caretVisible, focusWithin, focusableWithin, work); len(childReplacements) > 0 {
				next := woxui.RebasePaintSegment(old, childReplacements)
				child.boundary.paint = next
				add(old, next)
			}
			continue
		}
		child.recordPaintSegment(&woxui.DisplayList{}, woxui.Point{}, focused, focusRingTarget, caretVisible, focusWithin, focusableWithin, work, false)
		add(old, child.boundary.paint)
	}
	return replacements
}

func (n *node) drawImmediate(displayList *woxui.DisplayList, origin woxui.Point, focused, focusRingTarget woxui.AccessibilityNodeID, caretVisible, focusWithin, focusableWithin bool, work *frameWorkCounters, retain bool, fingerprint *woxui.PaintFingerprint, nested *[]*node) {
	if work != nil {
		work.paintVisits++
	}
	if n.focus != nil {
		focusWithin = n.id == focused
		focusableWithin = true
		if fingerprint != nil {
			fingerprint.UsesFocus = true
			fingerprint.Focused = focused
		}
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
		if fingerprint != nil {
			fingerprint.UsesCaret = true
			fingerprint.CaretVisible = caretVisible
		}
		n.caretPaint(displayList, bounds, caretFocused, caretVisible)
	}
	if n.clip {
		displayList.PushClipRect(bounds)
	}
	for _, child := range n.children {
		child.drawAt(displayList, childOrigin, focused, focusRingTarget, caretVisible, focusWithin, focusableWithin, work, retain, nested)
	}
	if n.focus != nil && n.focus.focusRingColor.A != 0 {
		if fingerprint != nil {
			// Record the ring dependency even when pointer focus hides it, so a
			// later keyboard-focus frame with the same focused ID misses the cache.
			fingerprint.UsesFocusRing = true
			fingerprint.FocusRing = focusRingTarget
		}
		if n.id == focusRingTarget {
			outsets := n.focus.focusRingOutsets
			ring := woxui.Rect{
				X: bounds.X - outsets.Left, Y: bounds.Y - outsets.Top,
				Width: bounds.Width + outsets.Left + outsets.Right, Height: bounds.Height + outsets.Top + outsets.Bottom,
			}
			displayList.StrokeRoundedRect(ring, n.focus.focusRingRadius, 2, n.focus.focusRingColor)
		}
	}
	if n.clip {
		displayList.PopClipRect()
	}
}
