//go:build windows

package woxui

import (
	"testing"

	"github.com/lxn/win"
)

func TestWebViewCursorOverridesHostOnlyWhilePointerIsOverSurface(t *testing.T) {
	const webViewCursor = win.HCURSOR(123)
	window := &platformWindow{
		pointerCursor:      PointerCursorText,
		webViewCursor:      webViewCursor,
		webViewCursorKnown: true,
		webViewPointerOver: true,
	}

	if actual := window.resolvedPointerCursor(); actual != webViewCursor {
		t.Fatalf("WebView cursor = %v, want %v", actual, webViewCursor)
	}
	window.webViewCursor = 0
	if actual := window.resolvedPointerCursor(); actual != 0 {
		t.Fatalf("CSS cursor:none = %v, want no cursor", actual)
	}
	window.webViewCursor = webViewCursor

	window.webViewPointerOver = false
	if actual := window.resolvedPointerCursor(); actual == webViewCursor {
		t.Fatal("WebView cursor remained active after the pointer left its surface")
	}

	window.webViewPointerOver = true
	window.clearWebViewPointerState()
	if actual := window.resolvedPointerCursor(); actual == webViewCursor {
		t.Fatal("WebView cursor remained active after clearing the embedded surface state")
	}
}
