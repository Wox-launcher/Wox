//go:build wox_ui_smoke

package query

import (
	"context"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxwidget "wox/ui/widget"
)

// Test016LauncherToolbarMessageKeepsDefaultShortcuts verifies a toolbar message keeps default Enter and More on the footer.
// Flow: query the toolbar fixture that publishes a status message plus Enter and a secondary result shortcut.
// Evidence: the status, Hide launcher, and More stay visible together while the query is complete.
func Test016LauncherToolbarMessageKeepsDefaultShortcuts(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.ShowLauncher(t, ctx, client)
		smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, toolbarSmokeQuery)

		snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			status, statusFound := automationdriver.Find(snapshot, "launcher.toolbar.status")
			_, enterFound := automationdriver.FindByAutomationIDPrefix(snapshot, "toolbar-action-result-hide-launcher-")
			_, moreFound := automationdriver.Find(snapshot, "result-toolbar-more")
			return statusFound && status.Value == "Toolbar fixture ready" && enterFound && moreFound
		})
		if err != nil {
			t.Fatalf("wait for toolbar message to keep default Enter and More: %v", err)
		}
		smoke.AssertNoDiagnostics(t, snapshot)
	})
}
