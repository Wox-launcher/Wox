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

const actionPanelShellCommand = "echo wox-action-panel-output"

// Test007LauncherActionPanelResultAction verifies executing a selected result action from the action panel.
// Flow: query a Shell command -> open the action panel -> activate Execute -> wait for terminal completion.
// Evidence: the terminal preview contains the command output and the action panel has closed.
func Test007LauncherActionPanelResultAction(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.ShowLauncher(t, ctx, client)
		snapshot := smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, "> "+actionPanelShellCommand)
		if _, found := smoke.FindLauncherResult(snapshot, actionPanelShellCommand); !found {
			t.Fatalf("Shell result %q was not found", actionPanelShellCommand)
		}

		snapshot = smoke.OpenResultActionPanel(t, ctx, client)
		execute, found := automationdriver.Find(snapshot, "action-result-execute-0")
		if !found || !execute.Selected {
			t.Fatalf("Execute action = found %v selected %v, want selected action", found, execute.Selected)
		}
		if err := client.Perform(ctx, execute.AutomationID, woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("activate Shell Execute action: %v", err)
		}

		snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			status, statusFound := automationdriver.Find(snapshot, "launcher.preview.terminal.status")
			output, outputFound := automationdriver.Find(snapshot, "launcher.preview.terminal.output")
			_, panelFound := automationdriver.Find(snapshot, "action-search")
			return statusFound && status.Value == "completed" && outputFound && strings.Contains(output.Value, "wox-action-panel-output") && !panelFound
		})
		if err != nil {
			t.Fatalf("wait for Action Panel Shell output: %v", err)
		}
		smoke.AssertNoDiagnostics(t, snapshot)
	})
}

// actionPanelResultNodes returns visible result actions from the current semantics snapshot.
func actionPanelResultNodes(snapshot woxwidget.AutomationSnapshot) []woxui.AccessibilityNode {
	nodes := make([]woxui.AccessibilityNode, 0)
	for _, node := range snapshot.Tree.Nodes {
		if strings.HasPrefix(node.AutomationID, "action-result-") {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

// selectedActionPanelResult returns the currently selected visible result action.
func selectedActionPanelResult(snapshot woxwidget.AutomationSnapshot) (woxui.AccessibilityNode, bool) {
	for _, node := range actionPanelResultNodes(snapshot) {
		if node.Selected {
			return node, true
		}
	}
	return woxui.AccessibilityNode{}, false
}
