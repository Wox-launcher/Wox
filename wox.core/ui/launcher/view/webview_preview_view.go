package view

import (
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// WebViewPreviewProps contains the native surface placement callback.
type WebViewPreviewProps struct {
	Width    float32
	Height   float32
	Theme    woxcomponent.Theme
	OnBounds func(woxui.Rect)
}

// WebViewPreview paints the portable backdrop and reports native content bounds.
func WebViewPreview(props WebViewPreviewProps) woxwidget.Widget {
	return woxwidget.Painter{Width: props.Width, Height: props.Height, Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
		displayList.FillRoundedRect(bounds, 10, props.Theme.QueryBackground)
		if props.OnBounds != nil {
			props.OnBounds(bounds)
		}
	}}
}

// WebViewPreviewMessage builds a portable WebView error or loading surface.
func WebViewPreviewMessage(message string, color woxui.Color, theme woxcomponent.Theme, width, height float32) woxwidget.Widget {
	return woxwidget.Container{Width: width, Height: height, Radius: 10, Color: theme.QueryBackground, Padding: woxwidget.UniformInsets(14), Child: woxwidget.TextBlock{
		Value: message, Width: max(float32(0), width-28), Height: max(float32(0), height-28), Style: woxui.TextStyle{Size: 13}, Color: color,
	}}
}
