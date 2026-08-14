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

// Test008LauncherActionPanelFilter verifies filtering hides unmatched actions and recovers when cleared.
// Flow: query a Shell command -> open the action panel -> enter an unmatched filter -> clear the filter.
// Evidence: no action rows appear for the unmatched value, then the original rows and a selected action return.
func Test008LauncherActionPanelFilter(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.ShowLauncher(t, ctx, client)
		smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, "> "+actionPanelShellCommand)
		snapshot := smoke.OpenResultActionPanel(t, ctx, client)
		initialCount := len(actionPanelResultNodes(snapshot))
		if initialCount < 2 {
			t.Fatalf("initial action row count = %d, want at least 2", initialCount)
		}

		const noMatch = "__wox_action_panel_no_match__"
		if err := client.Perform(ctx, "action-search", woxui.AccessibilityActionSetValue, noMatch); err != nil {
			t.Fatalf("set unmatched action filter: %v", err)
		}
		snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			input, inputFound := automationdriver.Find(snapshot, "action-search")
			return inputFound && input.Value == noMatch && len(actionPanelResultNodes(snapshot)) == 0
		})
		if err != nil {
			t.Fatalf("wait for unmatched action filter: %v", err)
		}

		if err := client.Perform(ctx, "action-search", woxui.AccessibilityActionSetValue, ""); err != nil {
			t.Fatalf("clear action filter: %v", err)
		}
		snapshot, err = client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			input, inputFound := automationdriver.Find(snapshot, "action-search")
			_, selectedFound := selectedActionPanelResult(snapshot)
			return inputFound && input.Value == "" && len(actionPanelResultNodes(snapshot)) == initialCount && selectedFound
		})
		if err != nil {
			t.Fatalf("wait for cleared action filter: %v", err)
		}
		smoke.AssertNoDiagnostics(t, snapshot)
	})
}
