//go:build windows

package hotkey

import (
	"testing"
	"time"
	"wox/util/keyboard"
)

// TestCapsLockCallbackWaitsForConsumedRelease models keys hidden from GetAsyncKeyState by the hook.
func TestCapsLockCallbackWaitsForConsumedRelease(t *testing.T) {
	capsLockComboMu.Lock()
	original := capsLockComboState
	capsLockComboState = newCapsLockComboTracker()
	capsLockComboState.capsPressed = true
	capsLockComboMu.Unlock()
	defer func() {
		capsLockComboMu.Lock()
		capsLockComboState = original
		capsLockComboMu.Unlock()
	}()
	done := make(chan struct{})
	go func() {
		waitForCapsLockComboRelease(keyboard.KeyUnknown)
		close(done)
	}()
	select {
	case <-done:
		t.Fatal("callback ran while the raw hook would still consume Ctrl+C")
	case <-time.After(80 * time.Millisecond):
	}
	capsLockComboMu.Lock()
	capsLockComboState.capsPressed = false
	capsLockComboMu.Unlock()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("callback did not resume after release")
	}
}
