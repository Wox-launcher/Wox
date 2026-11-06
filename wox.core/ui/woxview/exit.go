package woxview

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	component "wox/ui/launcher/component"
	ui "wox/ui/runtime"
	"wox/ui/widget"
)

// nodeLifetime survives prop updates but is revoked immediately by SetTree when
// its identity disappears. Reusing the key creates a different event capability.
type nodeLifetime struct {
	node     *Node
	live     bool
	handlers map[HandlerID]bool
	stateKey widget.Key
	serial   uint64
}

type snapshotPart struct {
	paint *ui.PaintSnapshot
	order int
}

// exitCapture contains one last-painted block or all painted fragments of a span.
type exitCapture struct {
	owner *nodeLifetime
	parts []snapshotPart
}

// exitingNode owns inert drawing data only. The pointer also namespaces the
// Host animation key, so cancellation/reinsertion cannot reuse an older clock.
type exitingNode struct {
	id     string
	preset PresenceAnimation
	parts  []snapshotPart
	batch  uint64
}

// reconcilePresence indexes current event capabilities and moves removed paint
// into exits. Parent/child removals select the highest captured ancestor once.
func (v *View) reconcilePresence(tree *Tree) {
	next := map[string]*nodeLifetime{}
	v.hasExit = false
	var walk func(*Node, string)
	walk = func(node *Node, id string) {
		owner := v.nodes[id]
		if owner == nil {
			// Queued events can outlive the node, so identity must not depend on
			// an address that the allocator can reuse after collection.
			v.nodeSerial++
			owner = &nodeLifetime{live: true, serial: v.nodeSerial}
			owner.stateKey = widget.Key(fmt.Sprintf("%s/visual-state/%d", id, owner.serial))
		}
		if owner.node != node {
			owner.node = node
			owner.handlers = map[HandlerID]bool{}
			if handlers, ok := node.Props.(handlerOwner); ok {
				for _, h := range handlers.handlers() {
					if h.handler != nil {
						owner.handlers[h.handler.ID] = true
					}
				}
			}
		}
		next[id] = owner
		if exit := node.Props.(interface{ commonProps() CommonProps }).commonProps().Exit; exit != nil && exit.duration() > 0 {
			v.hasExit = true
		}
		for i, child := range node.Children {
			walk(child, nodeID(id, child, i))
		}
	}
	if tree != nil && tree.Root != nil {
		walk(tree.Root, nodeID(v.prefix, tree.Root, 0))
	}
	for id, owner := range v.nodes {
		if next[id] != owner {
			owner.live = false
		}
	}
	v.nodes = next
	// A reinserted key cancels its old ghost, without reviving its event capability.
	for id := range v.exits {
		if next[id] != nil {
			delete(v.exits, id)
		}
	}
	removed := []string{}
	for id, capture := range v.captures {
		if !capture.owner.live {
			removed = append(removed, id)
		} else if exit := capture.owner.node.Props.(interface{ commonProps() CommonProps }).commonProps().Exit; exit == nil || exit.duration() == 0 {
			delete(v.captures, id)
		}
	}
	slices.SortFunc(removed, func(a, b string) int { return cmp.Compare(len(a), len(b)) })
	selected := map[string]bool{}
	v.exitBatch++
	for _, id := range removed {
		capture := v.captures[id]
		delete(v.captures, id)
		covered := false
		for parent := id; strings.Contains(parent, "/"); {
			parent = parent[:strings.LastIndex(parent, "/")]
			if selected[parent] {
				covered = true
				break
			}
		}
		exit := capture.owner.node.Props.(interface{ commonProps() CommonProps }).commonProps().Exit
		if covered || exit == nil || exit.duration() == 0 || len(capture.parts) == 0 {
			continue
		}
		selected[id] = true
		v.exits[id] = &exitingNode{id: id, preset: *exit, parts: capture.parts, batch: v.exitBatch}
	}
}

// beginCapture runs for every actual paint, including cached Build frames. A
// hidden or virtualized subtree therefore cannot leave a visible old capture.
func (v *View) beginCapture() {
	clear(v.captures)
	v.captureOrder = 0
}

