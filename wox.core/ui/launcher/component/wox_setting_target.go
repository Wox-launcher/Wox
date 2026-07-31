package component

import (
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// SettingTargetProps describes the temporary destination cue used by Settings search.
type SettingTargetProps struct {
	Width       float32
	Height      float32
	Highlighted bool
	Child       woxwidget.Widget
	Theme       Theme
}

// WoxSettingTarget keeps the search cue local to the destination without changing its layout.
func WoxSettingTarget(props SettingTargetProps) woxwidget.Widget {
	background := woxui.Color{}
	border := woxui.Color{}
	if props.Highlighted {
		background = props.Theme.SelectedBackground
		background.A = 31
		border = props.Theme.SelectedBackground
		border.A = 87
	}
	return woxwidget.Container{
		Width: props.Width, Height: props.Height, Radius: 6, Color: background, BorderColor: border, BorderWidth: 1, Child: props.Child,
	}
}
