package preview

import (
	"testing"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestWebViewPreviewReportsOnlyPositiveBounds(t *testing.T) {
	var reported []woxui.Rect
	focusable := WebViewPreview(WebViewPreviewProps{Width: 100, Height: 100, OnBounds: func(bounds woxui.Rect) {
		reported = append(reported, bounds)
	}}).(woxwidget.Focusable)
	if focusable.Key != WebViewPreviewFocusKey || !focusable.SkipTraversal {
		t.Fatalf("WebView focus behavior = key %q skip traversal %v", focusable.Key, focusable.SkipTraversal)
	}
	gesture := focusable.Child.(woxwidget.Gesture)
	painter := gesture.Child.(woxwidget.Painter)

	var displayList woxui.DisplayList
	painter.Paint(&displayList, woxui.Rect{Width: 100})
	painter.Paint(&displayList, woxui.Rect{X: 32, Y: 48, Width: 100, Height: 100})

	want := woxui.Rect{X: 32, Y: 48, Width: 100, Height: 100}
	if len(reported) != 1 || reported[0] != want {
		t.Fatalf("reported bounds = %v, want %v", reported, want)
	}
}