// capturePaint appends immutable fragments only while their node is still live.
func (v *View) capturePaint(id string, owner *nodeLifetime, paint *ui.PaintSnapshot) {
	if owner == nil || !owner.live || v.nodes[id] != owner || paint == nil {
		return
	}
	exit := owner.node.Props.(interface{ commonProps() CommonProps }).commonProps().Exit
	if exit == nil || exit.duration() == 0 {
		return
	}
	capture := v.captures[id]
	if capture == nil {
		capture = &exitCapture{owner: owner}
		v.captures[id] = capture
	}
	capture.parts = append(capture.parts, snapshotPart{paint: paint, order: v.captureOrder})
	v.captureOrder++
}

// captureNode wraps only declared exit nodes; snapshot ownership stays in View.
func (b *builder) captureNode(id string, content widget.Widget) widget.Widget {
	owner := b.view.nodes[id]
	if owner == nil {
		return content
	}
	exit := owner.node.Props.(interface{ commonProps() CommonProps }).commonProps().Exit
	if exit == nil || exit.duration() == 0 {
		return content
	}
	view := b.view
	return widget.CapturePaint{Child: content, OnCapture: func(paint *ui.PaintSnapshot) { view.capturePaint(id, owner, paint) }}
}

// captureInline records each painted fragment, outside its state/animation
// decorators. It retains the actual clip and never keeps an editor callback.
func (b *builder) captureInline(id string, base func(*ui.DisplayList, ui.Rect, bool, func())) func(*ui.DisplayList, ui.Rect, bool, func()) {
	owner := b.view.nodes[id]
	if owner == nil {
		return base
	}
	exit := owner.node.Props.(interface{ commonProps() CommonProps }).commonProps().Exit
	if exit == nil || exit.duration() == 0 {
		return base
	}
	view := b.view
	return func(list *ui.DisplayList, bounds ui.Rect, selected bool, draw func()) {
		capture := list.BeginSnapshot(bounds)
		if base == nil {
			draw()
		} else {
			base(list, bounds, selected, draw)
		}
		view.capturePaint(id, owner, list.EndSnapshot(capture))
	}
}

// exitSurface samples one keyed clock and composites one layer per removed node.
// Fragments retain paint order inside that layer, so overlapping selection and
// glyphs fade together rather than accumulating independently applied alpha.
func (v *View) exitSurface(ctx widget.StateContext, theme component.ControlTheme, child widget.Widget) widget.Widget {
	if !v.hasExit && len(v.exits) == 0 {
		return child
	}
	exits := make([]*exitingNode, 0, len(v.exits))
	for _, exit := range v.exits {
		exits = append(exits, exit)
	}
	slices.SortFunc(exits, func(a, b *exitingNode) int {
		if a.batch != b.batch {
			return cmp.Compare(a.batch, b.batch)
		}
		return cmp.Compare(a.parts[0].order, b.parts[0].order)
	})
	layers := make([]widget.Widget, 0, len(exits))
	for _, exit := range exits {
		fragments := make([]*ui.PaintSnapshot, 0, len(exit.parts))
		for _, part := range exit.parts {
			fragments = append(fragments, part.paint)
		}
		initial := float32(1)
		layers = append(layers, widget.AnimatedFloat{Key: widget.Key(fmt.Sprintf("%p/%p/exit", v, exit)), Initial: &initial, Target: 0, Duration: exit.preset.duration(), Curve: widget.AnimationEaseInOutCubic,
			Builder: func(remaining float32) widget.Widget {
				if remaining <= 0 {
					ctx.PostFrame(func() {
						if ctx.Mounted() && v.exits[exit.id] == exit {
							ctx.SetState(func() { delete(v.exits, exit.id) })
						}
					})
					return widget.Container{}
				}
				style := presenceStyle(Style{}, exit.preset.Type, remaining, theme)
				// slideUp continues upward past the target when leaving.
				style.TranslateY.Value = -style.TranslateY.Value
				matrix := style.transformMatrix(theme)
				return widget.SnapshotLayer{Fragments: fragments, Opacity: remaining, Matrix: &matrix, OriginX: .5, OriginY: .5}
			}})
	}
	return widget.SnapshotScope{Child: child, Layers: layers, OnPaint: v.beginCapture}
}

// clearPresence releases retained paint and revokes callbacks held by a previous
// host/document without discarding a reusable View's current tree.
func (v *View) clearPresence() {
	clear(v.captures)
	clear(v.exits)
	v.eventEpoch++
}
