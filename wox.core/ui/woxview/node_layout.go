package woxview

import (
	"errors"

	woxui "wox/ui/runtime"
	widget "wox/ui/widget"
)

// BoxProps supplies decoration and press behavior shared by layout containers.
type BoxProps struct {
	Styled
	// OnPress makes the whole box a tap target, for cards and list rows.
	OnPress *Handler `json:"onPress,omitempty"`
	// Label is the accessible name of a pressable box.
	Label string `json:"label,omitempty"`
}

func (p *BoxProps) handlers() []namedHandler {
	return append(p.Styled.handlers(), []namedHandler{{"onPress", p.OnPress}}...)
}

func (p *BoxProps) prepare() error {
	return p.Style.validate()
}

// SpacerProps is a fixed gap when Size is set, otherwise it absorbs free space in its parent.
type SpacerProps struct {
	Styled
	Size Size `json:"size,omitempty"`
}

func (p *SpacerProps) prepare() error {
	if p.Size.validate() != nil {
		return errors.New("size must be between 0 and 16384")
	}
	return p.Style.validate()
}

// buildBox lays out view/column children vertically with flex semantics.
// Children align to the start by default, unlike React Native's stretch, so intrinsic-width
// controls such as buttons keep their natural size.
func buildBox(b *builder, node *Node, id string) widget.Widget {
	return buildFlexBox(b, node, id, node.Props.(*BoxProps), widget.Vertical)
}

// buildFlexBox shares the standard column and unwrapped row layout without changing node identity.
func buildFlexBox(b *builder, node *Node, id string, props *BoxProps, axis widget.Axis) widget.Widget {
	content := widget.Flex{Axis: axis, Gap: props.Style.Gap.resolve(b.theme()), MainAxisAlignment: props.Style.mainAxis(), CrossAxisAlignment: props.Style.crossAxis(), Children: b.flexChildren(node, id)}
	return pressable(b, id, props, props.Style.decorate(b.theme(), content))
}

// buildStack overlays children. Each child is positioned by its own top/left/right/bottom style;
// setting both opposite sides stretches it. A stack needs a bounded height from its style or parent.
func buildStack(b *builder, node *Node, id string) widget.Widget {
	props := node.Props.(*BoxProps)
	children := make([]widget.StackChild, 0, len(node.Children))
	for index, child := range node.Children {
		style := styleOf(child).resolve(b.theme())
		value := func(position *float32) float32 {
			if position == nil {
				return 0
			}
			return *position
		}
		children = append(children, widget.StackChild{
			Left: value(style.Left), Top: value(style.Top), Right: value(style.Right), Bottom: value(style.Bottom),
			AnchorRight: style.Right != nil && style.Left == nil, AnchorBottom: style.Bottom != nil && style.Top == nil,
			StretchWidth: style.Left != nil && style.Right != nil, StretchHeight: style.Top != nil && style.Bottom != nil,
			Child: b.node(child, nodeID(id, child, index)),
		})
	}
	return pressable(b, id, props, props.Style.decorate(b.theme(), widget.Stack{Children: children}))
}

func buildSpacer(b *builder, node *Node, id string) widget.Widget {
	props := node.Props.(*SpacerProps)
	return props.Style.wrapOuter(b.theme(), widget.Container{Width: props.Size.resolve(b.theme()), Height: props.Size.resolve(b.theme())})
}

// pressable wraps a decorated box in an accessible tap target when the producer registered onPress.
func pressable(b *builder, id string, props *BoxProps, child widget.Widget) widget.Widget {
	onTap := b.tap(props.OnPress)
	if onTap == nil {
		return child
	}
	pressID := id + "#press"
	return widget.Semantics{
		Key: widget.Key(pressID), AutomationID: pressID, Role: woxui.AccessibilityRoleButton, Label: b.translate(props.Label),
		Actions: []woxui.AccessibilityAction{woxui.AccessibilityActionActivate},
		OnAction: func(woxui.AccessibilityAction, string) error {
			onTap()
			return nil
		},
		Child: widget.Gesture{ID: pressID, Cursor: woxui.PointerCursorHand, OnTap: onTap, Child: child},
	}
}
