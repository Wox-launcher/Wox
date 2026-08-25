//go:build wox_ui_smoke

package query

import (
	"context"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxwidget "wox/ui/widget"
)

const (
	toolbarLongSmokeQuery = "wox-smoke toolbar-long "
	// toolbarLongSmokeStatus must match the smoke fixture long toolbar title.
	toolbarLongSmokeStatus = "Indexing file contents across every configured search root: 18365 files are already processed and the catalog is still updating with additional paths that must remain visible in the launcher status"
)

// Test017LauncherToolbarMessageFillsLeftoverShortcuts verifies leftover footer width after a toolbar message fills extra hotkey actions and drops them when the status grows.
// Flow: query the short toolbar fixture -> replace the query with the long status fixture.
// Evidence: Open folder appears beside Enter and More for the short status, then disappears once the long status is visible while Enter and More remain.
func Test017LauncherToolbarMessageFillsLeftoverShortcuts(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.ShowLauncher(t, ctx, client)
		smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, toolbarSmokeQuery)
		waitForToolbarShortcutPack(t, ctx, client, "Toolbar fixture ready", true)

		smoke.ReplaceLauncherQuery(t, ctx, client, toolbarLongSmokeQuery)
		waitForToolbarShortcutPack(t, ctx, client, toolbarLongSmokeStatus, false)
	})
}

func waitForToolbarShortcutPack(t *testing.T, ctx context.Context, client *automationdriver.Client, statusValue string, extraVisible bool) {
	t.Helper()
	snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		status, statusFound := automationdriver.Find(snapshot, "launcher.toolbar.status")
		_, enterFound := automationdriver.FindByAutomationIDPrefix(snapshot, "toolbar-action-result-hide-launcher-")
		_, moreFound := automationdriver.Find(snapshot, "result-toolbar-more")
		_, extraFound := automationdriver.FindByAutomationIDPrefix(snapshot, "toolbar-action-result-open-folder-")
		return statusFound && status.Value == statusValue && enterFound && moreFound && extraFound == extraVisible
	})
	if err != nil {
		t.Fatalf("wait for toolbar shortcut pack status %q extra=%t: %v", statusValue, extraVisible, err)
	}
	smoke.AssertNoDiagnostics(t, snapshot)
}
