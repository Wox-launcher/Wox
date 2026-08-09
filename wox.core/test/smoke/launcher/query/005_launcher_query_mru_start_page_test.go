//go:build wox_ui_smoke

package query

import (
	"context"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxwidget "wox/ui/widget"
)

// Test005LauncherQueryMRUStartPage verifies that Most Recently Used restores eligible items for an empty query.
// Flow: create a durable MRU item -> select fresh launch and Most Recently Used -> show the launcher with an empty query.
// Evidence: the real launcher keeps an empty input while exposing the restored Converter result in a completed generation.
func Test005LauncherQueryMRUStartPage(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		configureStartPage(t, ctx, client, 1)
		mruLabel := seedConverterMRU(t, ctx, client)

		smoke.ShowLauncher(t, ctx, client)
		snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			input, inputFound := automationdriver.Find(snapshot, "launcher.query.input")
			results, resultsFound := automationdriver.Find(snapshot, "launcher.results")
			return inputFound && input.Value == "" && resultsFound && results.Value == "complete" && hasLauncherResultLabel(snapshot, mruLabel)
		})
		if err != nil {
			t.Fatalf("wait for MRU Start Page result %q: %v", mruLabel, err)
		}
		smoke.AssertNoDiagnostics(t, snapshot)
	})
}
