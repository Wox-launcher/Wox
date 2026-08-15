package component

import (
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// WoxSettingTextField applies the shared Flutter-compatible appearance for settings inputs.
func WoxSettingTextField(props TextFieldProps) woxwidget.Widget {
	props.Height = SettingsControlHeight
	props.Radius = 4
	props.Padding = woxwidget.Insets{Left: 8, Top: 6, Right: 8, Bottom: 6}
	props.Background = woxui.Color{}
	props.Transparent = true
	props.BorderColor = props.Theme.ResultSubtitle
	props.BorderWidth = 1
	props.Style = woxui.TextStyle{Size: SettingsControlFontSize}
	props.TextColor = props.Theme.ResultTitle
	if props.Disabled {
		props.TextColor = props.Theme.ResultSubtitle
	}
	props.TextAlignmentY = 0.5
	props.MaxLines = 1
	return WoxTextField(props)
}
