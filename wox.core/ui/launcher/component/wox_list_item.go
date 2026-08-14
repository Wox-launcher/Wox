package component

import (
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// ListItemProps describes one selectable Wox list row.
type ListItemProps struct {
	ID              string
	Label           string
	Width           float32
	Height          float32
	Radius          *float32
	Padding         woxwidget.Insets
	Background      *woxui.Color
	HoverBackground *woxui.Color
	BorderColor     woxui.Color
	BorderWidth     float32
	Selected        bool
	Disabled        bool
	SkipFocus       bool
	OnTap           func()
	Child           woxwidget.Widget
	Theme           Theme
}

// WoxListItem builds a selectable row with shared pointer, keyboard, and accessibility behavior.
func WoxListItem(props ListItemProps) woxwidget.Widget {
	radius := props.Radius
	if radius == nil {
		defaultRadius := float32(7)
		radius = &defaultRadius
	}
	background := props.Theme.QueryBackground
	if props.Background != nil {
		background = *props.Background
	} else if props.Selected {
		background = props.Theme.SelectedBackground
	}
	onTap := props.OnTap
	if props.Disabled {
		onTap = nil
	}
	actions := []woxui.AccessibilityAction{woxui.AccessibilityActionActivate}
	if props.Disabled {
		actions = nil
	}
	key := woxwidget.Key(props.ID)
	buildContent := func(hovered bool, onHoverAt func(bool, woxui.Rect)) woxwidget.Widget {
		rowBackground := background
		if hovered && !props.Selected {
			if props.HoverBackground != nil {
				rowBackground = *props.HoverBackground
			} else {
				rowBackground = controlHoverColor(background, props.Theme.ResultTitle)
			}
		}
		return woxwidget.Gesture{ID: props.ID, OnTap: onTap, OnHoverAt: onHoverAt, Child: woxwidget.Container{
			Width: props.Width, Height: props.Height, Radius: *radius, Color: rowBackground, BorderColor: props.BorderColor, BorderWidth: props.BorderWidth, Padding: props.Padding, Child: props.Child,
		}}
	}
	var content woxwidget.Widget = buildContent(false, nil)
	if onTap != nil {
		content = hoverable(key, false, buildContent)
	}
	var child woxwidget.Widget = content
	if !props.SkipFocus {
		child = woxwidget.Focusable{Key: key, Disabled: props.Disabled, FocusRingColor: props.Theme.Cursor, FocusRingRadius: *radius, OnKey: func(event woxui.KeyEvent) bool {
			if event.Key != woxui.KeyEnter && event.Key != woxui.KeySpace {
				return false
			}
			if event.Down && onTap != nil {
				onTap()
			}
			return true
		}, Child: content}
	}
	return woxwidget.Semantics{
		Key: key, AutomationID: props.ID, Role: woxui.AccessibilityRoleListItem, Label: props.Label,
		Actions: actions, Disabled: props.Disabled, Selected: props.Selected,
		Child: child,
	}
}
