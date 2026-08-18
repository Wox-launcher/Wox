//go:build wox_ui_smoke && darwin

package hotkey

import (
	"context"
	"testing"

	"wox/test/smoke"
)

func requireIgnoredHotkeyAppRuntime(t *testing.T) {
	t.Helper()
}

func ignoredHotkeyAppTarget(t *testing.T) (string, string) {
	t.Helper()
	return "com.apple.TextEdit", "com.apple.TextEdit"
}

func ignoredHotkeyAppHotkey(t *testing.T) string {
	t.Helper()
	return "cmd+space"
}

func sendIgnoredAppNativeHotkey(t *testing.T, hotkey string) {
	t.Helper()
	if hotkey != "cmd+space" {
		t.Fatalf("unsupported native macOS ignored-app hotkey %q", hotkey)
	}
	if err := smoke.SendNativeKeyChord("command", "space"); err != nil {
		t.Fatalf("post native macOS ignored-app hotkey %q: %v", hotkey, err)
	}
}

// activateIgnoredHotkeyApp launches and focuses one new TextEdit instance without touching existing instances.
func activateIgnoredHotkeyApp(t *testing.T, ctx context.Context) {
	t.Helper()
	smoke.OpenDarwinTextEdit(t, ctx, "")
}
