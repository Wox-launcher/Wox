package preview

import (
	"testing"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestWebViewPreviewReportsOnlyPositiveBounds(t *testing.T) {
	var reported []woxui.Rect
	painter := WebViewPreview(WebViewPreviewProps{Width: 100, Height: 100, OnBounds: func(bounds woxui.Rect) {
		reported = append(reported, bounds)
	}}).(woxwidget.Painter)

	var displayList woxui.DisplayList
	painter.Paint(&displayList, woxui.Rect{Width: 100})
	painter.Paint(&displayList, woxui.Rect{Width: 100, Height: 100})

	if len(reported) != 1 || reported[0].Width != 100 || reported[0].Height != 100 {
		t.Fatalf("reported bounds = %v, want one 100x100 rect", reported)
	}
}
