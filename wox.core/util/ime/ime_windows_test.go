//go:build windows

package ime

import "testing"

func TestTakeCapturedForegroundKeyboardLayoutClearsStoredValue(t *testing.T) {
	capturedForegroundKeyboardLayout.Lock()
	capturedForegroundKeyboardLayout.value = 0x04090409
	capturedForegroundKeyboardLayout.Unlock()

	if got := takeCapturedForegroundKeyboardLayout(); got != 0x04090409 {
		t.Fatalf("first take = %#x, want %#x", got, uintptr(0x04090409))
	}
	if got := takeCapturedForegroundKeyboardLayout(); got != 0 {
		t.Fatalf("second take = %#x, want 0", got)
	}
}
