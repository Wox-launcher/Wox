package preview

import (
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestWebViewPreviewLoadingCentersSharedIndicator(t *testing.T) {
	theme := woxcomponent.Theme{QueryBackground: woxui.Color{R: 20, G: 24, B: 28, A: 255}, PreviewText: woxui.Color{R: 220, G: 230, B: 240, A: 255}}
	surface := WebViewPreviewLoading(theme, 320, 180).(woxwidget.Container)
	if surface.Width != 320 || surface.Height != 180 || surface.Color != theme.QueryBackground {
		t.Fatalf("WebView loading surface = %#v, want the deferred preview shell", surface)
	}
	loading := surface.Child.(woxwidget.Align)
	if loading.Horizontal != 0.5 || loading.Vertical != 0.5 {
		t.Fatalf("WebView loading align = %#v, want a centered placeholder", loading)
	}
	if indicator := loading.Child.(woxwidget.LoopAnimation); indicator.Key != "wox-loading-indicator" {
		t.Fatalf("WebView loading child = %#v, want WoxLoadingIndicator", indicator)
	}
}

func TestWebViewPreviewCornerRadiusStaysConcentricWithPreviewShell(t *testing.T) {
	if WebViewPreviewCornerRadius != previewSurfaceRadius-previewSurfaceBorderWidth {
		t.Fatalf("WebView radius = %v, want preview shell %v minus %v border", WebViewPreviewCornerRadius, previewSurfaceRadius, previewSurfaceBorderWidth)
	}
}

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
