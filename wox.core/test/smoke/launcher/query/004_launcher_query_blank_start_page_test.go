//go:build wox_ui_smoke

package query

import (
	"context"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxwidget "wox/ui/widget"
)

// Test004LauncherQueryBlankStartPage verifies that Blank Page suppresses eligible recent items for an empty query.
// Flow: create a durable MRU item -> select fresh launch and Blank Page -> show the launcher with an empty query.
// Evidence: the real launcher input is empty and exposes no result rows despite the persisted MRU seed.
func Test004LauncherQueryBlankStartPage(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		configureStartPage(t, ctx, client, 0)
		seedConverterMRU(t, ctx, client)

		smoke.ShowLauncher(t, ctx, client)
		snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			input, found := automationdriver.Find(snapshot, "launcher.query.input")
			return found && input.Value == "" && !hasLauncherResults(snapshot)
		})
		if err != nil {
			t.Fatalf("wait for blank Start Page: %v", err)
		}
		smoke.AssertNoDiagnostics(t, snapshot)
	})
}
