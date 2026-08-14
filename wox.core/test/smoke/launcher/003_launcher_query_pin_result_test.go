//go:build wox_ui_smoke

package query

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

const (
	pinRankingShellPluginID = "8a4b5c6d-7e8f-9a0b-1c2d-3e4f5a6b7c8d"
	pinRankingQuery         = "woxsmokepin"
	pinRankingFirstAlias    = pinRankingQuery + "-first"
	pinRankingSecondAlias   = pinRankingQuery + "-second"
	pinActionPrefix         = "action-result-__system_pin_in_query__-"
	unpinActionPrefix       = "action-result-__system_unpin_in_query__-"
)

// Test003LauncherQueryPinResult verifies pinning promotes a non-leading result for the same query.
// Flow: configure two matching results -> equalize their action history -> pin the second result -> repeat the query -> unpin and clean up.
// Evidence: the pinned result moves from second to first, then the original order returns after both results have equal action history.
func Test003LauncherQueryPinResult(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		configurePinRankingResults(t, ctx, client)
		smoke.ShowLauncher(t, ctx, client)

		initial := queryPinRankingResults(t, ctx, client)
		assertPinRankingOrder(t, initial, pinRankingFirstAlias, pinRankingSecondAlias)

		// Pinning and unpinning the leading result gives it two action-history entries.
		// The target receives the same two entries across its pin/unpin cycle, so
		// only the pin score can move it ahead while it is pinned.
		activatePinRankingAction(t, ctx, client, initial, pinRankingFirstAlias, pinActionPrefix)
		smoke.WaitForResultActionsClosed(t, ctx, client)
		controlPinned := queryPinRankingResults(t, ctx, client)
		activatePinRankingAction(t, ctx, client, controlPinned, pinRankingFirstAlias, unpinActionPrefix)
		smoke.WaitForResultActionsClosed(t, ctx, client)

		baseline := queryPinRankingResults(t, ctx, client)
		assertPinRankingOrder(t, baseline, pinRankingFirstAlias, pinRankingSecondAlias)
		activatePinRankingAction(t, ctx, client, baseline, pinRankingSecondAlias, pinActionPrefix)
		smoke.WaitForResultActionsClosed(t, ctx, client)

		pinned := queryPinRankingResults(t, ctx, client)
		assertPinRankingOrder(t, pinned, pinRankingSecondAlias, pinRankingFirstAlias)
		activatePinRankingAction(t, ctx, client, pinned, pinRankingSecondAlias, unpinActionPrefix)
		smoke.WaitForResultActionsClosed(t, ctx, client)

		unpinned := queryPinRankingResults(t, ctx, client)
		assertPinRankingOrder(t, unpinned, pinRankingFirstAlias, pinRankingSecondAlias)
		smoke.AssertNoDiagnostics(t, unpinned)

		deletePinRankingResults(t, ctx, client)
	})
}

// configurePinRankingResults borrows Shell settings to create two deterministic launcher results.
func configurePinRankingResults(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	smoke.OpenInstalledPluginSettings(t, ctx, client, pinRankingShellPluginID)
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, found := automationdriver.Find(snapshot, "plugin-settings-field-2-add")
		return found
	}); err != nil {
		t.Fatalf("wait for Shell command settings used by pin ranking: %v", err)
	}
	for index, alias := range []string{pinRankingFirstAlias, pinRankingSecondAlias} {
		if err := client.Perform(ctx, "plugin-settings-field-2-add", woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("add pin-ranking result %d: %v", index, err)
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			_, aliasFound := automationdriver.Find(snapshot, "form-table-row-field-0")
			_, commandFound := automationdriver.Find(snapshot, "form-table-row-field-1")
			_, saveFound := automationdriver.Find(snapshot, "form-table-row-save")
			return aliasFound && commandFound && saveFound
		}); err != nil {
			t.Fatalf("wait for pin-ranking result %d editor: %v", index, err)
		}
		if err := client.Perform(ctx, "form-table-row-field-0", woxui.AccessibilityActionSetValue, alias); err != nil {
			t.Fatalf("set pin-ranking result %d alias: %v", index, err)
		}
		if err := client.Perform(ctx, "form-table-row-field-1", woxui.AccessibilityActionSetValue, "echo smoke pin ranking"); err != nil {
			t.Fatalf("set pin-ranking result %d command: %v", index, err)
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			enabled, found := automationdriver.Find(snapshot, "form-table-row-field-4")
			return found && !enabled.Checked
		}); err != nil {
			t.Fatalf("wait for pin-ranking result %d enabled field: %v", index, err)
		}
		if err := client.Perform(ctx, "form-table-row-field-4", woxui.AccessibilityActionToggle, ""); err != nil {
			t.Fatalf("enable pin-ranking result %d: %v", index, err)
		}
		if err := client.Perform(ctx, "form-table-row-save", woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("save pin-ranking result %d: %v", index, err)
		}
		rowID := fmt.Sprintf("plugin-settings-field-2-row-%d-edit", index)
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			_, rowFound := automationdriver.Find(snapshot, rowID)
			_, editorFound := automationdriver.Find(snapshot, "form-table-row-save")
			return rowFound && !editorFound
		}); err != nil {
			t.Fatalf("wait for pin-ranking result %d to persist: %v", index, err)
		}
	}
	if err := client.Hide(ctx); err != nil {
		t.Fatalf("close Shell settings after configuring pin-ranking results: %v", err)
	}
}

