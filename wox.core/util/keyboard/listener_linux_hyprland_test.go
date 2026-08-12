//go:build linux && cgo

package keyboard

import (
	"strings"
	"testing"
)

func TestHyprlandBindScriptReplacesPreviousGroup(t *testing.T) {
	bindings := []hyprlandBinding{
		{luaKey: "ALT + SPACE", deeplink: "wox://toggle"},
		{luaKey: "CTRL + SHIFT + S", deeplink: "wox://hyprland-hotkey?key=CTRL+%2B+SHIFT+%2B+S"},
	}

	script := hyprlandBindScript(bindings, "/opt/Wox Launcher/wox")

	cleanup := "for _, key in ipairs(wox_bound_keys) do hl.unbind(key) end"
	if !strings.Contains(script, cleanup) {
		t.Fatalf("bind script does not remove the previous group:\n%s", script)
	}
	if strings.Contains(script, "wox_binds_loaded") {
		t.Fatalf("bind script still contains the process-lifetime registration guard:\n%s", script)
	}
	for _, luaKey := range []string{"ALT + SPACE", "CTRL + SHIFT + S"} {
		if !strings.Contains(script, "hl.bind(\""+luaKey+"\"") {
			t.Fatalf("bind script does not register %q:\n%s", luaKey, script)
		}
		if !strings.Contains(script, "table.insert(wox_bound_keys, \""+luaKey+"\")") {
			t.Fatalf("bind script does not track %q:\n%s", luaKey, script)
		}
	}
}

func TestHyprlandUnbindScriptUsesRegistrationKeysAsFallback(t *testing.T) {
	script := hyprlandUnbindScript([]string{"SUPER + SPACE", "CTRL + SHIFT + S"})

	if !strings.Contains(script, "local keys = wox_bound_keys or {\"SUPER + SPACE\", \"CTRL + SHIFT + S\"}") {
		t.Fatalf("unbind script does not preserve the registration keys:\n%s", script)
	}
	if !strings.Contains(script, "for _, key in ipairs(keys) do hl.unbind(key) end") {
		t.Fatalf("unbind script does not call hl.unbind:\n%s", script)
	}
	if !strings.Contains(script, "wox_bound_keys = nil") {
		t.Fatalf("unbind script does not clear compositor state:\n%s", script)
	}
}

func TestUnregisterHyprlandHotkeyCallbacks(t *testing.T) {
	const removedKey = "ALT + SPACE"
	const retainedKey = "CTRL + SPACE"
	RegisterHyprlandHotkeyCallback(removedKey, func() {})
	RegisterHyprlandHotkeyCallback(retainedKey, func() {})
	t.Cleanup(func() {
		unregisterHyprlandHotkeyCallbacks([]string{removedKey, retainedKey})
	})

	unregisterHyprlandHotkeyCallbacks([]string{removedKey})

	hyprlandCallbacksMu.Lock()
	defer hyprlandCallbacksMu.Unlock()
	if _, ok := hyprlandCallbacks[removedKey]; ok {
		t.Fatalf("callback for %q was not removed", removedKey)
	}
	if _, ok := hyprlandCallbacks[retainedKey]; !ok {
		t.Fatalf("callback for %q was removed unexpectedly", retainedKey)
	}
}
