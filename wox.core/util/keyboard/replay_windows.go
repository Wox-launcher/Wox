package keyboard

/*
#include <windows.h>
#include <stdint.h>
int woxKeyboardReplayTap(unsigned int vkCode);
static uintptr_t woxForegroundInputTarget(void) { return (uintptr_t)GetForegroundWindow(); }
*/
import "C"

import "fmt"

// ReplayStandaloneTap restores a consumed short press to the foreground application.
func ReplayStandaloneTap(nativeCode uint32) error {
	if C.woxKeyboardReplayTap(C.uint(nativeCode)) == 0 {
		return fmt.Errorf("failed to replay key tap")
	}
	return nil
}

// ForegroundInputTarget identifies the window that would receive replayed input.
func ForegroundInputTarget() uint64 {
	return uint64(C.woxForegroundInputTarget())
}

// SupportsStandaloneHold reports whether the platform can consume and replay this key.
func SupportsStandaloneHold(key Key) bool {
	_, err := keyToWindowsVK(key)
	return err == nil
}

// StandaloneHoldListenerReady reports whether the intercepting listener is active.
func StandaloneHoldListenerReady() bool { return true }
