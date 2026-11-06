package keyboard

import (
	"fmt"
	"wox/util"
)

// ReplayStandaloneTap is unavailable for passive Linux raw listeners; Hyprland
// uses its consuming subscription's ReplayTap instead.
func ReplayStandaloneTap(nativeCode uint32) error {
	return fmt.Errorf("standalone hold hotkeys are unavailable on Linux")
}

// ForegroundInputTarget is unavailable without intercepting input.
func ForegroundInputTarget() uint64 { return 0 }

// SupportsStandaloneHold reports whether the platform can consume and replay this key.
func SupportsStandaloneHold(key Key) bool {
	return util.IsHyprlandSession() && (key == KeyDelete || key == KeyInsert || key >= KeyF1 && key <= KeyF12)
}

// StandaloneHoldListenerReady reports whether the intercepting listener is active.
func StandaloneHoldListenerReady() bool { return false }
