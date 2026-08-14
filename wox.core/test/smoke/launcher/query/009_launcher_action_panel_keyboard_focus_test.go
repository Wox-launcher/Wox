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

// Test009LauncherActionPanelKeyboardFocus verifies keyboard selection and focus return across panel close.
// Flow: query a Shell command -> open the action panel -> move selection -> press Escape -> reopen the panel.
// Evidence: selection changes, Escape restores query focus, and reopening starts with an empty filter.
func Test009LauncherActionPanelKeyboardFocus(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.ShowLauncher(t, ctx, client)
		smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, "> "+actionPanelShellCommand)
		snapshot := smoke.OpenResultActionPanel(t, ctx, client)
		first, found := selectedActionPanelResult(snapshot)
		if !found {
			t.Fatal("no selected action after opening Action Panel")
		}

		if err := client.PressKey(ctx, woxui.KeyArrowDown, 0); err != nil {
			t.Fatalf("move Action Panel selection down: %v", err)
		}
		snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			selected, selectedFound := selectedActionPanelResult(snapshot)
			return selectedFound && selected.AutomationID != first.AutomationID
		})
		if err != nil {
			t.Fatalf("wait for changed Action Panel selection: %v", err)
		}

		if err := client.PressKey(ctx, woxui.KeyEscape, 0); err != nil {
			t.Fatalf("close Action Panel with Escape: %v", err)
		}
		snapshot, err = client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			query, queryFound := automationdriver.Find(snapshot, "launcher.query.input")
			_, searchFound := automationdriver.Find(snapshot, "action-search")
			return queryFound && query.Focused && !searchFound && len(actionPanelResultNodes(snapshot)) == 0
		})
		if err != nil {
			t.Fatalf("wait for query focus after Action Panel close: %v", err)
		}

		snapshot = smoke.OpenResultActionPanel(t, ctx, client)
		search, searchFound := automationdriver.Find(snapshot, "action-search")
		if !searchFound || search.Value != "" {
			t.Fatalf("reopened Action Panel search = found %v value %q, want empty", searchFound, search.Value)
		}
		smoke.AssertNoDiagnostics(t, snapshot)
	})
}
