//go:build wox_ui_smoke && !windows

package hotkey

import "testing"

func sendNativeHotkey(t *testing.T, _ string) {
	t.Helper()
	t.Skip("main hotkey global registration smoke coverage requires Windows keyboard injection")
}
