//go:build windows

package woxui

import "testing"

// TestSelectionIgnoresStaleWindowsFocus ensures cached editor text cannot suppress OS capture.
func TestSelectionIgnoresStaleWindowsFocus(t *testing.T) {
	window := &platformWindow{}
	accessibilityWindows.Store(window, accessibilityWindowState{tree: AccessibilityTree{
		WindowFocused: true,
		Nodes:         []AccessibilityNode{{Focused: true, HasTextSelection: true, Value: "stale", SelectionEnd: 5}},
	}})
	t.Cleanup(func() { accessibilityWindows.Delete(window) })
	if text, handled := SelectedTextFromFocusedWindow(); handled || text != "" {
		t.Fatalf("stale focus blocked OS selection: text=%q handled=%t", text, handled)
	}
}
