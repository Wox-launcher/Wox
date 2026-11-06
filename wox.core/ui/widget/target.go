package widget

import (
	"fmt"
	"slices"

	ui "wox/ui/runtime"
)

// Target names a subtree for imperative focus, selection, and scroll operations
// without replacing the identity or semantics of its child controls.
type Target struct {
	Key   Key
	Child Widget
}

func (w Target) layout(ctx context, available constraints) *node {
	if w.Child == nil {
		return &node{key: w.Key, kind: "target"}
	}
	child := layoutBehaviorChild(ctx, available, w.Child)
	return &node{key: w.Key, kind: "target", bounds: ui.Rect{Width: child.bounds.Width, Height: child.bounds.Height}, children: []*node{child}}
}

// HasModalScope reports whether a modal subtree currently owns input.
func (h *Host) HasModalScope() bool { return h != nil && h.activeModalScope() != 0 }

// targetNode resolves a unique laid-out key and respects modal input isolation.
func (h *Host) targetNode(key Key) (*node, error) {
	if h == nil || h.disposed || key == "" {
		return nil, fmt.Errorf("target %q is unavailable", key)
	}
	var found *node
	for _, current := range h.nodes {
		if current.key == key {
			if found != nil {
				return nil, fmt.Errorf("target %q is ambiguous", key)
			}
			found = current
		}
	}
	if found == nil {
		return nil, fmt.Errorf("target %q is not laid out", key)
	}
	if scope := h.activeModalScope(); scope != 0 && !h.isDescendantOf(found, scope) {
		return nil, fmt.Errorf("target %q is outside the active modal scope", key)
	}
	return found, nil
}

// focusWithin resolves controls in layout order, optionally requiring select-all.
func (h *Host) focusWithin(key Key, selectAll bool) error {
	root, err := h.targetNode(key)
	if err != nil {
		return err
	}
	var find func(*node) *node
	find = func(current *node) *node {
		if current.semantic != nil && (current.semantic.hidden || !current.semantic.enabled) {
			return nil
		}
		if h.isFocusable(current) && (!selectAll || current.semantic != nil && slices.Contains(current.semantic.actions, ui.AccessibilityActionSelectAll)) {
			return current
		}
		for _, child := range current.children {
			if found := find(child); found != nil {
				return found
			}
		}
		return nil
	}
	found := find(root)
	if found == nil {
		return fmt.Errorf("target %q has no enabled control for this operation", key)
	}
	h.setFocus(found.id)
	if h.focused != found.id {
		return fmt.Errorf("target %q could not receive focus", key)
	}
	if selectAll {
		return h.performAccessibilityAction(found.id, ui.AccessibilityActionSelectAll, "")
	}
	return nil
}

// FocusWithin focuses the first enabled focusable control in a named subtree.
// Call on the UI thread after layout; this does not activate the native window.
func (h *Host) FocusWithin(key Key) error { return h.focusWithin(key, false) }

// SelectAllWithin focuses and selects all text in the first enabled text control.
func (h *Host) SelectAllWithin(key Key) error { return h.focusWithin(key, true) }

// Reveal minimally scrolls a named subtree into view without changing focus.
// A true result requires another layout before revealing the next ancestor,
// because the current geometry still reflects the previous scroll offsets.
func (h *Host) Reveal(key Key) (bool, error) {
	current, err := h.targetNode(key)
	if err != nil {
		return false, err
	}
	for ancestor := current.parent; ancestor != nil; ancestor = ancestor.parent {
		if ancestor.scroll == nil {
			continue
		}
		x, y := offsetInAncestor(current, ancestor)
		start, extent := y+ancestor.scroll.offset, current.bounds.Height
		if ancestor.scroll.horizontal {
			start, extent = x+ancestor.scroll.offset, current.bounds.Width
		}
		if ancestor.scroll.ensureVisible(start, start+extent) {
			h.invalidate()
			return true, nil
		}
	}
	return false, nil
}
