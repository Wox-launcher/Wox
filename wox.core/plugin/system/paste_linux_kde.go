//go:build linux

package system

import (
	"context"
	"strings"
	"wox/plugin"
	"wox/util/keyboard"
	"wox/util/window"
)

// pasteToKDEWindow waits for the compositor's virtual keyboard before restoring focus.
func pasteToKDEWindow(ctx context.Context, target plugin.QueryEnv, focus func() error) error {
	shift := kdePasteUsesShift(window.GetProcessIdentity(target.ActiveWindowPid))
	return keyboard.SimulateKDEPasteAfterFocus(ctx, shift, focus)
}

func kdePasteUsesShift(identity string) bool {
	switch strings.TrimSuffix(strings.ToLower(identity), ".desktop") {
	case "org.kde.konsole", "konsole":
		return true
	default:
		return false
	}
}
