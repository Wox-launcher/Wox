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

// Test040LauncherGlobalToolbarContext requires an action before showing a global result's toolbar status.
// Flow: select a global-only plugin result -> execute its action -> select its other result -> select another plugin -> return.
// Evidence: selection alone stays quiet, while action status survives same-plugin selection and clears on exit.
func Test040LauncherGlobalToolbarContext(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.ShowLauncher(t, ctx, client)
		snapshot := smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, "wox-global-toolbar-smoke")
		globalID, found := smoke.FindLauncherResult(snapshot, "Global toolbar fixture")
		if !found {
			t.Fatal("global-only plugin result not found")
		}
		secondID, found := smoke.FindLauncherResult(snapshot, "Global toolbar second fixture")
		if !found {
			t.Fatal("second global-only plugin result not found")
		}
		otherID, found := smoke.FindLauncherResult(snapshot, "Other global fixture")
		if !found {
			t.Fatal("other plugin result not found")
		}
		smoke.SelectLauncherResult(t, ctx, client, globalID)
		waitGlobalToolbar(t, ctx, client, "Global selection toolbar", false)
		if err := client.Perform(ctx, globalID, woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatal(err)
		}
		waitGlobalToolbar(t, ctx, client, "Global fixture operation completed", true)
		smoke.SelectLauncherResult(t, ctx, client, secondID)
		waitGlobalToolbar(t, ctx, client, "Global fixture operation completed", true)
		smoke.SelectLauncherResult(t, ctx, client, otherID)
		waitGlobalToolbar(t, ctx, client, "Global fixture operation completed", false)
		smoke.SelectLauncherResult(t, ctx, client, globalID)
		waitGlobalToolbar(t, ctx, client, "Global selection toolbar", false)
	})
}

func waitGlobalToolbar(t *testing.T, ctx context.Context, client *automationdriver.Client, message string, present bool) {
	t.Helper()
	snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		status, found := automationdriver.Find(snapshot, "launcher.toolbar.status")
		return (found && status.Value == message) == present
	})
	if err != nil {
		t.Fatalf("wait for global toolbar present=%v: %v", present, err)
	}
	smoke.AssertNoDiagnostics(t, snapshot)
}
