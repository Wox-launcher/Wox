//go:build wox_ui_smoke

package hotkeywarning

import (
	"context"
	"runtime"
	"strings"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// Test001SettingMainHotkeyWarning verifies recovery from an occupied main shortcut.
// Flow: start Wox with its shortcut occupied -> open the warning action -> record an available replacement.
// Evidence: the action opens main-hotkey settings and the warning clears after the replacement registers.
func Test001SettingMainHotkeyWarning(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("the smoke runner reserves a conflicting native shortcut on Windows")
	}
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.ShowLauncher(t, ctx, client)
		snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			status, found := automationdriver.Find(snapshot, "launcher.toolbar.status")
			return found && strings.Contains(strings.ToLower(status.Value), "ctrl+shift+alt+f12")
		})
		if err != nil {
			t.Fatalf("wait for main hotkey warning: %v", err)
		}
		smoke.AssertNoDiagnostics(t, snapshot)
		if err := client.Perform(ctx, "launcher.toolbar.status", woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatal(err)
		}
		snapshot, err = client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			_, found := automationdriver.FindByAutomationIDPrefix(snapshot, "action-toolbar-open-main-hotkey-settings")
			return found
		})
		if err != nil {
			t.Fatalf("wait for hotkey recovery action: %v", err)
		}
		action, _ := automationdriver.FindByAutomationIDPrefix(snapshot, "action-toolbar-open-main-hotkey-settings")
		if err := client.Perform(ctx, action.AutomationID, woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatal(err)
		}
		const fieldID = "hotkey-settings-field-0"
		snapshot, err = client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			field, found := automationdriver.Find(snapshot, fieldID)
			return found && field.Value == "ctrl+shift+alt+f12"
		})
		if err != nil {
			t.Fatalf("warning action did not open main shortcut settings: %v", err)
		}
		field, _ := automationdriver.Find(snapshot, fieldID)
		if err := client.Perform(ctx, fieldID, woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatal(err)
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			current, found := automationdriver.Find(snapshot, fieldID)
			return found && current.Description != field.Description
		}); err != nil {
			t.Fatalf("start main hotkey recording: %v", err)
		}
		if err := smoke.SendNativeKeyChord("ctrl", "alt", "shift", "f11"); err != nil {
			t.Fatal(err)
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			current, found := automationdriver.Find(snapshot, fieldID)
			return found && current.Value == "ctrl+shift+alt+f11"
		}); err != nil {
			t.Fatalf("record replacement main hotkey: %v", err)
		}
		if err := client.PressKey(ctx, woxui.KeyEscape, 0); err != nil {
			t.Fatal(err)
		}
		if err := client.Hide(ctx); err != nil {
			t.Fatal(err)
		}
		smoke.ShowLauncher(t, ctx, client)
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			status, found := automationdriver.Find(snapshot, "launcher.toolbar.status")
			return !found || !strings.Contains(strings.ToLower(status.Value), "ctrl+shift+alt+f12")
		}); err != nil {
			t.Fatalf("warning remained after registering a new main hotkey: %v", err)
		}
	})
}
