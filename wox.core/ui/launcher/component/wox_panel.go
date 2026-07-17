package component

import (
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// PanelProps describes the shared card surface used by Wox settings pages.
type PanelProps struct {
	Width       float32
	Height      float32
	Padding     woxwidget.Insets
	Radius      float32
	Color       woxui.Color
	BorderColor woxui.Color
	Child       woxwidget.Widget
	Theme       Theme
}

// WoxPanel builds one themed settings or popup surface.
func WoxPanel(props PanelProps) woxwidget.Widget {
	color := props.Color
	if color.A == 0 {
		color = props.Theme.QueryBackground
	}
	radius := props.Radius
	if radius <= 0 {
		radius = 8
	}
	return woxwidget.Container{
		Width: props.Width, Height: props.Height, Padding: props.Padding, Radius: radius,
		Color: color, BorderColor: props.BorderColor, BorderWidth: boolFloat(props.BorderColor.A != 0), Child: props.Child,
	}
}
