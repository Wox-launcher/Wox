//go:build wox_ui_smoke

package hotkey

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxwidget "wox/ui/widget"
)

// Test003SettingHotkeyFullscreen verifies that fullscreen suppression follows the setting and foreground window.
// Flow: enable suppression -> press the main hotkey in fullscreen -> leave fullscreen -> disable suppression -> retry in fullscreen.
// Evidence: a fresh suppression log and hidden launcher, followed by visible launcher windows in both recovery paths.
func Test003SettingHotkeyFullscreen(t *testing.T) {
	requireFullscreenHotkeyRuntime(t)
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		const key = "IgnoreHotkeysOnFullscreen"
		const hotkey = "ctrl+f12"
		ensureMainHotkey(t, ctx, client, hotkey)
		previous := smoke.OpenGeneralSettingsAndReadSwitch(t, ctx, client, key)
		t.Cleanup(func() { smoke.RestoreGeneralSettingSwitch(t, client, key, previous) })
		smoke.SetSettingSwitch(t, ctx, client, key, true)
		if !smoke.OpenGeneralSettingsAndReadSwitch(t, ctx, client, key) {
			t.Fatal("fullscreen suppression did not persist")
		}
		target := newFullscreenHotkeyTarget(t)
		logPath := filepath.Join(os.Getenv(automationdriver.SharedDataDirectoryEnvironment), "log", "wox.log")
		for _, step := range []struct {
			name                   string
			fullscreen, suppressed bool
		}{
			{"enabled in fullscreen", true, true},
			{"enabled outside fullscreen", false, false},
			{"disabled in fullscreen", true, false},
		} {
			if step.name == "disabled in fullscreen" {
				smoke.OpenGeneralSettingsAndReadSwitch(t, ctx, client, key)
				smoke.SetSettingSwitch(t, ctx, client, key, false)
				if smoke.OpenGeneralSettingsAndReadSwitch(t, ctx, client, key) {
					t.Fatal("disabling fullscreen suppression did not persist")
				}
			}
			if err := client.Hide(ctx); err != nil {
				t.Fatal(err)
			}
			target(step.fullscreen)
			if _, err := client.WaitFor(ctx, func(_ woxwidget.AutomationSnapshot) bool {
				return fullscreenHotkeyTargetFocused()
			}); err != nil {
				t.Fatalf("%s: target did not gain foreground focus: %v", step.name, err)
			}
			offset := currentHotkeyLogSize(t, logPath)
			sendNativeHotkey(t, hotkey)
			if step.suppressed {
				logs := waitForHotkeyLog(t, ctx, logPath, offset, "ignore hotkey trigger for fullscreen foreground window")
				state, err := client.WindowState(ctx, "primary")
				if err != nil || state.Visible {
					t.Fatalf("%s: launcher should remain hidden: state=%+v err=%v logs=%s", step.name, state, err, logs)
				}
			} else {
				if _, err := client.WaitForWindowState(ctx, "primary", func(state automationdriver.WindowState) bool {
					return state.Visible && state.Lifecycle == "visible"
				}); err != nil {
					t.Fatalf("%s: hotkey did not show launcher: %v", step.name, err)
				}
			}
		}
	})
}
