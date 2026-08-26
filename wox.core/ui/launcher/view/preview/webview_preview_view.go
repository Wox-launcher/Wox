package preview

import (
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// WebViewPreviewFocusKey identifies the native browser as a Host focus owner.
const WebViewPreviewFocusKey woxwidget.Key = "webview-preview-focus"

// WebViewPreviewCornerRadius keeps the inset native surface concentric with the preview shell.
const WebViewPreviewCornerRadius = previewSurfaceRadius - previewSurfaceBorderWidth

// WebViewPreviewProps contains the native surface placement callback.
type WebViewPreviewProps struct {
	Width     float32
	Height    float32
	Theme     woxcomponent.Theme
	OnBounds  func(woxui.Rect)
	OnPointer func(woxui.PointerEvent) bool
	OnEscape  func()
}

// WebViewPreview paints the portable backdrop and reports native content bounds.
func WebViewPreview(props WebViewPreviewProps) woxwidget.Widget {
	return woxwidget.Focusable{
		Key: WebViewPreviewFocusKey, SkipTraversal: true,
		OnKey: func(event woxui.KeyEvent) bool {
			if !event.Down || event.Repeat || event.Composing || event.Modifiers != 0 || event.Key != woxui.KeyEscape {
				return false
			}
			if props.OnEscape == nil {
				return false
			}
			props.OnEscape()
			return true
		},
		Child: woxwidget.Gesture{ID: "webview-preview-input", OnPointer: props.OnPointer, Child: woxwidget.Painter{Width: props.Width, Height: props.Height, Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
			displayList.FillRoundedRect(bounds, WebViewPreviewCornerRadius, props.Theme.QueryBackground)
			displayList.BeginEmbeddedSurfaceOverlay(bounds)
			if props.OnBounds != nil && bounds.Width > 0 && bounds.Height > 0 {
				props.OnBounds(bounds)
			}
		}}},
	}
}

// WebViewPreviewMessage builds a portable WebView error or unavailable-state surface.
func WebViewPreviewMessage(message string, color woxui.Color, theme woxcomponent.Theme, width, height float32) woxwidget.Widget {
	return woxwidget.Container{Width: width, Height: height, Radius: 10, Color: theme.QueryBackground, Padding: woxwidget.UniformInsets(14), Child: woxwidget.TextBlock{
		Value: message, Width: max(float32(0), width-28), Height: max(float32(0), height-28), Style: woxui.TextStyle{Size: 13}, Color: color,
	}}
}

// WebViewPreviewLoading mirrors Flutter's centered deferred-preview indicator without exposing implementation text.
func WebViewPreviewLoading(theme woxcomponent.Theme, width, height float32) woxwidget.Widget {
	return woxwidget.Container{Width: width, Height: height, Radius: 10, Color: theme.QueryBackground, Child: PreviewLoading(width, height, theme.PreviewText)}
}
