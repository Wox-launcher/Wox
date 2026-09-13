//go:build darwin

package woxui

import "testing"

// TestSelectionIgnoresStaleDarwinFocus prevents cached focus from suppressing OS capture.
func TestSelectionIgnoresStaleDarwinFocus(t *testing.T) {
	for _, value := range []string{"", "stale"} {
		t.Run("selection="+value, func(t *testing.T) {
			window := &platformWindow{}
			accessibilityWindows.Store(window, accessibilityWindowState{tree: AccessibilityTree{
				WindowFocused: true,
				Nodes: []AccessibilityNode{{
					Focused: true, HasTextSelection: true, Value: value, SelectionEnd: len(value),
				}},
			}})
			t.Cleanup(func() { accessibilityWindows.Delete(window) })
			if text, handled := SelectedTextFromFocusedWindow(); handled || text != "" {
				t.Fatalf("stale focus blocked OS selection: text=%q handled=%t", text, handled)
			}
		})
	}
}
