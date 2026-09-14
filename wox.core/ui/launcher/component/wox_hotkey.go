package component

import (
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// HotkeyProps describes a sequence of already formatted key labels.
type HotkeyProps struct {
	Toolbar    bool
	Primary    bool
	Selected   bool
	Theme      *Theme
	Labels     []string
	Foreground woxui.Color
	Background woxui.Color
	Border     woxui.Color
	FontSize   float32
	Compact    bool
	Window     *woxui.Window
}

// WoxHotkey builds shared keycaps and returns their total width.
func WoxHotkey(props HotkeyProps) (woxwidget.Widget, float32) {
	fontSize := props.FontSize
	if fontSize <= 0 {
		fontSize = TailFontSize
	}
	style := woxui.TextStyle{Size: fontSize, Weight: woxui.FontWeightSemibold}
	keyHeight, minWidth, horizontalInset := float32(22), float32(28), float32(14)
	// Themed launcher shortcuts are supporting hints; keep the unthemed recorder at its existing density.
	if props.Toolbar || props.Theme != nil {
		style.Weight = woxui.FontWeightRegular
		keyHeight, minWidth, horizontalInset = 20, 20, 10
	}
	border := props.Border
	if border.A == 0 {
		border = props.Foreground
	}
	// Select the surface/state tokens before applying them so explicit transparency survives.
	if props.Theme != nil {
		foreground, background, outline := props.Theme.ActionItemHotkeyFontColor, props.Theme.ActionItemHotkeyBackgroundColor, props.Theme.ActionItemHotkeyBorderColor
		if props.Toolbar {
			foreground, background, outline = props.Theme.ToolbarHotkeyFontColor, props.Theme.ToolbarHotkeyBackgroundColor, props.Theme.ToolbarHotkeyBorderColor
			if props.Primary {
				if props.Theme.ToolbarPrimaryHotkeyFontColor != nil {
					foreground = props.Theme.ToolbarPrimaryHotkeyFontColor
				}
				if props.Theme.ToolbarPrimaryHotkeyBackgroundColor != nil {
					background = props.Theme.ToolbarPrimaryHotkeyBackgroundColor
				}
				if props.Theme.ToolbarPrimaryHotkeyBorderColor != nil {
					outline = props.Theme.ToolbarPrimaryHotkeyBorderColor
				}
			}
		} else if props.Selected {
			foreground, background, outline = props.Theme.ActionItemActiveHotkeyFontColor, props.Theme.ActionItemActiveHotkeyBackgroundColor, props.Theme.ActionItemActiveHotkeyBorderColor
		}
		if foreground != nil {
			props.Foreground = *foreground
		}
		if background != nil {
			props.Background = *background
		}
		if outline != nil {
			border = *outline
		}
	}
	children := make([]woxwidget.Widget, 0, len(props.Labels))
	totalWidth := float32(0)
	for _, label := range props.Labels {
		metrics, _ := props.Window.MeasureText(label, style)
		width := max(minWidth, metrics.Size.Width+horizontalInset)
		children = append(children, woxwidget.Stack{Width: width, Height: keyHeight, Children: []woxwidget.StackChild{
			{Child: woxwidget.Container{Width: width, Height: keyHeight, Radius: 4, Color: props.Background}},
			{Child: woxwidget.Painter{Width: width, Height: keyHeight, Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
				displayList.StrokeRoundedRect(bounds, 4, 1, border)
			}}},
			{Child: woxwidget.Align{Width: width, Height: keyHeight, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Text{Value: label, Style: style, Color: props.Foreground}}},
		}})
		totalWidth += width
	}
	if len(children) > 1 {
		totalWidth += float32(len(children)-1) * 4
	}
	height := float32(28)
	padding := woxwidget.Insets{Top: 3, Bottom: 3}
	if props.Compact {
		height = 22
		padding = woxwidget.Insets{}
	}
	if props.Toolbar || props.Theme != nil {
		padding = woxwidget.Insets{}
	}
	return woxwidget.Container{Width: totalWidth, Height: height, Padding: padding, Child: woxwidget.Align{
		Width: totalWidth, Height: height - padding.Top - padding.Bottom, Vertical: 0.5,
		Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 4, Children: children},
	}}, totalWidth
}
