package widget

import ui "wox/ui/runtime"

// FocusObserver observes focus entering or leaving a subtree without adding a tab stop.
type FocusObserver struct {
	Key      Key
	OnChange func(bool)
	Child    Widget
}

func (w FocusObserver) layout(ctx context, available constraints) *node {
	child := layoutBehaviorChild(ctx, available, w.Child)
	return &node{key: w.Key, kind: "focus_observer", bounds: ui.Rect{Width: child.bounds.Width, Height: child.bounds.Height}, children: []*node{child}, onFocusWithin: w.OnChange}
}

// syncFocusObservers compares stable identities, so rebuilding or switching between
// sibling fields does not produce a false blur/focus pair for their common ancestor.
func (h *Host) syncFocusObservers() {
	var next map[Key]func(bool)
	for n := h.nodes[h.focused]; n != nil; n = n.parent {
		if n.key != "" && n.onFocusWithin != nil {
			if next == nil {
				next = map[Key]func(bool){}
			}
			next[n.key] = n.onFocusWithin
		}
	}
	previous := h.focusObservers
	h.focusObservers = next
	for key := range previous {
		if _, ok := next[key]; ok {
			continue
		}
		// Removed observers cannot own live callbacks; a surviving ancestor still
		// receives blur if its focused descendant disappeared during this frame.
		for _, n := range h.nodes {
			if n.key == key && n.onFocusWithin != nil {
				n.onFocusWithin(false)
				break
			}
		}
	}
	for key, callback := range next {
		if _, ok := previous[key]; !ok {
			callback(true)
		}
	}
}
