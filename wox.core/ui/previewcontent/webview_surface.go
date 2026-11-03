package previewcontent

import (
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// WebViewSurfaceProps supplies the layout and input routing for an embedded browser.
type WebViewSurfaceProps struct {
	Key                         woxwidget.Key
	Width, Height, CornerRadius float32
	Background                  woxui.Color
	OnBounds                    func(woxui.Rect)
	OnPointer                   func(woxui.PointerEvent) bool
	OnEscape                    func()
}

// WebViewSurface places the native browser within a portable widget host.
func WebViewSurface(props WebViewSurfaceProps) woxwidget.Widget {
	return woxwidget.Focusable{
		Key: props.Key, SkipTraversal: true,
		OnKey: func(event woxui.KeyEvent) bool {
			if !event.Down || event.Repeat || event.Composing || event.Modifiers != 0 || event.Key != woxui.KeyEscape || props.OnEscape == nil {
				return false
			}
			props.OnEscape()
			return true
		},
		Child: woxwidget.Gesture{ID: "webview-preview-input", OnPointer: props.OnPointer, Child: woxwidget.Painter{Width: props.Width, Height: props.Height, Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
			displayList.FillRoundedRect(bounds, props.CornerRadius, props.Background)
			displayList.DeferEmbeddedSurfaceOverlay(bounds)
			if props.OnBounds != nil && bounds.Width > 0 && bounds.Height > 0 {
				props.OnBounds(bounds)
			}
		}}},
	}
}
