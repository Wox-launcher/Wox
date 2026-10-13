//go:build wox_ui_smoke && !windows

package hotkey

import "testing"

func requireFullscreenHotkeyRuntime(t *testing.T) {
	t.Helper()
	t.Skip("fullscreen smoke requires the Windows native foreground-window fixture")
}

func newFullscreenHotkeyTarget(t *testing.T) func(bool) error {
	return func(bool) error { return nil }
}