// queryPinRankingResults forces a fresh generation and returns the two matching results in semantic order.
func queryPinRankingResults(t *testing.T, ctx context.Context, client *automationdriver.Client) woxwidget.AutomationSnapshot {
	t.Helper()
	smoke.ReplaceLauncherQuery(t, ctx, client, pinRankingQuery)
	snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		results, resultsFound := automationdriver.Find(snapshot, "launcher.results")
		return resultsFound && results.Value == "complete" && len(pinRankingOrder(snapshot)) == 2
	})
	if err != nil {
		t.Fatalf("wait for pin-ranking results: %v", err)
	}
	return snapshot
}

func pinRankingOrder(snapshot woxwidget.AutomationSnapshot) []string {
	order := make([]string, 0, 2)
	for _, node := range snapshot.Tree.Nodes {
		if strings.HasPrefix(node.AutomationID, "launcher.result.") && strings.HasPrefix(node.Label, pinRankingQuery+"-") {
			order = append(order, node.Label)
		}
	}
	return order
}

func assertPinRankingOrder(t *testing.T, snapshot woxwidget.AutomationSnapshot, first, second string) {
	t.Helper()
	order := pinRankingOrder(snapshot)
	if len(order) != 2 || order[0] != first || order[1] != second {
		t.Fatalf("pin-ranking result order = %v, want [%s %s]", order, first, second)
	}
}

// activatePinRankingAction selects one result and invokes its current system pin action.
func activatePinRankingAction(t *testing.T, ctx context.Context, client *automationdriver.Client, snapshot woxwidget.AutomationSnapshot, alias, actionPrefix string) {
	t.Helper()
	resultID, found := smoke.FindLauncherResult(snapshot, alias)
	if !found {
		t.Fatalf("pin-ranking result %q was not found", alias)
	}
	smoke.SelectLauncherResult(t, ctx, client, resultID)
	smoke.ActivateSelectedResultAction(t, ctx, client, actionPrefix)
}

// deletePinRankingResults removes the deterministic results from shared Shell settings.
func deletePinRankingResults(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	if err := client.Hide(ctx); err != nil {
		t.Fatalf("hide launcher before deleting pin-ranking results: %v", err)
	}
	smoke.OpenInstalledPluginSettings(t, ctx, client, pinRankingShellPluginID)
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, firstFound := automationdriver.Find(snapshot, "plugin-settings-field-2-row-0-delete")
		_, secondFound := automationdriver.Find(snapshot, "plugin-settings-field-2-row-1-delete")
		return firstFound && secondFound
	}); err != nil {
		t.Fatalf("wait for pin-ranking result deletion actions: %v", err)
	}
	for _, rowIndex := range []int{1, 0} {
		rowID := fmt.Sprintf("plugin-settings-field-2-row-%d-delete", rowIndex)
		if err := client.Perform(ctx, rowID, woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("delete pin-ranking result row %d: %v", rowIndex, err)
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			_, found := automationdriver.Find(snapshot, "form-table-delete-confirm")
			return found
		}); err != nil {
			t.Fatalf("wait to confirm pin-ranking result row %d deletion: %v", rowIndex, err)
		}
		if err := client.Perform(ctx, "form-table-delete-confirm", woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("confirm pin-ranking result row %d deletion: %v", rowIndex, err)
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			_, dialogFound := automationdriver.Find(snapshot, "form-table-delete-dialog")
			_, rowFound := automationdriver.Find(snapshot, rowID)
			return !dialogFound && !rowFound
		}); err != nil {
			t.Fatalf("wait for pin-ranking result row %d deletion: %v", rowIndex, err)
		}
	}
	if err := client.Hide(ctx); err != nil {
		t.Fatalf("close Shell settings after deleting pin-ranking results: %v", err)
	}
}
