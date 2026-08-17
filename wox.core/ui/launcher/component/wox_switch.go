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
	buildVisual := func(hoverPosition float32) woxwidget.Widget {
		return woxwidget.AnimatedFloat{Key: key, Target: target, Duration: 300 * time.Millisecond, Curve: woxwidget.AnimationEaseOutBack, Builder: func(position float32) woxwidget.Widget {
			colorPosition := min(max(position, float32(0)), float32(1))
			trackColor := lerpColor(withAlpha(props.Theme.ResultTitle, 77), props.Theme.ActionSelected, colorPosition)
			hoverForeground := props.Theme.ResultTitle
			if props.Value {
				hoverForeground = props.Theme.ActionSelectedText
			}
			trackColor = lerpColor(trackColor, controlHoverColor(trackColor, hoverForeground), hoverPosition)
			// Rest-state track and thumb sizes stay on whole logical units.
			thumbSize := float32(10) + 4*colorPosition + 2*hoverPosition
			return woxwidget.Stack{Width: SettingsSwitchWidth, Height: 24, Children: []woxwidget.StackChild{
				{Left: 2, Top: 2, Child: woxwidget.Container{Width: 32, Height: 20, Radius: 10, Color: trackColor}},
				{Left: 12 + 12*position - thumbSize/2, Top: 12 - thumbSize/2, Child: woxwidget.Container{Width: thumbSize, Height: thumbSize, Radius: thumbSize / 2, Color: woxui.Color{R: 255, G: 255, B: 255, A: 255}}},
			}}
		}}
	}
	if props.ID == "" || props.OnChange == nil {
		return buildVisual(0)
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
		}, Child: hoverable(key, props.Disabled, func(hovered bool, onHoverAt func(bool, woxui.Rect)) woxwidget.Widget {
			hoverTarget := float32(0)
			if hovered {
				hoverTarget = 1
			}
			hoverAnimation := woxwidget.AnimatedFloat{
				Key: woxwidget.Key(props.ID + "-hover"), Target: hoverTarget, Duration: 120 * time.Millisecond, Curve: woxwidget.AnimationEaseInOutCubic,
				Builder: buildVisual,
			}
			return woxwidget.Gesture{ID: props.ID, OnTap: toggle, OnHoverAt: onHoverAt, Child: hoverAnimation}
		})},
	}
}

// lerpColor interpolates each RGBA channel between two colors.
func lerpColor(from, to woxui.Color, progress float32) woxui.Color {
	channel := func(start, end uint8) uint8 {
		return uint8(float32(start) + (float32(end)-float32(start))*progress + 0.5)
	}
	return woxui.Color{R: channel(from.R, to.R), G: channel(from.G, to.G), B: channel(from.B, to.B), A: channel(from.A, to.A)}
}
