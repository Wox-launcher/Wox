package component

import (
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// ButtonVariant selects one of the shared Wox button treatments.
type ButtonVariant uint8

const (
	ButtonSecondary ButtonVariant = iota
	ButtonPrimary
	ButtonOutline
	ButtonMuted
	ButtonSelected
	ButtonSurface
	ButtonText
)

// ButtonSize selects the standard geometry for a Wox button.
type ButtonSize uint8

const (
	ButtonNormal ButtonSize = iota
	ButtonCompact
)

// ButtonProps describes one themed, focusable Wox button.
type ButtonProps struct {
	ID       string
	Label    string
	Icon     *woxui.Image
	IconSize float32
	IconGap  float32
	Width    float32
	Height   float32
	Radius   float32
	Padding  woxwidget.Insets
	FontSize float32
	Disabled bool
	Variant  ButtonVariant
	Size     ButtonSize
	OnTap    func()
	Theme    Theme
}

// WoxButton builds a button with shared visuals, keyboard activation, and accessibility semantics.
func WoxButton(props ButtonProps) woxwidget.Widget {
	height := float32(38)
	radius := float32(8)
	padding := woxwidget.Insets{Left: 16, Right: 12}
	if props.Size == ButtonCompact {
		height = 30
		radius = 4
		padding = woxwidget.Insets{Left: 12, Right: 8}
	}
	if props.Height > 0 {
		height = props.Height
	}
	if props.Radius > 0 {
		radius = props.Radius
	}
	if props.Padding != (woxwidget.Insets{}) {
		padding = props.Padding
	}
	fontSize := props.FontSize
	if fontSize <= 0 {
		fontSize = 11
	}

	background := props.Theme.QueryBackground
	foreground := props.Theme.ActionText
	border := woxui.Color{}
	switch props.Variant {
	case ButtonPrimary:
		background = props.Theme.ActionSelected
		foreground = props.Theme.ActionSelectedText
	case ButtonOutline:
		background = woxui.Color{}
		foreground = props.Theme.ResultTitle
		border = props.Theme.ResultSubtitle
	case ButtonMuted:
		background = withAlpha(props.Theme.ResultSubtitle, 72)
		foreground = props.Theme.ResultTitle
	case ButtonSelected:
		background = props.Theme.SelectedBackground
		foreground = props.Theme.SelectedTitle
	case ButtonSurface:
		background = props.Theme.ActionBackground
		foreground = props.Theme.PreviewText
	case ButtonText:
		background = woxui.Color{}
		foreground = props.Theme.ResultTitle
	}

	onTap := props.OnTap
	if props.Disabled {
		foreground = withAlpha(foreground, 88)
		border = withAlpha(border, 88)
		onTap = nil
	}
	actions := []woxui.AccessibilityAction{woxui.AccessibilityActionActivate}
	if props.Disabled {
		actions = nil
	}
	key := woxwidget.Key(props.ID)
	var child woxwidget.Widget = woxwidget.Text{Value: props.Label, Style: woxui.TextStyle{Size: fontSize, Weight: woxui.FontWeightSemibold}, Color: foreground}
	if props.Icon != nil {
		iconSize := props.IconSize
		if iconSize <= 0 {
			iconSize = 16
		}
		iconGap := props.IconGap
		if iconGap <= 0 {
			iconGap = 6
		}
		child = woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: iconGap, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
			woxwidget.Image{Source: props.Icon, Width: iconSize, Height: iconSize},
			woxwidget.Text{Value: props.Label, Style: woxui.TextStyle{Size: fontSize, Weight: woxui.FontWeightSemibold}, Color: foreground},
		}}
	}
	// Center measured text and icon content inside the padded button box instead of relying on font-specific top offsets.
	content := woxwidget.Gesture{ID: props.ID, OnTap: onTap, Child: woxwidget.Container{
		Width: props.Width, Height: height, Radius: radius, Color: background, BorderColor: border, BorderWidth: boolFloat(border.A != 0), Padding: padding,
		Child: woxwidget.Align{Vertical: 0.5, Child: child},
	}}
	return woxwidget.Semantics{
		Key: key, AutomationID: props.ID, Role: woxui.AccessibilityRoleButton, Label: props.Label,
		Actions: actions, Disabled: props.Disabled,
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

func boolFloat(enabled bool) float32 {
	if enabled {
		return 1
	}
	return 0
}
