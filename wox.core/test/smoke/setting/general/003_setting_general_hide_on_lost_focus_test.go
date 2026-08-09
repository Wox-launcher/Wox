//go:build wox_ui_smoke

package general

import (
	"context"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxwidget "wox/ui/widget"
)

// Test003SettingGeneralHideOnLostFocus verifies that the main and selection-query windows honor the General focus-loss setting.
// Flow: enable Hide on focus loss -> focus a selection query away from the main launcher -> focus the main launcher again.
// Evidence: the real managed lifecycle hides the main window first and then closes the blurred selection window.
func Test003SettingGeneralHideOnLostFocus(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.ShowLauncher(t, ctx, client)
		previousValue := smoke.OpenGeneralSettingsAndReadSwitch(t, ctx, client, "HideOnLostFocus")
		smoke.SetSettingSwitch(t, ctx, client, "HideOnLostFocus", true)
		t.Cleanup(func() {
			if !previousValue {
				smoke.RestoreGeneralSettingSwitch(t, client, "HideOnLostFocus", previousValue)
			}
		})
		if err := client.Hide(ctx); err != nil {
			t.Fatalf("close General settings after enabling focus-loss hiding: %v", err)
		}

		smoke.ShowLauncher(t, ctx, client)
		if _, err := client.WaitForWindowState(ctx, "primary", func(state automationdriver.WindowState) bool {
			return state.Exists && state.Visible && state.BlurReady && state.Lifecycle == "visible"
		}); err != nil {
			t.Fatalf("wait for visible primary launcher: %v", err)
		}
		if err := client.OpenSelectionQuery(ctx, "smoke focus selection"); err != nil {
			t.Fatalf("open selection query: %v", err)
		}
		if _, err := client.WaitForWindowState(ctx, "selection", func(state automationdriver.WindowState) bool {
			return state.Exists && state.Visible && state.BlurReady && state.Lifecycle == "visible"
		}); err != nil {
			t.Fatalf("wait for visible selection query: %v", err)
		}
		if _, err := client.WaitForWindowState(ctx, "primary", func(state automationdriver.WindowState) bool {
			return state.Exists && !state.Visible && state.Lifecycle == "hidden"
		}); err != nil {
			t.Fatalf("wait for primary launcher to hide on focus loss: %v", err)
		}

		smoke.ShowLauncher(t, ctx, client)
		if _, err := client.WaitForWindowState(ctx, "selection", func(state automationdriver.WindowState) bool {
			return !state.Exists
		}); err != nil {
			t.Fatalf("wait for selection query to close on focus loss: %v", err)
		}
		snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			_, found := automationdriver.Find(snapshot, "launcher.query.input")
			return found
		})
		if err != nil {
			t.Fatalf("wait for restored primary launcher: %v", err)
		}
		smoke.AssertNoDiagnostics(t, snapshot)
	})
}
