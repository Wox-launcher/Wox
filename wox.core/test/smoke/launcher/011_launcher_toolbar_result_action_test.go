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

// Test011LauncherToolbarResultAction verifies result actions exposed by the launcher bottom toolbar.
// Flow: query the toolbar fixture -> open More actions -> close the panel -> activate the result action.
// Evidence: the action is exposed as a button and the real launcher window hides after execution.
func Test011LauncherToolbarResultAction(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.ShowLauncher(t, ctx, client)
		const query = "wox-smoke toolbar "
		snapshot := smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, query)
		if _, found := smoke.FindLauncherResult(snapshot, "Toolbar smoke fixture"); !found {
			t.Fatal("toolbar fixture result was not found")
		}

		snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			_, moreFound := automationdriver.Find(snapshot, "result-toolbar-more")
			_, resultActionFound := automationdriver.FindByAutomationIDPrefix(snapshot, "toolbar-action-result-hide-launcher-")
			return moreFound && resultActionFound
		})
		if err != nil {
			t.Fatalf("wait for bottom toolbar actions: %v", err)
		}
		smoke.AssertNoDiagnostics(t, snapshot)

		if err := client.Perform(ctx, "result-toolbar-more", woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("open bottom toolbar More actions: %v", err)
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			_, found := automationdriver.Find(snapshot, "action-search")
			return found
		}); err != nil {
			t.Fatalf("wait for bottom toolbar action panel: %v", err)
		}
		if err := client.PressKey(ctx, woxui.KeyEscape, 0); err != nil {
			t.Fatalf("close bottom toolbar action panel: %v", err)
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			_, found := automationdriver.Find(snapshot, "action-search")
			return !found
		}); err != nil {
			t.Fatalf("wait for bottom toolbar action panel to close: %v", err)
		}

		snapshot, err = client.Snapshot(ctx)
		if err != nil {
			t.Fatalf("read bottom toolbar actions: %v", err)
		}
		resultAction, found := automationdriver.FindByAutomationIDPrefix(snapshot, "toolbar-action-result-hide-launcher-")
		if !found {
			t.Fatal("bottom toolbar result action was not found after closing More actions")
		}
		if err := client.Perform(ctx, resultAction.AutomationID, woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("activate bottom toolbar result action: %v", err)
		}
		if _, err := client.WaitForWindowState(ctx, "primary", func(state automationdriver.WindowState) bool {
			return state.Exists && !state.Visible && state.Lifecycle == "hidden"
		}); err != nil {
			t.Fatalf("wait for launcher to hide after bottom toolbar result action: %v", err)
		}
	})
}
