//go:build linux

package window

import (
	"errors"
	"strings"
	"testing"

	"wox/util"
)

func TestHyprlandLayerNoAnimScriptMatchesDesktopID(t *testing.T) {
	script := hyprlandLayerNoAnimScript(util.LinuxDesktopAppID)
	if !strings.Contains(script, `namespace = [[^io\.github\.WoxLauncher\.Wox$]]`) {
		t.Fatalf("script missing anchored namespace:\n%s", script)
	}
	if !strings.Contains(script, `name = "wox-no-layer-anim"`) || !strings.Contains(script, "no_anim = true") {
		t.Fatalf("script missing named no_anim rule:\n%s", script)
	}
	if !strings.Contains(script, `if wox_layer_no_anim ~= nil then return "exists" end`) {
		t.Fatalf("script missing compositor-side guard:\n%s", script)
	}
}

func TestHyprlandLayerNoAnimInstallsOnceAndRetriesAfterFailure(t *testing.T) {
	hyprlandLayerRule.mu.Lock()
	hyprlandLayerRule.ready = false
	hyprlandLayerRule.mu.Unlock()
	t.Cleanup(func() {
		hyprlandLayerRule.mu.Lock()
		hyprlandLayerRule.ready = false
		hyprlandLayerRule.mu.Unlock()
	})

	var calls int
	if err := ensureHyprlandLayerNoAnimWith(func(string) error {
		calls++
		return errors.New("hyprctl unavailable")
	}); err == nil {
		t.Fatal("install error was dropped")
	}
	if err := ensureHyprlandLayerNoAnimWith(func(script string) error {
		calls++
		if !strings.Contains(script, "no_anim = true") {
			t.Fatalf("script = %s", script)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := ensureHyprlandLayerNoAnimWith(func(string) error {
		calls++
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("hyprctl calls = %d, want 2", calls)
	}
}
