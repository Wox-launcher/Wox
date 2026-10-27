package setting

import (
	"testing"

	"wox/util"
)

func TestDefaultLinuxMainHotkeyUsesAltSpaceOnOmarchy(t *testing.T) {
	got := defaultLinuxMainHotkey()
	if util.IsOmarchy() {
		if got != "alt+space" {
			t.Fatalf("Omarchy main hotkey = %q", got)
		}
		return
	}
	if got != "ctrl+space" {
		t.Fatalf("Linux main hotkey = %q", got)
	}
}
