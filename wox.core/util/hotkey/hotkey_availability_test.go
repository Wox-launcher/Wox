package hotkey

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"wox/util/keyboard"
)

func TestCheckHotkeyAvailabilityProbesWithoutRegistrationRetries(t *testing.T) {
	originalRegister := registerGlobalHotkey
	originalTry := tryRegisterGlobalHotkey
	t.Cleanup(func() {
		registerGlobalHotkey = originalRegister
		tryRegisterGlobalHotkey = originalTry
	})

	tryCalls := 0
	registerGlobalHotkey = func(keyboard.Modifier, keyboard.Key, func()) (keyboard.HotkeyRegistration, error) {
		t.Fatal("availability probe used the retrying registrar")
		return nil, nil
	}
	tryRegisterGlobalHotkey = func(keyboard.Modifier, keyboard.Key, func()) (keyboard.HotkeyRegistration, error) {
		tryCalls++
		return nil, fmt.Errorf("failed to register hotkey (err=1409)")
	}

	available, err := CheckHotkeyAvailability(context.Background(), "ctrl+shift+p")
	if available || err != nil {
		t.Fatalf("probe = available=%v err=%v, want a fast conflict", available, err)
	}
	if tryCalls != availabilityProbeMaxAttempts {
		t.Fatalf("probe attempts = %d, want %d one-shot registrations", tryCalls, availabilityProbeMaxAttempts)
	}
}

func TestCheckHotkeyAvailabilityPreservesBackendError(t *testing.T) {
	original := platformHotkeyAvailableCheck
	t.Cleanup(func() { platformHotkeyAvailableCheck = original })
	for _, backendErr := range []error{keyboard.ErrHotkeyConflict, keyboard.ErrGlobalHotkeysUnavailable, errors.New("unreadable configuration")} {
		platformHotkeyAvailableCheck = func(context.Context, string) (bool, bool, error) {
			return false, true, backendErr
		}
		available, err := CheckHotkeyAvailability(context.Background(), "Ctrl+K")
		if available || !errors.Is(err, backendErr) {
			t.Fatalf("lost backend error: available=%v err=%v want=%v", available, err, backendErr)
		}
	}
}
