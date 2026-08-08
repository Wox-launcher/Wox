package component

import (
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// WoxTag builds the compact outlined label shared by settings lists.
func WoxTag(label string, color woxui.Color) woxwidget.Widget {
	return woxwidget.Container{
		Radius: 3, BorderColor: color, BorderWidth: 0.5,
		Padding: woxwidget.Insets{Left: 4, Top: 1, Right: 4, Bottom: 1},
		Child:   woxwidget.Text{Value: label, Style: woxui.TextStyle{Size: 11}, Color: color},
	}
}
