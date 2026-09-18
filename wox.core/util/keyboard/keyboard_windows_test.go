package keyboard

import "testing"

// TestWindowsTypingRejectsMultiline verifies no partial message reaches SendInput.
func TestWindowsTypingRejectsMultiline(t *testing.T) {
	for _, text := range []string{"first\nsecond", "first\r\nsecond", "first\rsecond", "\n"} {
		if err := simulateType(text); err == nil {
			t.Errorf("multiline input %q must require paste", text)
		}
	}
}
