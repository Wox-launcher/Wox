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

func TestWebViewPreviewRoutesUnhandledEscape(t *testing.T) {
	escapeCalls := 0
	focusable := WebViewPreview(WebViewPreviewProps{Width: 100, Height: 100, OnEscape: func() { escapeCalls++ }}).(woxwidget.Focusable)
	if focusable.OnKey == nil {
		t.Fatal("WebView focus owner has no Escape handler")
	}
	if focusable.OnKey(woxui.KeyEvent{Key: woxui.KeyEscape}) || focusable.OnKey(woxui.KeyEvent{Key: woxui.KeyEnter, Down: true}) {
		t.Fatal("WebView Escape handler consumed an unrelated key transition")
	}
	if !focusable.OnKey(woxui.KeyEvent{Key: woxui.KeyEscape, Down: true}) || escapeCalls != 1 {
		t.Fatalf("WebView Escape result = calls %d, want one handled callback", escapeCalls)
	}
}
