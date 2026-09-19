//go:build windows

package hotkey

import (
	"context"
	"testing"
	"time"
	"wox/util/keyboard"
)

// TestCapsLockGroupReleasePolicy exercises registration and the actual hook callback,
// including held-key repeat suppression and the default synthetic-input guard.
func TestCapsLockGroupReleasePolicy(t *testing.T) {
	for _, name := range []string{"default", "denied", "immediate"} {
		t.Run(name, func(t *testing.T) {
			var handler keyboard.RawKeyHandler
			restore := replaceRawKeyListenerForTest(t, func(callback keyboard.RawKeyHandler) (keyboard.RawKeySubscription, error) {
				handler = callback
				return noopRawKeySubscription{}, nil
			})
			defer restore()
			done := make(chan struct{}, 2)
			spec := Spec{CombineKey: "capslock+a", Callback: func() { done <- struct{}{} }}
			if name != "default" {
				spec.CanTriggerBeforeRelease = func() bool { return name == "immediate" }
			}
			group, err := RegisterGroup(context.Background(), []Spec{spec})
			if err != nil {
				t.Fatal(err)
			}
			defer group.Unregister(context.Background())
			if handler == nil {
				t.Fatal("Caps listener was not registered")
			}
			capsLockComboMu.Lock()
			capsLockComboState.capsPressed = true
			capsLockComboMu.Unlock()
			event := keyboard.RawKeyEvent{Type: keyboard.EventTypeKeyDown, Key: keyboard.KeyA}
			if !handler(event) || !handler(event) {
				t.Fatal("Caps combo and its repeat must be consumed")
			}
			fired := false
			select {
			case <-done:
				fired = true
			case <-time.After(100 * time.Millisecond):
			}
			capsLockComboMu.Lock()
			capsLockComboState.capsPressed = false
			capsLockComboMu.Unlock()
			if fired != (name == "immediate") {
				t.Errorf("callback before release = %t", fired)
			}
			if !fired {
				select {
				case <-done:
				case <-time.After(2 * time.Second):
					t.Fatal("callback did not finish after release")
				}
			}
			select {
			case <-done:
				t.Fatal("held-key repeat triggered another callback")
			case <-time.After(30 * time.Millisecond):
			}
		})
	}
}

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
