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

	cleanup := "for _, bind in ipairs(wox_bind_handles) do bind:set_enabled(false) end"
	if !strings.Contains(script, cleanup) {
		t.Fatalf("bind script does not disable the previous owned group:\n%s", script)
	}
	if strings.Contains(script, "hl.unbind") {
		t.Fatalf("bind script globally unbinds user-owned keys:\n%s", script)
	}
	for _, luaKey := range []string{"ALT + SPACE", "CTRL + SHIFT + S"} {
		if !strings.Contains(script, "table.insert(wox_bind_handles, hl.bind(\""+luaKey+"\"") {
			t.Fatalf("bind script does not register %q:\n%s", luaKey, script)
		}
	}
	if !strings.Contains(script, `description = "Wox global hotkey"`) {
		t.Fatalf("bind script does not identify Wox-owned handles:\n%s", script)
	}
}

func TestHyprlandDisableScriptOnlyDisablesOwnedHandles(t *testing.T) {
	script := hyprlandDisableScript()

	if !strings.Contains(script, "for _, bind in ipairs(wox_bind_handles) do bind:set_enabled(false) end") {
		t.Fatalf("disable script does not disable owned handles:\n%s", script)
	}
	if strings.Contains(script, "hl.unbind") {
		t.Fatalf("disable script globally unbinds user-owned keys:\n%s", script)
	}
	if !strings.Contains(script, "wox_bind_handles = nil") {
		t.Fatalf("unbind script does not clear compositor state:\n%s", script)
	}
}

func TestHyprlandBindingConflictsIgnoreWoxAndSubmaps(t *testing.T) {
	binds := []hyprlandConfiguredBind{
		{ModMask: 64, Key: "left"},
		{ModMask: 8, Key: "SPACE", Description: hyprlandBindDescription},
		{ModMask: 4, Key: "K", Submap: "resize"},
	}

	if !hyprlandBindingConflicts(binds, 64, "LEFT") {
		t.Fatal("user-owned SUPER + LEFT should conflict")
	}
	if hyprlandBindingConflicts(binds, 8, "space") {
		t.Fatal("Wox-owned ALT + SPACE should not conflict")
	}
	if hyprlandBindingConflicts(binds, 4, "K") {
		t.Fatal("a non-default-submap binding should not conflict")
	}
}

func TestHyprlandKeyToModMask(t *testing.T) {
	modifiers := ModifierCtrl | ModifierAlt | ModifierShift | ModifierSuper
	if got := hyprlandKeyToModMask(modifiers); got != 77 {
		t.Fatalf("Hyprland modifier mask = %d, want 77", got)
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
