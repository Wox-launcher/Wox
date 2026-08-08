package component

import (
	"fmt"
	"time"

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
	toggle := func() {
		if !props.Disabled && props.OnChange != nil {
			props.OnChange(!props.Value)
		}
	}
	key := woxwidget.Key(props.ID)
	target := float32(0)
	if props.Value {
		target = 1
	}
	visual := woxwidget.AnimatedFloat{Key: key, Target: target, Duration: 300 * time.Millisecond, Curve: woxwidget.AnimationEaseOutBack, Builder: func(position float32) woxwidget.Widget {
		colorPosition := min(max(position, float32(0)), float32(1))
		trackColor := lerpColor(withAlpha(props.Theme.ResultTitle, 77), props.Theme.ActionSelected, colorPosition)
		thumbSize := 9.6 + 4.8*colorPosition
		return woxwidget.Stack{Width: 36, Height: 24, Children: []woxwidget.StackChild{
			{Left: 2.4, Top: 2.4, Child: woxwidget.Container{Width: 31.2, Height: 19.2, Radius: 9.6, Color: trackColor}},
			{Left: 12 + 12*position - thumbSize/2, Top: 12 - thumbSize/2, Child: woxwidget.Container{Width: thumbSize, Height: thumbSize, Radius: thumbSize / 2, Color: woxui.Color{R: 255, G: 255, B: 255, A: 255}}},
		}}
	}}
	content := woxwidget.Gesture{ID: props.ID, OnTap: toggle, Child: visual}
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
		Child: woxwidget.Focusable{Key: key, Disabled: props.Disabled, FocusRingColor: props.Theme.Cursor, FocusRingRadius: 12, OnKey: func(event woxui.KeyEvent) bool {
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

// lerpColor interpolates each RGBA channel between two colors.
func lerpColor(from, to woxui.Color, progress float32) woxui.Color {
	channel := func(start, end uint8) uint8 {
		return uint8(float32(start) + (float32(end)-float32(start))*progress + 0.5)
	}
	return woxui.Color{R: channel(from.R, to.R), G: channel(from.G, to.G), B: channel(from.B, to.B), A: channel(from.A, to.A)}
}
