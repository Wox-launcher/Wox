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
	Theme    Theme
	OnTap    func()
}

// WoxSegmentedButton builds a shared compact option with selected and hover states.
func WoxSegmentedButton(props SegmentedButtonProps) woxwidget.Widget {
	background := woxui.Color{}
	foreground := props.Theme.ResultSubtitle
	if props.Selected {
		background = props.Theme.SelectedBackground
		foreground = props.Theme.SelectedTitle
	}
	if props.Disabled {
		foreground = withAlpha(foreground, 120)
	}
	onTap := props.OnTap
	if props.Disabled {
		onTap = nil
	}

	key := woxwidget.Key(props.ID)
	content := hoverable(key, props.Disabled, func(hovered bool, onHoverAt func(bool, woxui.Rect)) woxwidget.Widget {
		buttonBackground := background
		if hovered {
			buttonBackground = controlHoverColor(background, foreground)
		}
		return woxwidget.Gesture{ID: props.ID, OnTap: onTap, OnHoverAt: onHoverAt, Child: woxwidget.Container{
			Width: props.Width, Height: SettingsControlHeight, Radius: 6, Color: buttonBackground,
			Child: woxwidget.Align{Width: props.Width, Height: SettingsControlHeight, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Text{
				Value: props.Label, Style: woxui.TextStyle{Size: CompactButtonFontSize, Weight: woxui.FontWeightSemibold}, Color: foreground,
			}},
		}}
	})
	actions := []woxui.AccessibilityAction(nil)
	if onTap != nil {
		actions = []woxui.AccessibilityAction{woxui.AccessibilityActionActivate}
	}
	return woxwidget.Semantics{
		Key: key, AutomationID: props.ID, Role: woxui.AccessibilityRoleButton, Label: props.Label,
		Actions: actions, Disabled: props.Disabled, Selected: props.Selected,
		Child: woxwidget.Focusable{Key: key, Disabled: props.Disabled, FocusRingColor: props.Theme.Cursor, FocusRingRadius: 6, OnKey: func(event woxui.KeyEvent) bool {
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
