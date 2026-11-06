package keyboard

import "fmt"

// ReplayStandaloneTap is unavailable because Linux raw listeners are passive.
func ReplayStandaloneTap(nativeCode uint32) error {
	return fmt.Errorf("standalone hold hotkeys are unavailable on Linux")
}

// ForegroundInputTarget is unavailable without intercepting input.
func ForegroundInputTarget() uint64 { return 0 }

// SupportsStandaloneHold reports whether the platform can consume and replay this key.
func SupportsStandaloneHold(key Key) bool {
	return false
}

// StandaloneHoldListenerReady reports whether the intercepting listener is active.
func StandaloneHoldListenerReady() bool { return false }
