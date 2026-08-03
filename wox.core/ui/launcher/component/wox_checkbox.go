package component

import (
	"fmt"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// CheckboxProps describes one compact Wox checkbox.
type CheckboxProps struct {
	ID       string
	Label    string
	Value    bool
	Disabled bool
	OnChange func(bool)
	Theme    Theme
}

// WoxCheckbox builds the Flutter-aligned 18px checkbox interaction.
func WoxCheckbox(props CheckboxProps) woxwidget.Widget {
	toggle := func() {
		if !props.Disabled && props.OnChange != nil {
			props.OnChange(!props.Value)
		}
	}
	border := props.Theme.ResultSubtitle
	background := woxui.Color{}
	var mark woxwidget.Widget
	if props.Value {
		border = props.Theme.ActionSelected
		background = props.Theme.ActionSelected
		mark = woxwidget.Align{Width: 18, Height: 18, Horizontal: 0.5, Vertical: 0.5, Child: CheckGlyph(12, props.Theme.ActionSelectedText)}
	}
	visual := woxwidget.Gesture{ID: props.ID, OnTap: toggle, Child: woxwidget.Container{
		Width: 18, Height: 18, Radius: 4, Color: background, BorderColor: border, BorderWidth: 1, Child: mark,
	}}
	if props.ID == "" || props.OnChange == nil {
		return visual.Child
	}
	actions := []woxui.AccessibilityAction{woxui.AccessibilityActionToggle}
	if props.Disabled {
		actions = nil
	}
	key := woxwidget.Key(props.ID)
	return woxwidget.Semantics{
		Key: key, AutomationID: props.ID, Role: woxui.AccessibilityRoleCheckBox, Label: props.Label,
		Actions: actions, Disabled: props.Disabled, Checked: props.Value,
		OnAction: func(action woxui.AccessibilityAction, _ string) error {
			if action != woxui.AccessibilityActionToggle && action != woxui.AccessibilityActionActivate {
				return fmt.Errorf("unsupported checkbox action %q", action)
			}
			toggle()
			return nil
		},
		Child: woxwidget.Focusable{Key: key, Disabled: props.Disabled, FocusRingColor: props.Theme.Cursor, FocusRingRadius: 4, OnKey: func(event woxui.KeyEvent) bool {
			if event.Key != woxui.KeyEnter && event.Key != woxui.KeySpace {
				return false
			}
			if event.Down {
				toggle()
			}
			return true
		}, Child: visual},
	}
}
