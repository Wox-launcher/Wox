//go:build wox_ui_smoke

package query

import (
	"context"
	"strings"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxwidget "wox/ui/widget"
)

// Test035LauncherEmptyQueryHidesToolbar verifies an empty launcher with no results hides the bottom toolbar.
// Flow: select fresh launch and Blank Page -> show the launcher -> query 1+1.
// Evidence: the empty launcher exposes no toolbar, then the completed Calculator result shows the toolbar again.
func Test035LauncherEmptyQueryHidesToolbar(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.ConfigureStartPage(t, ctx, client, 0)
		smoke.ShowLauncher(t, ctx, client)
		snapshot, err := client.WaitForReason(ctx, emptyLauncherOmitsToolbar)
		if err != nil {
			t.Fatalf("wait for empty launcher to hide the toolbar: %v", err)
		}
		smoke.AssertNoDiagnostics(t, snapshot)

		snapshot = smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, "1+1")
		if !smoke.HasLauncherResultLabel(snapshot, "2") {
			t.Fatal("calculator result was not found")
		}
		if _, found := automationdriver.Find(snapshot, "launcher.toolbar"); !found {
			t.Fatal("toolbar did not return with calculator results")
		}
		smoke.AssertNoDiagnostics(t, snapshot)
	})
}

// emptyLauncherOmitsToolbar is the idle launcher: an empty query, no result rows, and no footer.
func emptyLauncherOmitsToolbar(snapshot woxwidget.AutomationSnapshot) (bool, string) {
	input, inputFound := automationdriver.Find(snapshot, "launcher.query.input")
	if !inputFound || input.Value != "" {
		return false, "query input is not empty"
	}
	if launcherHasResultRows(snapshot) {
		return false, "result rows are still visible"
	}
	if _, found := automationdriver.Find(snapshot, "launcher.toolbar"); found {
		status, statusFound := automationdriver.Find(snapshot, "launcher.toolbar.status")
		if statusFound {
			return false, "toolbar stayed visible for status " + status.Value
		}
		return false, "toolbar stayed visible without results or status"
	}
	return true, ""
}

// launcherHasResultRows reports whether the current generation exposes any result row.
func launcherHasResultRows(snapshot woxwidget.AutomationSnapshot) bool {
	for _, node := range snapshot.Tree.Nodes {
		if strings.HasPrefix(node.AutomationID, "launcher.result.") {
			return true
		}
	}
	return false
}
