//go:build wox_ui_smoke

package query

import (
	"context"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// Test041LauncherUpdateNotice verifies the update confirmation appears on the next primary show.
// Flow: arm the confirmation -> hide and show the primary launcher -> update the plugin status while covered -> let the notice expire -> execute the restored action.
// Evidence: the notice retains priority, then the latest plugin status and its callback work unless a higher-priority warning remains.
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
		updateTitle := smoke.LocalizedText(t, ctx, client, "ui_update_success")
		observed, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			status, found := automationdriver.Find(snapshot, "launcher.toolbar.status")
			return found && status.Value == updateTitle
		})
		if err != nil {
			t.Fatalf("wait for update confirmation: %v", err)
		}
		smoke.AssertNoDiagnostics(t, observed)
		for _, query := range []string{toolbarLongSmokeQuery, toolbarSmokeQuery} {
			snapshot := smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, query)
			status, found := automationdriver.Find(snapshot, "launcher.toolbar.status")
			if !found || status.Value != updateTitle {
				t.Fatalf("system notice stopped covering plugin update for %q: found=%t status=%q", query, found, status.Value)
			}
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			status, found := automationdriver.Find(snapshot, "launcher.toolbar.status")
			if previous != "" {
				return found && status.Value == previous
			}
			return found && status.Value == "Toolbar fixture ready"
		}); err != nil {
			t.Fatalf("system or plugin status not restored after notice: %v", err)
		}
		if previous == "" {
			activateToolbarFixtureAction(t, ctx, client, "toolbar-action-toolbar-keep-open-", "action-toolbar-keep-open-")
			updated, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
				status, found := automationdriver.Find(snapshot, "launcher.toolbar.status")
				return found && status.Value == "Toolbar fixture keep-open: context-round-trip"
			})
			if err != nil {
				t.Fatalf("restored plugin callback did not update its status: %v", err)
			}
			smoke.AssertNoDiagnostics(t, updated)
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
