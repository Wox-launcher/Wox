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

// Test024LauncherResultStillPointerHover verifies a still pointer does not hover a rebuilt result.
// Flow: query two pin-ranking fixtures -> move onto the unselected row -> replace the query with quick-select -> move onto the new unselected row.
// Evidence: no rebuilt result is Hovered until a real pointer move, and keyboard selection stays on the default row.
func Test024LauncherResultStillPointerHover(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.ShowLauncher(t, ctx, client)
		snapshot := smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, pinRankingQuery)
		hoverID, found := unselectedLauncherResultID(snapshot)
		if !found {
			t.Fatalf("need a non-selected pin-ranking result to hover; %s", formatLauncherResultHoverNodes(snapshot))
		}
		if _, err := client.MovePointerTo(ctx, hoverID); err != nil {
			t.Fatalf("move onto unselected pin-ranking result: %v", err)
		}
		snapshot, err := client.WaitFor(ctx, func(current woxwidget.AutomationSnapshot) bool {
			node, ok := automationdriver.Find(current, hoverID)
			return ok && node.Hovered && !node.Selected
		})
		if err != nil {
			t.Fatalf("wait for hover on unselected pin-ranking result: %v; %s", err, formatLauncherResultHoverNodes(snapshot))
		}

		snapshot = smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, quickSelectSmokeQuery)
		if _, found := smoke.FindLauncherResult(snapshot, quickSelectFirstFixture); !found {
			t.Fatal("quick-select first fixture was not found")
		}
		if _, found := smoke.FindLauncherResult(snapshot, quickSelectSecondFixture); !found {
			t.Fatal("quick-select second fixture was not found")
		}
		if hovered, hoveredFound := hoveredLauncherResult(snapshot); hoveredFound {
			t.Fatalf("rebuilt result %q was hovered by a still pointer; %s", hovered.Label, formatLauncherResultHoverNodes(snapshot))
		}
		selected, selectedFound := selectedLauncherResult(snapshot)
		if !selectedFound {
			t.Fatalf("rebuilt results should keep a selected row; %s", formatLauncherResultHoverNodes(snapshot))
		}
		rebuildHoverID, found := unselectedLauncherResultID(snapshot)
		if !found {
			t.Fatalf("need a non-selected rebuilt result to hover; %s", formatLauncherResultHoverNodes(snapshot))
		}

		if _, err := client.MovePointerTo(ctx, rebuildHoverID); err != nil {
			t.Fatalf("move onto rebuilt unselected result: %v", err)
		}
		snapshot, err = client.WaitFor(ctx, func(current woxwidget.AutomationSnapshot) bool {
			hovered, ok := automationdriver.Find(current, rebuildHoverID)
			currentSelected, selectedOK := selectedLauncherResult(current)
			return ok && hovered.Hovered && !hovered.Selected && selectedOK && currentSelected.AutomationID == selected.AutomationID
		})
		if err != nil {
			t.Fatalf("wait for hover after pointer move: %v; %s", err, formatLauncherResultHoverNodes(snapshot))
		}
		smoke.AssertNoDiagnostics(t, snapshot)
	})
}

func unselectedLauncherResultID(snapshot woxwidget.AutomationSnapshot) (string, bool) {
	for _, node := range snapshot.Tree.Nodes {
		if strings.HasPrefix(node.AutomationID, "launcher.result.") && !node.Selected {
			return node.AutomationID, true
		}
	}
	return "", false
}

func selectedLauncherResult(snapshot woxwidget.AutomationSnapshot) (woxui.AccessibilityNode, bool) {
	for _, node := range snapshot.Tree.Nodes {
		if strings.HasPrefix(node.AutomationID, "launcher.result.") && node.Selected {
			return node, true
		}
	}
	return woxui.AccessibilityNode{}, false
}

func hoveredLauncherResult(snapshot woxwidget.AutomationSnapshot) (woxui.AccessibilityNode, bool) {
	for _, node := range snapshot.Tree.Nodes {
		if strings.HasPrefix(node.AutomationID, "launcher.result.") && node.Hovered {
			return node, true
		}
	}
	return woxui.AccessibilityNode{}, false
}

func formatLauncherResultHoverNodes(snapshot woxwidget.AutomationSnapshot) string {
	parts := make([]string, 0)
	for _, node := range snapshot.Tree.Nodes {
		if strings.HasPrefix(node.AutomationID, "launcher.result.") {
			parts = append(parts, fmt.Sprintf("%s label=%q selected=%v hovered=%v", node.AutomationID, node.Label, node.Selected, node.Hovered))
		}
	}
	if len(parts) == 0 {
		return "result nodes=[]"
	}
	return "result nodes=[" + strings.Join(parts, "; ") + "]"
}
