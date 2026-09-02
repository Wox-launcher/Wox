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
	ButtonOutlinedSurface
)

// ButtonProps describes one themed, focusable Wox button.
type ButtonProps struct {
	ID               string
	Label            string
	Icon             *woxui.Image
	TrailingIcon     *woxui.Image
	TrailingLabel    string
	IconSize         float32
	TrailingIconSize float32
	IconGap          float32
	// IntrinsicWidth sizes the button to its label/icon content. Omitted Width already does this;
	// keep the flag for call sites that want the intent to be explicit.
	IntrinsicWidth bool
	Width          float32
	Radius         float32
	Padding        woxwidget.Insets
	FontSize       float32
	// FontWeight overrides the default regular button label. Leave zero unless
	// a specific surface needs extra emphasis.
	FontWeight        woxui.FontWeight
	Disabled          bool
	Variant           ButtonVariant
	OnTap             func()
	OnTrailingHoverAt func(bool, woxui.Rect)
	OnFocusChange     func(bool)
	Theme             Theme
}

// WoxButton builds a button with shared visuals, keyboard activation, and accessibility semantics.
func WoxButton(props ButtonProps) woxwidget.Widget {
	height := float32(32)
	radius := float32(4)
	padding := woxwidget.Insets{Left: 12, Right: 12}
	fontSize := CompactButtonFontSize
	fontWeight := woxui.FontWeightRegular
	if props.FontWeight != 0 {
		fontWeight = props.FontWeight
	}
	if props.Radius > 0 {
		radius = props.Radius
	}
	if props.Padding != (woxwidget.Insets{}) {
		padding = props.Padding
	}
	if props.FontSize > 0 {
		fontSize = props.FontSize
	}
	// Keep a whole-unit label slot. fontSize*1.35 produced fractional padding and
	// left CJK ink off the button centerline.
	const labelLineHeight = float32(18)
	contentHeight := labelLineHeight

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
		border = props.Theme.ResultTitle
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
	case ButtonOutlinedSurface:
		background = props.Theme.QueryBackground
		foreground = props.Theme.ResultTitle
		border = props.Theme.ResultTitle
	}

	onTap := props.OnTap
	if props.Disabled {
		foreground = withAlpha(foreground, DisabledContentAlpha)
		border = withAlpha(border, DisabledContentAlpha)
		onTap = nil
	}
	actions := []woxui.AccessibilityAction{woxui.AccessibilityActionActivate}
	if props.Disabled {
		actions = nil
	}
	key := woxwidget.Key(props.ID)
	label := woxwidget.TextBlock{
		Value: props.Label, Height: labelLineHeight, LineHeight: labelLineHeight, MaxLines: 1, AlignmentY: 0.5, ShrinkWrap: true,
		Style: woxui.TextStyle{Size: fontSize, Weight: fontWeight}, Color: foreground,
	}
	var child woxwidget.Widget = label
	if props.Icon != nil {
		iconSize := props.IconSize
		if iconSize <= 0 {
			iconSize = 16
		}
		contentHeight = max(contentHeight, iconSize)
		iconGap := props.IconGap
		if iconGap <= 0 {
			iconGap = 8
		}
		child = woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: iconGap, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
			woxwidget.Image{Source: props.Icon, Width: iconSize, Height: iconSize},
			label,
		}}
	}
	if props.TrailingIcon != nil {
		iconSize := props.TrailingIconSize
		if iconSize <= 0 {
			iconSize = 15
		}
		contentHeight = max(contentHeight, iconSize)
		icon := woxwidget.Gesture{ID: props.ID + "-trailing", OnHoverAt: props.OnTrailingHoverAt, Child: woxwidget.Image{Source: props.TrailingIcon, Width: iconSize, Height: iconSize}}
		trailingLabel := props.TrailingLabel
		if trailingLabel == "" {
			trailingLabel = props.Label
		}
		child = woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 6, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
			child,
			woxwidget.Semantics{AutomationID: props.ID + "-trailing", Role: woxui.AccessibilityRoleButton, Label: trailingLabel, Child: icon},
		}}
	}
	// Match Flutter WoxButton: omitted width follows label/icon content. A zero-width Align child
	// would otherwise expand to the Flex parent's full available width and clip or stretch labels.
	intrinsicWidth := props.IntrinsicWidth || props.Width <= 0
	var alignedChild woxwidget.Widget = woxwidget.Align{Horizontal: 0.5, Vertical: 0.5, Child: child}
	buttonWidth := props.Width
	if intrinsicWidth {
		if padding.Top == 0 && padding.Bottom == 0 {
			verticalPadding := max(float32(0), (height-contentHeight)/2)
			padding.Top = verticalPadding
			padding.Bottom = verticalPadding
		}
		alignedChild = child
		buttonWidth = 0
	}
	// Intrinsic buttons cannot wrap content in Align: a zero-width Align expands to
	// the parent Flex width. Integer vertical padding around the 18px label slot
	// keeps the label on the centerline without fractional insets.
	content := hoverable(key, props.Disabled, func(hovered bool, onHoverAt func(bool, woxui.Rect)) woxwidget.Widget {
		buttonBackground := background
		if hovered {
			buttonBackground = controlHoverColor(background, foreground)
		}
		return woxwidget.Gesture{ID: props.ID, OnTap: onTap, OnHoverAt: onHoverAt, Child: woxwidget.Container{
			Width: buttonWidth, Height: height, Radius: radius, Color: buttonBackground, BorderColor: border, BorderWidth: boolFloat(border.A != 0), Padding: padding,
			Child: alignedChild,
		}}
	})
	return woxwidget.Semantics{
		Key: key, AutomationID: props.ID, Role: woxui.AccessibilityRoleButton, Label: props.Label,
		Actions: actions, Disabled: props.Disabled,
		Child: woxwidget.Focusable{Key: key, Disabled: props.Disabled, FocusRingColor: props.Theme.Cursor, FocusRingRadius: radius, OnKey: func(event woxui.KeyEvent) bool {
			if event.Key != woxui.KeyEnter && event.Key != woxui.KeySpace {
				return false
			}
			if event.Down && onTap != nil {
				onTap()
			}
			return true
		}, OnFocusChange: props.OnFocusChange, Child: content},
	}
}

func boolFloat(enabled bool) float32 {
	if enabled {
		return 1
	}
	return 0
}
