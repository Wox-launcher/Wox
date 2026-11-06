package woxview

import (
	"reflect"

	"wox/ui/widget"
)

// nodeVisualConfig carries an immutable tree snapshot and per-build host capabilities.
type nodeVisualConfig struct {
	builder builder
	node    *Node
	id      string
}

// nodeVisualState belongs to the mounted node, never to the plugin or a global registry.
type nodeVisualState struct {
	hovered, pressed, focused bool
	owner                     *nodeLifetime
	enter                     *PresenceAnimation
}

// InitState captures presence once, independently of subsequent producer echoes.
func (s *nodeVisualState) InitState(_ widget.StateContext, value any) {
	c := value.(nodeVisualConfig)
	s.receiveEnter(c.builder.nodeOwner, c.node.Props.(interface{ commonProps() CommonProps }).commonProps())
}
func (*nodeVisualState) Dispose() {}

// DidUpdateWidget prevents stale feedback after removing observers, and prevents
// a held press from reappearing after disable/re-enable.
func (s *nodeVisualState) DidUpdateWidget(_ widget.StateContext, _, next any) {
	c := next.(nodeVisualConfig)
	s.receiveEnter(c.builder.nodeOwner, c.node.Props.(interface{ commonProps() CommonProps }).commonProps())
	if c.node.Props.(interface{ interactionStyles() InteractionStyles }).interactionStyles().empty() {
		s.hovered, s.pressed, s.focused = false, false, false
		return
	}
	if c.builder.nodeDisabled(c.node) {
		s.pressed = false
	}
}

// Build applies hover, then press, then focus; disabled suppresses all active states.
func (s *nodeVisualState) Build(ctx widget.StateContext, value any) widget.Widget {
	c := value.(nodeVisualConfig)
	common := c.node.Props.(interface{ commonProps() CommonProps }).commonProps()
	p := c.node.Props.(interface{ interactionStyles() InteractionStyles }).interactionStyles()
	// Keep the state boundary mounted so adding paint state or animation never
	// resets a field. Plain nodes avoid allocating the animation closure chain.
	if p.empty() && common.Transition == nil && s.enter == nil && common.Animation == nil {
		return c.builder.decorateNode(c.node, c.id, catalog[c.node.Type].build(&c.builder, c.node, c.id))
	}
	return s.buildVisual(ctx, c)
}

// buildVisual isolates escaping animation snapshots from the ordinary node path.
func (s *nodeVisualState) buildVisual(ctx widget.StateContext, c nodeVisualConfig) widget.Widget {
	b, node := c.builder, c.node
	p := node.Props.(interface{ interactionStyles() InteractionStyles }).interactionStyles()
	disabled := b.nodeDisabled(node)
	patch := s.patch(p, disabled)
	style := styleOf(node)
	if patch != (StateStyle{}) {
		style = patch.apply(style)
	}
	common := node.Props.(interface{ commonProps() CommonProps }).commonProps()
	content := b.transitionStyle(style, common.Transition, c.id, func(style Style) widget.Widget {
		return s.enterStyle(ctx, &b, c.id, style, func(sample Style) widget.Widget {
			return b.buildAnimatedNode(node, c.id, sample, common.Animation)
		})
	})
	if p.empty() {
		return content
	}
	change := func(current *bool, next bool) {
		if *current != next {
			ctx.SetState(func() { *current = next })
		}
	}
	return widget.FocusObserver{Key: widget.Key(c.id + "/style-focus"), OnChange: func(focused bool) { change(&s.focused, focused) },
		Child: widget.Gesture{ID: c.id + "/style-pointer", CoverHover: true, ObserveDescendants: true,
			OnHover:       func(hovered bool) { change(&s.hovered, hovered) },
			OnPressChange: func(pressed bool) { change(&s.pressed, pressed && !disabled) },
			Child:         widget.Container{Child: content},
		},
	}
}

// nodeWithStyle preserves the document while builders consume a per-frame sample.
func nodeWithStyle(node *Node, style Style) *Node {
	copyNode := *node
	copyProps := reflect.New(reflect.TypeOf(node.Props).Elem())
	copyProps.Elem().Set(reflect.ValueOf(node.Props).Elem())
	copyNode.Props = copyProps.Interface()
	copyNode.Props.(interface{ setStyle(Style) }).setStyle(style)
	return &copyNode
}

// patch shares state precedence between block nodes and inline text fragments.
func (s *nodeVisualState) patch(p InteractionStyles, disabled bool) StateStyle {
	patch := StateStyle{}
	if disabled {
		patch = patch.merge(p.DisabledStyle)
	} else {
		if s.hovered {
			patch = patch.merge(p.HoverStyle)
		}
		if s.pressed {
			patch = patch.merge(p.PressedStyle)
		}
		if s.focused {
			patch = patch.merge(p.FocusStyle)
		}
	}
	return patch
}

// nodeDisabled follows existing control and form semantics without changing activation policy.
func (b *builder) nodeDisabled(node *Node) bool {
	if p, ok := node.Props.(interface{ isDisabled() bool }); ok && p.isDisabled() {
		return true
	}
	if _, field := node.Props.(fieldOwner); field && b.fieldsDisabled() {
		return true
	}
	if node.Type == NodeSubmit {
		return b.formSubmit() == nil
	}
	if node.Type == NodeLink {
		p := node.Props.(*LinkProps)
		return p.OnPress == nil && b.props.OnOpenLink == nil || p.OnPress != nil && b.props.OnEvent == nil && b.props.OnNodeEvent == nil
	}
	return false
}
