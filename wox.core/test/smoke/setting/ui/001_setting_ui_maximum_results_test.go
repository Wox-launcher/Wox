//go:build wox_ui_smoke

package ui

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const maximumResultsShellCommandPrefix = "woxsmokemaxresult"

// Test001SettingUIMaximumResults verifies that the UI maximum-results choice caps real launcher result rows.
// Flow: configure eight matching commands -> query them with limits of 5 and 15 -> count rows in the result viewport.
// Evidence: five rows are fully visible at the five-result limit, and all eight rows are fully visible at the fifteen-result limit.
func Test001SettingUIMaximumResults(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		configureMaximumResultsShellCommands(t, ctx, client, 8)

		previousValue := smoke.OpenSettingsAndReadChoice(t, ctx, client, "/appearance", "MaxResultCount")
		smoke.SelectSettingChoiceByLabel(t, ctx, client, "setting-choice-MaxResultCount", "5")
		t.Cleanup(func() {
			if previousValue != "5" {
				smoke.RestoreSettingChoice(t, client, "/appearance", "MaxResultCount", previousValue)
			}
		})
		if err := client.Hide(ctx); err != nil {
			t.Fatalf("close UI settings after selecting five maximum results: %v", err)
		}

		smoke.ShowLauncher(t, ctx, client)
		fiveResults := queryMaximumResults(t, ctx, client, 5)
		fiveHeight := resultListHeight(t, fiveResults)
		if visibleMaximumResultsCount(t, fiveResults) != 5 {
			t.Fatalf("fully visible maximum-results rows at limit 5 = %d, want 5", visibleMaximumResultsCount(t, fiveResults))
		}
		if err := client.Hide(ctx); err != nil {
			t.Fatalf("hide launcher after five-result query: %v", err)
		}

		smoke.OpenSettingsAndReadChoice(t, ctx, client, "/appearance", "MaxResultCount")
		smoke.SelectSettingChoiceByLabel(t, ctx, client, "setting-choice-MaxResultCount", "15")
		if err := client.Hide(ctx); err != nil {
			t.Fatalf("close UI settings after selecting 15 maximum results: %v", err)
		}

		smoke.ShowLauncher(t, ctx, client)
		eightResults := queryMaximumResults(t, ctx, client, 8)
		eightHeight := resultListHeight(t, eightResults)
		if visibleMaximumResultsCount(t, eightResults) != 8 {
			t.Fatalf("fully visible maximum-results rows at limit 15 = %d, want 8", visibleMaximumResultsCount(t, eightResults))
		}
		if rowHeight := launcherResultRowHeight(t, fiveResults); eightHeight-fiveHeight < 3*rowHeight-1 {
			t.Fatalf("result-list height growth = %.1f, want at least three row heights %.1f", eightHeight-fiveHeight, 3*rowHeight)
		}
		smoke.AssertNoDiagnostics(t, eightResults)
	})
}

// configureMaximumResultsShellCommands creates a deterministic result set through the real Shell settings form.
func configureMaximumResultsShellCommands(t *testing.T, ctx context.Context, client *automationdriver.Client, count int) {
	t.Helper()
	smoke.OpenInstalledPluginSettings(t, ctx, client, "8a4b5c6d-7e8f-9a0b-1c2d-3e4f5a6b7c8d")
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, found := automationdriver.Find(snapshot, "plugin-settings-field-3-add")
		return found
	}); err != nil {
		t.Fatalf("wait for Shell command settings: %v", err)
	}
	for index := range count {
		if err := client.Perform(ctx, "plugin-settings-field-3-add", woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("add Shell command %d: %v", index, err)
		}
		waitForMaximumResultsShellCommandEditor(t, ctx, client)
		alias := fmt.Sprintf("%s-%d", maximumResultsShellCommandPrefix, index)
		if err := client.Perform(ctx, "form-table-row-field-0", woxui.AccessibilityActionSetValue, alias); err != nil {
			t.Fatalf("set Shell command %d alias: %v", index, err)
		}
		if err := client.Perform(ctx, "form-table-row-field-1", woxui.AccessibilityActionSetValue, "echo smoke maximum results"); err != nil {
			t.Fatalf("set Shell command %d script: %v", index, err)
		}
		if err := client.Perform(ctx, "form-table-row-field-4", woxui.AccessibilityActionToggle, ""); err != nil {
			t.Fatalf("enable Shell command %d: %v", index, err)
		}
		if err := client.Perform(ctx, "form-table-row-save", woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("save Shell command %d: %v", index, err)
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			_, editorFound := automationdriver.Find(snapshot, "form-table-row-save")
			_, rowFound := automationdriver.Find(snapshot, fmt.Sprintf("plugin-settings-field-3-row-%d-edit", index))
			return !editorFound && rowFound
		}); err != nil {
			t.Fatalf("wait for saved Shell command %d: %v", index, err)
		}
	}
	if err := client.Hide(ctx); err != nil {
		t.Fatalf("close Shell command settings: %v", err)
	}
}

