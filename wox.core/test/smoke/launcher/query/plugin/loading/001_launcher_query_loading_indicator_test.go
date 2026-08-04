//go:build wox_ui_smoke

package loading

import (
	"context"
	"strings"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// Test001LauncherQueryLoadingIndicator verifies delayed plugin queries expose progress without leaving stale loading UI.
// Flow: enter the slow plugin query -> observe querybox loading -> receive the plugin result.
// Evidence: the progress-bar semantics appears while waiting, then disappears when the completed result generation arrives.
func Test001LauncherQueryLoadingIndicator(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.ShowLauncher(t, ctx, client)
		const query = "wox-smoke-slow wait"
		if err := client.Perform(ctx, "launcher.query.input", woxui.AccessibilityActionSetValue, query); err != nil {
			t.Fatalf("enter slow plugin query: %v", err)
		}

		loadingSnapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			input, inputFound := automationdriver.Find(snapshot, "launcher.query.input")
			loading, loadingFound := automationdriver.Find(snapshot, "launcher.query.loading")
			return inputFound && input.Value == query && loadingFound && loading.Role == woxui.AccessibilityRoleProgressBar && loading.Value == "loading"
		})
		if err != nil {
			t.Fatalf("wait for delayed query loading: %v", err)
		}
		smoke.AssertNoDiagnostics(t, loadingSnapshot)

		completedSnapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			_, loadingFound := automationdriver.Find(snapshot, "launcher.query.loading")
			results, resultsFound := automationdriver.Find(snapshot, "launcher.results")
			return !loadingFound && resultsFound && results.Value == "complete" && hasCompletedSlowQueryResult(snapshot)
		})
		if err != nil {
			t.Fatalf("wait for slow plugin query completion: %v", err)
		}
		smoke.AssertNoDiagnostics(t, completedSnapshot)
	})
}

// hasCompletedSlowQueryResult identifies the fixture's result without retaining a generation-bound ID.
func hasCompletedSlowQueryResult(snapshot woxwidget.AutomationSnapshot) bool {
	for _, node := range snapshot.Tree.Nodes {
		if strings.HasPrefix(node.AutomationID, "launcher.result.") && node.Label == "Slow query completed" {
			return true
		}
	}
	return false
}
