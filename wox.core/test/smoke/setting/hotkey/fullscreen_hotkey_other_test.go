//go:build wox_ui_smoke && !windows

package hotkey

import "testing"

func requireFullscreenHotkeyRuntime(t *testing.T) {
	t.Helper()
	t.Skip("fullscreen smoke requires the Windows native foreground-window fixture")
}

func fullscreenHotkeyTargetFocused() bool               { return false }
func newFullscreenHotkeyTarget(t *testing.T) func(bool) { return func(bool) {} }
