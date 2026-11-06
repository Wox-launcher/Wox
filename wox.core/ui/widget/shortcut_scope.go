package widget

import ui "wox/ui/runtime"

// ShortcutScope handles keys for its focused subtree without creating a Tab stop.
// Window also allows matching when no element is focused. Nearest scopes win;
// a modal scope prevents matching shortcuts outside the modal surface.
type ShortcutScope struct {
	Key    Key
	Window bool
	OnKey  func(ui.KeyEvent) bool
	Child  Widget
}

type shortcutBehavior struct {
	window bool
	onKey  func(ui.KeyEvent) bool
}

func (w ShortcutScope) layout(ctx context, available constraints) *node {
	child := layoutBehaviorChild(ctx, available, w.Child)
	return &node{key: w.Key, kind: "shortcuts", bounds: ui.Rect{Width: child.bounds.Width, Height: child.bounds.Height}, children: []*node{child}, shortcuts: &shortcutBehavior{window: w.Window, onKey: w.OnKey}}
}

// handleShortcuts dispatches before default control actions, inside the active modal boundary.
func (h *Host) handleShortcuts(target *node, event ui.KeyEvent) bool {
	if !event.Down || event.Composing {
		return false
	}
	modal := h.activeModalScope()
	if target != nil {
		for current := target; current != nil; current = current.parent {
			if current.shortcuts != nil && current.shortcuts.onKey != nil && current.shortcuts.onKey(event) {
				return true
			}
			if current.id == modal {
				break
			}
		}
		return false
	}
	root := h.root
	if modal != 0 {
		root = h.nodes[modal]
	}
	var visit func(*node) bool
	visit = func(n *node) bool {
		if n == nil {
			return false
		}
		if n.shortcuts != nil && n.shortcuts.window && n.shortcuts.onKey != nil && n.shortcuts.onKey(event) {
			return true
		}
		for _, child := range n.children {
			if visit(child) {
				return true
			}
		}
		return false
	}
	return visit(root)
}