// waitForMaximumResultsShellCommandEditor waits until the Shell table editor accepts a complete command row.
func waitForMaximumResultsShellCommandEditor(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, aliasFound := automationdriver.Find(snapshot, "form-table-row-field-0")
		_, commandFound := automationdriver.Find(snapshot, "form-table-row-field-1")
		enabled, enabledFound := automationdriver.Find(snapshot, "form-table-row-field-4")
		_, saveFound := automationdriver.Find(snapshot, "form-table-row-save")
		return aliasFound && commandFound && enabledFound && !enabled.Checked && saveFound
	}); err != nil {
		t.Fatalf("wait for Shell command row editor: %v", err)
	}
}

// queryMaximumResults runs the configured prefix query and waits until the expected number of result rows reaches the semantic viewport.
func queryMaximumResults(t *testing.T, ctx context.Context, client *automationdriver.Client, expectedCount int) woxwidget.AutomationSnapshot {
	t.Helper()
	snapshot := smoke.ReplaceLauncherQuery(t, ctx, client, maximumResultsShellCommandPrefix)
	snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		results, resultsFound := automationdriver.Find(snapshot, "launcher.results")
		return resultsFound && results.Value == "complete" && maximumResultsCount(snapshot) >= expectedCount
	})
	if err != nil {
		t.Fatalf("wait for %d maximum-results commands: %v", expectedCount, err)
	}
	return snapshot
}

// visibleMaximumResultsCount excludes virtual-list overscan and counts only complete rows inside the result viewport.
func visibleMaximumResultsCount(t *testing.T, snapshot woxwidget.AutomationSnapshot) int {
	t.Helper()
	results, found := automationdriver.Find(snapshot, "launcher.results")
	if !found {
		t.Fatal("launcher result viewport was not found")
	}
	viewportTop := results.Bounds.Y
	viewportBottom := results.Bounds.Y + results.Bounds.Height
	count := 0
	for _, node := range snapshot.Tree.Nodes {
		if !strings.HasPrefix(node.AutomationID, "launcher.result.") || !strings.HasPrefix(node.Label, maximumResultsShellCommandPrefix) {
			continue
		}
		rowTop := node.Bounds.Y
		rowBottom := node.Bounds.Y + node.Bounds.Height
		if rowTop >= viewportTop && rowBottom <= viewportBottom {
			count++
		}
	}
	return count
}

// maximumResultsCount reports the visible rows produced by the deterministic Shell command prefix.
func maximumResultsCount(snapshot woxwidget.AutomationSnapshot) int {
	count := 0
	for _, node := range snapshot.Tree.Nodes {
		if strings.HasPrefix(node.AutomationID, "launcher.result.") && strings.HasPrefix(node.Label, maximumResultsShellCommandPrefix) {
			count++
		}
	}
	return count
}

// resultListHeight returns the semantic viewport height for the visible launcher result list.
func resultListHeight(t *testing.T, snapshot woxwidget.AutomationSnapshot) float32 {
	t.Helper()
	results, found := automationdriver.Find(snapshot, "launcher.results")
	if !found || results.Bounds.Height <= 0 {
		t.Fatalf("visible launcher result list has invalid bounds: found=%t bounds=%+v", found, results.Bounds)
	}
	return results.Bounds.Height
}

// launcherResultRowHeight returns one visible result-row height for the viewport comparison.
func launcherResultRowHeight(t *testing.T, snapshot woxwidget.AutomationSnapshot) float32 {
	t.Helper()
	for _, node := range snapshot.Tree.Nodes {
		if strings.HasPrefix(node.AutomationID, "launcher.result.") && strings.HasPrefix(node.Label, maximumResultsShellCommandPrefix) {
			return node.Bounds.Height
		}
	}
	t.Fatal("no visible maximum-results row has bounds")
	return 0
}
