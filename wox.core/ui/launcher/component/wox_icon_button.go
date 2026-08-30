package component

import (
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// IconButtonProps describes a compact icon-only button with retained hover state.
type IconButtonProps struct {
	ID                 string
	Label              string
	Icon               woxwidget.Widget
	Width              float32
	Height             float32
	Radius             float32
	Background         woxui.Color
	HoverBackground    woxui.Color
	Selected           bool
	SelectedBackground woxui.Color
	FocusRingColor     woxui.Color
	Disabled           bool
	OnTap              func()
	OnHoverAt          func(bool, woxui.Rect)
}

type iconButtonState struct {
	hovered bool
}

// WoxIconButton builds an icon-only button with centered content and hover feedback.
func WoxIconButton(props IconButtonProps) woxwidget.Widget {
	return woxwidget.Stateful{
		Key: woxwidget.Key(props.ID), Type: (*iconButtonState)(nil), Widget: props,
		CreateState: func() woxwidget.State { return &iconButtonState{} },
	}
}

func (s *iconButtonState) InitState(_ woxwidget.StateContext, _ any) {}

func (s *iconButtonState) DidUpdateWidget(_ woxwidget.StateContext, _, _ any) {}

func (s *iconButtonState) Build(context woxwidget.StateContext, widget any) woxwidget.Widget {
	props := widget.(IconButtonProps)
	background := props.Background
	if props.Selected {
		background = props.SelectedBackground
		if background.A == 0 {
			background = props.HoverBackground
		}
	} else if s.hovered && !props.Disabled {
		background = props.HoverBackground
	}
	onTap := props.OnTap
	actions := []woxui.AccessibilityAction{woxui.AccessibilityActionActivate}
	if props.Disabled {
		onTap = nil
		actions = nil
	}
	key := woxwidget.Key(props.ID)
	content := woxwidget.Gesture{ID: props.ID, OnTap: onTap, OnHoverAt: func(inside bool, bounds woxui.Rect) {
		if inside != s.hovered {
			context.SetState(func() { s.hovered = inside })
		}
		if props.OnHoverAt != nil {
			props.OnHoverAt(inside, bounds)
		}
	}, Child: woxwidget.Container{
		Width: props.Width, Height: props.Height, Radius: props.Radius, Color: background,
		Child: woxwidget.Align{Width: props.Width, Height: props.Height, Horizontal: 0.5, Vertical: 0.5, Child: props.Icon},
	}}
	return woxwidget.Semantics{
		Key: key, AutomationID: props.ID, Role: woxui.AccessibilityRoleButton, Label: props.Label, Actions: actions, Disabled: props.Disabled, Selected: props.Selected,
		Child: woxwidget.Focusable{Key: key, Disabled: props.Disabled, FocusRingColor: props.FocusRingColor, FocusRingRadius: props.Radius, OnKey: func(event woxui.KeyEvent) bool {
			if event.Key != woxui.KeyEnter && event.Key != woxui.KeySpace {
				return false
			}
			if event.Down && onTap != nil {
				onTap()
			}
			return true
		}, Child: content},
	}
}

func (s *iconButtonState) Dispose() {}
