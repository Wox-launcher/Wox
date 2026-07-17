package component

import (
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// ListItemProps describes one selectable Wox list row.
type ListItemProps struct {
	ID       string
	Label    string
	Width    float32
	Height   float32
	Radius   float32
	Padding  woxwidget.Insets
	Selected bool
	Disabled bool
	OnTap    func()
	Child    woxwidget.Widget
	Theme    Theme
}

// WoxListItem builds a selectable row with shared pointer, keyboard, and accessibility behavior.
func WoxListItem(props ListItemProps) woxwidget.Widget {
	radius := props.Radius
	if radius <= 0 {
		radius = 7
	}
	background := props.Theme.QueryBackground
	if props.Selected {
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
	content := woxwidget.Gesture{ID: props.ID, OnTap: onTap, Child: woxwidget.Container{
		Width: props.Width, Height: props.Height, Radius: radius, Color: background, Padding: props.Padding, Child: props.Child,
	}}
	return woxwidget.Semantics{
		Key: key, AutomationID: props.ID, Role: woxui.AccessibilityRoleListItem, Label: props.Label,
		Actions: actions, Disabled: props.Disabled, Selected: props.Selected,
		Child: woxwidget.Focusable{Key: key, Disabled: props.Disabled, OnKey: func(event woxui.KeyEvent) bool {
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
