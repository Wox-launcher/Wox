//go:build wox_ui_smoke

package query

import (
	"context"
	"testing"

	"wox/i18n"
	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// Test041LauncherUpdateNotice verifies the update confirmation appears on the next primary show.
// Flow: arm the confirmation -> hide and show the primary launcher -> publish a plugin status -> let the notice expire -> leave the plugin.
// Evidence: the success message appears first; after expiry the warning or plugin status appears according to priority.
func Test041LauncherUpdateNotice(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.ShowLauncher(t, ctx, client)
		snapshot := smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, "wox-smoke startup-update ")
		id, found := smoke.FindLauncherResult(snapshot, "Arm update confirmation fixture")
		if !found {
			t.Fatal("update confirmation fixture not found")
		}
		previous := ""
		if status, found := automationdriver.Find(snapshot, "launcher.toolbar.status"); found {
			previous = status.Value
		}
		if err := client.Perform(ctx, id, woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatal(err)
		}
		if err := client.Hide(ctx); err != nil {
			t.Fatal(err)
		}
		if _, err := client.WaitForWindowState(ctx, "primary", func(state automationdriver.WindowState) bool { return !state.Visible }); err != nil {
			t.Fatal(err)
		}
		smoke.ShowLauncher(t, ctx, client)
		values := map[string]bool{}
		for _, lang := range i18n.GetSupportedLanguages() {
			texts, err := i18n.GetI18nManager().GetLangMap(ctx, lang.Code)
			if err != nil {
				t.Fatal(err)
			}
			values[texts["ui_update_success"]] = true
		}
		observed, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			status, found := automationdriver.Find(snapshot, "launcher.toolbar.status")
			return found && status.Value != "" && values[status.Value]
		})
		if err != nil {
			t.Fatalf("wait for update confirmation: %v", err)
		}
		smoke.AssertNoDiagnostics(t, observed)
		smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, "wox-smoke toolbar ")
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			status, found := automationdriver.Find(snapshot, "launcher.toolbar.status")
			if previous != "" {
				return found && status.Value == previous
			}
			return found && status.Value == "Toolbar fixture ready"
		}); err != nil {
			t.Fatalf("system or plugin status not restored after notice: %v", err)
		}
		smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, "1+1")
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			status, found := automationdriver.Find(snapshot, "launcher.toolbar.status")
			if previous == "" {
				return !found
			}
			return found && status.Value == previous
		}); err != nil {
			t.Fatalf("warning not restored after leaving plugin: %v", err)
		}
	})
}
