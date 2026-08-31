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

const (
	pinRankingQuery       = "wox-smoke pin-ranking "
	pinRankingFirstAlias  = "Pin ranking first fixture"
	pinRankingSecondAlias = "Pin ranking second fixture"
	pinActionPrefix       = "action-result-__system_pin_in_query__-"
	unpinActionPrefix     = "action-result-__system_unpin_in_query__-"
)

// Test003LauncherQueryPinResult verifies pinning promotes a non-leading result for the same query.
// Flow: query two deterministic results -> equalize their action history -> pin the second result -> repeat the query.
// Evidence: the pinned result moves from second to first after both results have equal action history.
func Test003LauncherQueryPinResult(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
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
		smoke.AssertNoDiagnostics(t, pinned)
	})
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
		if strings.HasPrefix(node.AutomationID, "launcher.result.") && strings.HasPrefix(node.Label, "Pin ranking ") {
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
