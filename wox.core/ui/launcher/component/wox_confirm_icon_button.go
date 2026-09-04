package component

import (
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// WoxConfirmIconButton requires two activations and cancels on pointer leave, blur, or Escape.
func WoxConfirmIconButton(props ConfirmIconButtonProps) woxwidget.Widget {
	return woxwidget.Stateful{Key: woxwidget.Key(props.ID), Type: (*confirmIconButtonState)(nil), Widget: props, CreateState: func() woxwidget.State { return &confirmIconButtonState{} }}
}

type confirmIconButtonState struct {
	hovered bool
	confirm bool
}

// ConfirmIconButtonProps describes a compact destructive action with two-step confirmation.
type ConfirmIconButtonProps struct {
	ID           string
	Label        string
	ConfirmLabel string
	Icon         *woxui.Image
	Theme        Theme
	OnDelete     func()
}

func (s *confirmIconButtonState) InitState(_ woxwidget.StateContext, _ any) {}

func (s *confirmIconButtonState) DidUpdateWidget(_ woxwidget.StateContext, _, _ any) {}

// Build keeps confirmation local to the mounted control.
func (s *confirmIconButtonState) Build(context woxwidget.StateContext, widget any) woxwidget.Widget {
	props := widget.(ConfirmIconButtonProps)
	return confirmIconButtonWithState(props, s.confirm, func(inside bool) {
		if inside != s.hovered || !inside && s.confirm {
			context.SetState(func() { s.setHovered(inside) })
		}
	}, func() {
		deleteConfirmed := false
		context.SetState(func() { deleteConfirmed = s.advanceConfirmation() })
		if deleteConfirmed && props.OnDelete != nil {
			props.OnDelete()
		}
	})
}

func (s *confirmIconButtonState) setHovered(inside bool) {
	s.hovered = inside
	if !inside {
		s.confirm = false
	}
}

// advanceConfirmation requires two consecutive activations before deletion.
func (s *confirmIconButtonState) advanceConfirmation() bool {
	if !s.confirm {
		s.confirm = true
		return false
	}
	s.confirm = false
	return true
}

func (s *confirmIconButtonState) Dispose() {}

// confirmIconButtonWithState applies the shared idle and confirmation treatments.
func confirmIconButtonWithState(props ConfirmIconButtonProps, confirm bool, onHover func(bool), onDelete func()) woxwidget.Widget {
	hoverBackground := props.Theme.ResultTitle
	hoverBackground.A = uint8(float32(hoverBackground.A) * 0.1)
	label := props.Label
	background := woxui.Color{}
	radius := float32(4)
	var icon woxwidget.Widget
	if props.Icon != nil {
		icon = woxwidget.Image{Source: props.Icon, Width: 16, Height: 16}
	}
	if confirm {
		label = props.ConfirmLabel
		background = props.Theme.ErrorText
		hoverBackground = props.Theme.ErrorText
		icon = CheckGlyph(14, props.Theme.SelectedTitle)
	}
	return WoxIconButton(IconButtonProps{
		ID: props.ID, Label: label, Icon: icon, Width: SettingsCompactControlHeight, Height: SettingsCompactControlHeight, Radius: radius,
		Background: background, HoverBackground: hoverBackground, FocusRingColor: props.Theme.Cursor, OnFocusChange: func(focused bool) {
			if !focused && onHover != nil {
				onHover(false)
			}
		}, OnKey: func(event woxui.KeyEvent) bool {
			if event.Key == woxui.KeyEscape && confirm {
				if event.Down && onHover != nil {
					onHover(false)
				}
				return true
			}
			return false
		}, OnTap: onDelete, OnHoverAt: func(inside bool, _ woxui.Rect) {
			if onHover != nil {
				onHover(inside)
			}
		},
	})
}
