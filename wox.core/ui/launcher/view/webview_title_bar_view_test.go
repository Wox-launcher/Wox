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
	for _, child := range titleBar.Children {
		if stateful, ok := child.Child.(woxwidget.Stateful); ok {
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
