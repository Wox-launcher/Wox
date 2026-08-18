//go:build wox_ui_smoke && windows

package hotkey

import (
	"strings"
	"testing"

	"wox/test/smoke"
)

// sendNativeHotkey uses Windows keyboard injection because main-hotkey registration is a global OS boundary.
func sendNativeHotkey(t *testing.T, hotkey string) {
	t.Helper()
	if err := smoke.SendNativeKeyChord(strings.Split(hotkey, "+")...); err != nil {
		t.Fatalf("send native smoke hotkey %q: %v", hotkey, err)
	}
}
