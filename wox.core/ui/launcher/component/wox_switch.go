package component

import (
	"fmt"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// SwitchProps describes one Wox boolean switch.
type SwitchProps struct {
	ID       string
	Label    string
	Value    bool
	Disabled bool
	OnChange func(bool)
	Theme    Theme
}

// WoxSwitch builds a compact switch with pointer, keyboard, and accessibility behavior.
func WoxSwitch(props SwitchProps) woxwidget.Widget {
	trackColor := withAlpha(props.Theme.ResultSubtitle, 104)
	knobLeft := float32(2)
	if props.Value {
		trackColor = props.Theme.Cursor
		knobLeft = 22
	}
	if props.Disabled {
		trackColor = withAlpha(trackColor, 88)
	}
	toggle := func() {
		if !props.Disabled && props.OnChange != nil {
			props.OnChange(!props.Value)
		}
	}
	key := woxwidget.Key(props.ID)
	content := woxwidget.Gesture{ID: props.ID, OnTap: toggle, Child: woxwidget.Stack{Width: 42, Height: 22, Children: []woxwidget.StackChild{
		{Child: woxwidget.Container{Width: 42, Height: 22, Radius: 11, Color: trackColor}},
		{Left: knobLeft, Top: 2, Child: woxwidget.Container{Width: 18, Height: 18, Radius: 9, Color: woxui.Color{R: 248, G: 248, B: 248, A: 255}}},
	}}}
	if props.ID == "" || props.OnChange == nil {
		return content.Child
	}
	actions := []woxui.AccessibilityAction{woxui.AccessibilityActionToggle}
	if props.Disabled {
		actions = nil
	}
	return woxwidget.Semantics{
		Key: key, AutomationID: props.ID, Role: woxui.AccessibilityRoleCheckBox, Label: props.Label,
		Actions: actions, Disabled: props.Disabled, Checked: props.Value,
		OnAction: func(action woxui.AccessibilityAction, _ string) error {
			if action != woxui.AccessibilityActionToggle && action != woxui.AccessibilityActionActivate {
				return fmt.Errorf("unsupported switch action %q", action)
			}
			toggle()
			return nil
		},
		Child: woxwidget.Focusable{Key: key, Disabled: props.Disabled, OnKey: func(event woxui.KeyEvent) bool {
			if event.Key != woxui.KeyEnter && event.Key != woxui.KeySpace {
				return false
			}
			if event.Down {
				toggle()
			}
			return true
		}, Child: content},
	}
}
