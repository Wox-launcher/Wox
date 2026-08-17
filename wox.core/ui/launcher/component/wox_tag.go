package component

import (
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// WoxTag builds the compact outlined label shared by settings lists.
// A 1-unit stroke and 2-unit vertical padding keep the outline inside a full
// pixel and clear of CJK glyph metrics, which otherwise clip the top edge.
func WoxTag(label string, color woxui.Color) woxwidget.Widget {
	return woxwidget.Container{
		Radius: 3, BorderColor: color, BorderWidth: 1,
		Padding: woxwidget.Insets{Left: 4, Top: 2, Right: 4, Bottom: 2},
		Child:   woxwidget.Text{Value: label, Style: woxui.TextStyle{Size: 11}, Color: color},
	}
}
