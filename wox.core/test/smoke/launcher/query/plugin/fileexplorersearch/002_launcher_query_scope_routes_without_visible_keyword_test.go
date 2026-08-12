//go:build wox_ui_smoke

package fileexplorersearch

import (
	"context"
	"strings"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxwidget "wox/ui/widget"
)

// Test002LauncherQueryScopeRoutesWithoutVisibleKeyword verifies that an unprefixed
// Explorer secondary query remains isolated from globally triggered plugins.
// Flow: open the scoped Explorer directly with a calculator expression.
// Evidence: the visible input has no Explorer keyword, the scope accessory is present,
// and the Calculator result does not leak into the Explorer-only result set.
func Test002LauncherQueryScopeRoutesWithoutVisibleKeyword(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		const expression = "1+1"
		if err := client.OpenExplorerQuery(ctx, expression); err != nil {
			t.Fatalf("open scoped explorer query: %v", err)
		}
		if _, err := client.WaitForWindowState(ctx, explorerInstance, func(state automationdriver.WindowState) bool {
			return state.Exists && state.Visible
		}); err != nil {
			t.Fatalf("wait for explorer window: %v", err)
		}
		if err := client.FocusInstance(ctx, explorerInstance); err != nil {
			t.Fatalf("focus explorer instance: %v", err)
		}

		snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			input, inputFound := automationdriver.Find(snapshot, "launcher.query.input")
			scopeIcons, scopeFound := automationdriver.Find(snapshot, "launcher.query.scope-icons")
			results, resultsFound := automationdriver.Find(snapshot, "launcher.results")
			_, loadingFound := automationdriver.Find(snapshot, "launcher.query.loading")
			return inputFound && input.Value == expression && !strings.HasPrefix(input.Value, "explorer ") &&
				scopeFound && scopeIcons.Value == "1" && !loadingFound && (!resultsFound || results.Value == "complete")
		})
		if err != nil {
			t.Fatalf("wait for Explorer-only scoped query: %v", err)
		}
		if len(snapshot.Diagnostics) > 0 {
			t.Fatalf("explorer semantics diagnostics: %v", snapshot.Diagnostics)
		}
		if hasLauncherResultTitle(snapshot, "2") {
			t.Fatal("calculator result leaked into the Explorer-scoped query")
		}
	})
}

func hasLauncherResultTitle(snapshot woxwidget.AutomationSnapshot, title string) bool {
	for _, node := range snapshot.Tree.Nodes {
		if strings.HasPrefix(node.AutomationID, "launcher.result.") && strings.TrimSpace(node.Label) == title {
			return true
		}
	}
	return false
}
