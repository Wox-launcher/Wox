package component

import (
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// HintBoxProps describes the shared informational banner used by settings surfaces.
type HintBoxProps struct {
	Text     string
	Width    float32
	MaxLines int
	Icon     *woxui.Image
	Accent   woxui.Color
	Theme    Theme
}

// WoxHintBox builds Flutter's compact blue-accent informational banner.
func WoxHintBox(props HintBoxProps) woxwidget.Widget {
	maxLines := max(1, props.MaxLines)
	backgroundAlpha := uint8(26)
	borderAlpha := uint8(77)
	if int(props.Theme.Background.R)*299+int(props.Theme.Background.G)*587+int(props.Theme.Background.B)*114 < 128000 {
		backgroundAlpha = 36
		borderAlpha = 89
	}
	var icon woxwidget.Widget = woxwidget.Painter{Width: 16, Height: 16}
	if props.Icon != nil {
		icon = woxwidget.Image{Source: props.Icon, Width: 16, Height: 16}
	}
	return woxwidget.Container{
		Width: props.Width, Radius: 10, Color: withAlpha(props.Accent, backgroundAlpha),
		BorderColor: withAlpha(props.Accent, borderAlpha), BorderWidth: 1, Padding: woxwidget.UniformInsets(12),
		Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, CrossAxisAlignment: woxwidget.CrossAxisStart, Children: []woxwidget.Widget{
			icon,
			woxwidget.TextBlock{Value: props.Text, Width: max(float32(0), props.Width-50), MaxLines: maxLines, LineHeight: 18, Style: woxui.TextStyle{Size: 13}, Color: props.Theme.ResultTitle},
		}},
	}
}
