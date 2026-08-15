package view

import (
	"testing"

	woxwidget "wox/ui/widget"
)

func TestWebViewTitleBarWindowsContainsNavigationAndCloseControls(t *testing.T) {
	titleBar := buildWebViewTitleBar(WebViewTitleBarProps{
		Width: 640, Platform: "windows", URL: "https://example.com",
		CanGoBack: true, CanGoForward: false,
	}, false, false, nil, nil).(woxwidget.Stack)

	controls := map[woxwidget.Key]bool{}
	for _, positioned := range titleBar.Children {
		child := positioned.Child
		if align, ok := child.(woxwidget.Align); ok {
			if positioned.Top != 0 || align.Height != SettingsTitleBarHeight || align.Vertical != 0.5 {
				t.Fatalf("WebView title-bar control alignment = top %.0f child %#v", positioned.Top, child)
			}
			child = align.Child
		}
		if stateful, ok := child.(woxwidget.Stateful); ok {
			controls[stateful.Key] = true
		}
	}
	for _, key := range []woxwidget.Key{"webview-go-back", "webview-refresh", "webview-go-forward", "webview-open-in-browser"} {
		if !controls[key] {
			t.Fatalf("Windows WebView title bar missing %q control", key)
		}
	}
	closeButton, ok := titleBar.Children[len(titleBar.Children)-1].Child.(woxwidget.Gesture)
	if !ok || closeButton.ID != "webview-window-close" {
		t.Fatalf("Windows WebView title bar close control = %#v, want webview-window-close", titleBar.Children[len(titleBar.Children)-1].Child)
	}
}
