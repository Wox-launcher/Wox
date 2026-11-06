package component

import (
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// SegmentedButtonProps describes one option in a compact segmented control.
type SegmentedButtonProps struct {
	ID       string
	Label    string
	Width    float32
	Selected bool
	Disabled bool
	Theme    ControlTheme
	Paint    *ControlPaint
	OnTap    func()
	OnKey    func(woxui.KeyEvent) bool
}

// WoxSegmentedButton builds a shared compact option with selected and hover states.
func WoxSegmentedButton(props SegmentedButtonProps) woxwidget.Widget {
	background := woxui.Color{}
	foreground := props.Theme.TextSecondary
	if props.Selected {
		background = props.Theme.SelectionBackground
		foreground = props.Theme.SelectionText
	}
	if props.Disabled {
		foreground = withAlpha(foreground, 120)
	}
	foreground = props.Paint.foreground(foreground)
	onTap := props.OnTap
	if props.Disabled {
		onTap = nil
	}

	key := woxwidget.Key(props.ID)
	height, radius := props.Theme.Scaled(SettingsControlHeight), props.Theme.Scaled(6)
	content := hoverable(key, props.Disabled, func(hovered bool, onHoverAt func(bool, woxui.Rect)) woxwidget.Widget {
		buttonBackground := background
		if hovered {
			buttonBackground = controlHoverColor(background, foreground)
		}
		return woxwidget.Gesture{ID: props.ID, OnTap: onTap, OnHoverAt: onHoverAt, Child: props.Paint.paintContainer(props.ID, woxwidget.Container{
			Width: props.Width, Height: height, Radius: radius, Color: buttonBackground,
			Child: woxwidget.Align{Width: props.Width, Height: height, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.TextBlock{
				Value: props.Label, MaxLines: 1, ShrinkWrap: true, Style: woxui.TextStyle{Size: props.Theme.Scaled(CompactButtonFontSize)}, Color: foreground,
			}},
		})}
	})
	actions := []woxui.AccessibilityAction(nil)
	if onTap != nil {
		actions = []woxui.AccessibilityAction{woxui.AccessibilityActionActivate}
	}
	return woxwidget.Semantics{
		Key: key, AutomationID: props.ID, Role: woxui.AccessibilityRoleButton, Label: props.Label,
		Actions: actions, Disabled: props.Disabled, Selected: props.Selected,
		Child: woxwidget.Focusable{Key: key, Disabled: props.Disabled, FocusRingColor: props.Theme.Focus, FocusRingRadius: radius, OnKey: func(event woxui.KeyEvent) bool {
			if props.Disabled || event.Composing || event.Modifiers != 0 {
				return false
			}
			if props.OnKey != nil && props.OnKey(event) {
				return true
			}
			if event.Key != woxui.KeyEnter && event.Key != woxui.KeySpace {
				return false
			}
			if event.Down && onTap != nil {
				onTap()
			}
			return onTap != nil
		}, Child: content},
	}
}
