package woxui

import (
	"testing"

	webviewruntime "wox/ui/runtime/internal/webview"
)

func TestWebViewContractConversionsPreserveFields(t *testing.T) {
	content := WebViewContent{URL: "https://example.com", HTML: "<p>preview</p>", InjectCSS: "body{}", CacheDisabled: true, CacheKey: "preview"}
	if roundTrip := fromWebViewContent(toWebViewContent(content)); roundTrip != content {
		t.Fatalf("content round trip = %+v, want %+v", roundTrip, content)
	}
	state := fromWebViewNavigationState(webviewruntime.NavigationState{URL: "https://example.com", CanGoBack: true, CanGoForward: true})
	if state != (WebViewNavigationState{URL: "https://example.com", CanGoBack: true, CanGoForward: true}) {
		t.Fatalf("navigation state = %+v", state)
	}
}

func TestIsAbsoluteWebViewURL(t *testing.T) {
	for _, test := range []struct {
		value string
		valid bool
	}{
		{value: "https://example.com/path", valid: true},
		{value: "http://localhost:8080", valid: true},
		{value: "example.com/path", valid: false},
		{value: "file:///tmp/index.html", valid: false},
		{value: "https:///missing-host", valid: false},
	} {
		if actual := isAbsoluteWebViewURL(test.value); actual != test.valid {
			t.Fatalf("isAbsoluteWebViewURL(%q) = %t, want %t", test.value, actual, test.valid)
		}
	}
}
