package component

import (
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// HotkeyProps describes a sequence of already formatted key labels.
type HotkeyProps struct {
	Labels     []string
	Foreground woxui.Color
	Background woxui.Color
	Border     woxui.Color
	Compact    bool
	Window     *woxui.Window
}

// WoxHotkey builds shared keycaps and returns their total width.
func WoxHotkey(props HotkeyProps) (woxwidget.Widget, float32) {
	style := woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold}
	border := props.Border
	if border.A == 0 {
		border = props.Foreground
	}
	children := make([]woxwidget.Widget, 0, len(props.Labels))
	totalWidth := float32(0)
	for _, label := range props.Labels {
		metrics, _ := props.Window.MeasureText(label, style)
		width := max(float32(28), metrics.Size.Width+14)
		children = append(children, woxwidget.Stack{Width: width, Height: 22, Children: []woxwidget.StackChild{
			{Child: woxwidget.Container{Width: width, Height: 22, Radius: 4, Color: props.Background}},
			{Child: woxwidget.Painter{Width: width, Height: 22, Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
				displayList.StrokeRoundedRect(bounds, 4, 1, border)
			}}},
			{Left: max(float32(0), (width-metrics.Size.Width)/2), Top: max(float32(0), (float32(22)-metrics.Size.Height)/2), Child: woxwidget.Text{Value: label, Style: style, Color: props.Foreground}},
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
	return woxwidget.Container{Width: totalWidth, Height: height, Padding: padding, Child: woxwidget.Flex{
		Axis: woxwidget.Horizontal, Gap: 4, Children: children,
	}}, totalWidth
}
