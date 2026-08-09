//go:build wox_ui_smoke && !windows && !darwin

package hotkey

import (
	"context"
	"testing"
)

func requireIgnoredHotkeyAppRuntime(t *testing.T) {
	t.Helper()
	t.Skip("ignored hotkey app runtime smoke requires Windows foreground-window and global-hotkey injection")
}

func ignoredHotkeyAppTarget(t *testing.T) (string, string) {
	t.Helper()
	t.Skip("ignored hotkey app runtime smoke has no deterministic Linux system editor")
	return "", ""
}

func ignoredHotkeyAppHotkey(t *testing.T) string {
	t.Helper()
	t.Skip("ignored hotkey app runtime smoke has no deterministic Linux hotkey injection")
	return ""
}

func sendIgnoredAppNativeHotkey(t *testing.T, _ string) {
	t.Helper()
	t.Skip("ignored hotkey app runtime smoke has no native Linux hotkey injection")
}

func activateIgnoredHotkeyApp(t *testing.T, _ context.Context) {
	t.Helper()
	t.Skip("ignored hotkey app runtime smoke requires Windows Notepad")
}
