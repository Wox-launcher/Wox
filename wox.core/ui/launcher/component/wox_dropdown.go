package component

import (
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// DropdownProps describes one accessible outlined dropdown trigger.
type DropdownProps struct {
	ID            string
	Label         string
	Value         string
	Trailing      string
	Leading       *woxui.Image
	Width         float32
	Height        float32
	Outline       woxui.Color
	Foreground    woxui.Color
	Secondary     woxui.Color
	Theme         Theme
	Focused       bool
	OnKey         func(woxui.KeyEvent) bool
	OnFocusChange func(bool)
	OnTap         func()
	OnTapBounds   func(woxui.Rect)
}

// WoxDropdown builds a focusable dropdown trigger with shared visuals and accessibility semantics.
func WoxDropdown(props DropdownProps) woxwidget.Widget {
	if props.Height <= 0 {
		props.Height = SettingsControlHeight
	}
	disabled := props.OnTap == nil && props.OnTapBounds == nil
	key := woxwidget.Key(props.ID)
	trigger := hoverable(key, disabled, func(hovered bool, onHoverAt func(bool, woxui.Rect)) woxwidget.Widget {
		return woxDropdownTrigger(props, hovered, onHoverAt)
	})
	actions := []woxui.AccessibilityAction(nil)
	if !disabled {
		actions = []woxui.AccessibilityAction{woxui.AccessibilityActionActivate}
	}
	label := props.Label
	if label == "" {
		label = props.Value
	}
	return woxwidget.Semantics{
		Key: key, AutomationID: props.ID, Role: woxui.AccessibilityRoleButton, Label: label, Value: props.Value,
		Actions: actions, Disabled: disabled, Child: woxwidget.Focusable{
			Key: key, Autofocus: props.Focused, Disabled: disabled, FocusRingColor: props.Theme.Cursor, FocusRingRadius: 4,
			OnKey: props.OnKey, OnFocusChange: props.OnFocusChange, Child: trigger,
		},
	}
}

func woxDropdownTrigger(props DropdownProps, hovered bool, onHoverAt func(bool, woxui.Rect)) woxwidget.Widget {
	const horizontalPadding = float32(8)
	const indicatorWidth = float32(24)
	contentWidth := max(float32(0), props.Width-horizontalPadding*2-indicatorWidth)
	children := make([]woxwidget.Widget, 0, 5)
	if props.Leading != nil {
		children = append(children,
			woxwidget.Align{Width: 18, Height: props.Height, Vertical: 0.5, Child: woxwidget.Image{Source: props.Leading, Width: 18, Height: 18}},
			woxwidget.Container{Width: 8, Height: props.Height},
		)
		contentWidth = max(float32(0), contentWidth-26)
	}
	trailingWidth := float32(0)
	if props.Trailing != "" {
		trailingWidth = min(float32(80), max(float32(0), contentWidth-60))
		contentWidth = max(float32(0), contentWidth-trailingWidth-10)
	}
	children = append(children, woxwidget.Align{Width: contentWidth, Height: props.Height, Vertical: 0.5, Child: woxwidget.TextBlock{
		Value: props.Value, Width: contentWidth, Height: 18, LineHeight: 18, MaxLines: 1, Style: woxui.TextStyle{Size: SettingsControlFontSize}, Color: props.Foreground,
	}})
	if trailingWidth > 0 {
		secondary := props.Secondary
		if secondary.A == 0 {
			secondary = props.Foreground
		}
		children = append(children,
			woxwidget.Container{Width: 10, Height: props.Height},
			woxwidget.Align{Width: trailingWidth, Height: props.Height, Horizontal: 1, Vertical: 0.5, Child: woxwidget.Text{
				Value: props.Trailing, Style: woxui.TextStyle{Size: SettingsSecondaryFontSize}, Color: secondary,
			}},
		)
	}
	children = append(children, WoxDropdownIndicator(indicatorWidth, props.Height, props.Foreground))
	outline := props.Outline
	if outline.A == 0 {
		// Keep the outline on the value text token so ResultSubtitle cannot restyle Settings chrome.
		outline = withAlpha(props.Foreground, 140)
	}
	background := woxui.Color{}
	if hovered {
		background = controlHoverColor(background, props.Foreground)
	}
	return woxwidget.Gesture{ID: props.ID, OnTap: props.OnTap, OnTapBounds: props.OnTapBounds, OnHoverAt: onHoverAt, Child: woxwidget.Container{
		Width: props.Width, Height: props.Height, Radius: 4, Color: background, BorderColor: outline, BorderWidth: 1,
		Padding: woxwidget.Insets{Left: horizontalPadding, Right: horizontalPadding},
		Child:   woxwidget.Flex{Axis: woxwidget.Horizontal, Children: children},
	}}
}

// WoxDropdownIndicator builds the shared dropdown arrow without its trigger surface.
func WoxDropdownIndicator(width, height float32, color woxui.Color) woxwidget.Widget {
	return woxwidget.Painter{Width: width, Height: height, Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
		triangleWidth := min(float32(10), bounds.Width)
		triangleHeight := min(float32(6), bounds.Height)
		left := bounds.X + (bounds.Width-triangleWidth)/2
		top := bounds.Y + (bounds.Height-triangleHeight)/2
		displayList.FillConvexPolygon([]woxui.Point{
			{X: left, Y: top},
			{X: left + triangleWidth, Y: top},
			{X: left + triangleWidth/2, Y: top + triangleHeight},
		}, color)
	}}
}
