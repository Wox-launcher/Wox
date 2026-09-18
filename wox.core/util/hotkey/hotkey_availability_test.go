package hotkey

import (
	"context"
	"errors"
	"testing"
	"wox/util/keyboard"
)

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
