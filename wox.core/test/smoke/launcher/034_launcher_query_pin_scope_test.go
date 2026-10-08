//go:build wox_ui_smoke

package query

import (
	"context"
	"strings"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const (
	pinScopeQuery      = "wox-smoke pin-ranking scope"
	pinScopeOtherQuery = "wox-smoke pin-ranking other"
	pinRankingSecondID = "pin-ranking-second-fixture"
)

// Test034LauncherQueryPinScope verifies a query pin promotes one result only for that query text.
// Flow: equalize action history -> pin the second fixture -> open the same rows with different query text -> unpin the original query.
// Evidence: the pinned row is first and shows the pin tail only for the pinned query, then returns to its original place after unpin.
func Test034LauncherQueryPinScope(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.ShowLauncher(t, ctx, client)

		initial := queryPinScopeResults(t, ctx, client, pinScopeQuery)
		assertPinRankingOrder(t, initial, pinRankingFirstAlias, pinRankingSecondAlias)
		if resultShowsPinTail(initial, pinRankingSecondID) {
			t.Fatal("pin tail was visible before pinning")
		}

		// Pinning records usage for every query. Two actions on the leading row keep it
		// ahead of the target's single pin action, so only the query pin can reorder the target.
		activatePinRankingAction(t, ctx, client, initial, pinRankingFirstAlias, pinActionPrefix)
		smoke.WaitForResultActionsClosed(t, ctx, client)
		controlPinned := queryPinScopeResults(t, ctx, client, pinScopeQuery)
		activatePinRankingAction(t, ctx, client, controlPinned, pinRankingFirstAlias, unpinActionPrefix)
		smoke.WaitForResultActionsClosed(t, ctx, client)

		baseline := queryPinScopeResults(t, ctx, client, pinScopeQuery)
		assertPinRankingOrder(t, baseline, pinRankingFirstAlias, pinRankingSecondAlias)
		activatePinRankingAction(t, ctx, client, baseline, pinRankingSecondAlias, pinActionPrefix)
		smoke.WaitForResultActionsClosed(t, ctx, client)

		pinned := queryPinScopeResults(t, ctx, client, pinScopeQuery)
		assertPinRankingOrder(t, pinned, pinRankingSecondAlias, pinRankingFirstAlias)
		if !resultShowsPinTail(pinned, pinRankingSecondID) {
			t.Fatal("pinned result did not show the pin tail")
		}

		other := queryPinScopeResults(t, ctx, client, pinScopeOtherQuery)
		assertPinRankingOrder(t, other, pinRankingFirstAlias, pinRankingSecondAlias)
		if resultShowsPinTail(other, pinRankingSecondID) {
			t.Fatal("pin tail was visible in a different query")
		}
		assertPinRankingAction(t, ctx, client, other, pinRankingSecondAlias, pinActionPrefix, unpinActionPrefix)

		stillPinned := queryPinScopeResults(t, ctx, client, pinScopeQuery)
		assertPinRankingOrder(t, stillPinned, pinRankingSecondAlias, pinRankingFirstAlias)
		activatePinRankingAction(t, ctx, client, stillPinned, pinRankingSecondAlias, unpinActionPrefix)
		smoke.WaitForResultActionsClosed(t, ctx, client)

		unpinned := queryPinScopeResults(t, ctx, client, pinScopeQuery)
		assertPinRankingOrder(t, unpinned, pinRankingFirstAlias, pinRankingSecondAlias)
		if resultShowsPinTail(unpinned, pinRankingSecondID) {
			t.Fatal("pin tail remained after unpin")
		}
		smoke.AssertNoDiagnostics(t, unpinned)
	})
}

// queryPinScopeResults forces a fresh generation for one pin-ranking query.
func queryPinScopeResults(t *testing.T, ctx context.Context, client *automationdriver.Client, query string) woxwidget.AutomationSnapshot {
	t.Helper()
	smoke.ReplaceLauncherQuery(t, ctx, client, query)
	snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		input, inputFound := automationdriver.Find(snapshot, "launcher.query.input")
		results, resultsFound := automationdriver.Find(snapshot, "launcher.results")
		return inputFound && input.Value == query && resultsFound && results.Value == "complete" && len(pinRankingOrder(snapshot)) == 2
	})
	if err != nil {
		t.Fatalf("wait for pin-scope results %q: %v", query, err)
	}
	return snapshot
}

// resultShowsPinTail reports whether the pin glyph for one stable result is in the semantic tree.
func resultShowsPinTail(snapshot woxwidget.AutomationSnapshot, resultID string) bool {
	prefix := "result-tail-" + resultID + "-"
	for _, node := range snapshot.Tree.Nodes {
		if strings.HasPrefix(node.AutomationID, prefix) {
			return true
		}
	}
	return false
}

// assertPinRankingAction opens the selected result and checks which system pin action is available.
func assertPinRankingAction(t *testing.T, ctx context.Context, client *automationdriver.Client, snapshot woxwidget.AutomationSnapshot, alias, presentPrefix, absentPrefix string) {
	t.Helper()
	resultID, found := smoke.FindLauncherResult(snapshot, alias)
	if !found {
		t.Fatalf("pin-ranking result %q was not found", alias)
	}
	smoke.SelectLauncherResult(t, ctx, client, resultID)
	smoke.OpenResultActionPanel(t, ctx, client)
	panel, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, present := automationdriver.FindByAutomationIDPrefix(snapshot, presentPrefix)
		_, absent := automationdriver.FindByAutomationIDPrefix(snapshot, absentPrefix)
		return present && !absent
	})
	if err != nil {
		t.Fatalf("wait for pin action %q without %q: %v", presentPrefix, absentPrefix, err)
	}
	if err := client.PressKey(ctx, woxui.KeyEscape, 0); err != nil {
		t.Fatalf("close pin action panel: %v", err)
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		for _, node := range snapshot.Tree.Nodes {
			if strings.HasPrefix(node.AutomationID, "action-result-") {
				return false
			}
		}
		return true
	}); err != nil {
		t.Fatalf("wait for pin action panel to close: %v", err)
	}
	smoke.AssertNoDiagnostics(t, panel)
}
