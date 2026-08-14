//go:build wox_ui_smoke

package query

import (
	"context"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// Test010LauncherActionPanelSecondaryClick verifies the result-row secondary-click entry into actions.
// Flow: query a Shell command -> secondary-click the live result row -> inspect the opened actions.
// Evidence: the focused action filter and the selected result action are present for the clicked result.
func Test010LauncherActionPanelSecondaryClick(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.ShowLauncher(t, ctx, client)
		snapshot := smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, "> "+actionPanelShellCommand)
		resultID, found := smoke.FindLauncherResult(snapshot, actionPanelShellCommand)
		if !found {
			t.Fatalf("Shell result %q was not found", actionPanelShellCommand)
		}
		result, found := automationdriver.Find(snapshot, resultID)
		if !found {
			t.Fatalf("result node %q was not found before secondary click", resultID)
		}

		// Result semantics expose activation but not the secondary-click gesture, so resolve the pointer target from the current semantic bounds.
		position := woxui.Point{X: result.Bounds.X + result.Bounds.Width/2, Y: result.Bounds.Y + result.Bounds.Height/2}
		if err := client.Pointer(ctx, woxui.PointerEvent{Kind: woxui.PointerDown, Button: woxui.PointerButtonSecondary, Position: position}); err != nil {
			t.Fatalf("secondary-click Shell result: %v", err)
		}

		snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			search, searchFound := automationdriver.Find(snapshot, "action-search")
			execute, executeFound := automationdriver.Find(snapshot, "action-result-execute-0")
			return searchFound && search.Focused && executeFound && execute.Selected
		})
		if err != nil {
			t.Fatalf("wait for actions after secondary click: %v", err)
		}
		if err := client.PressKey(ctx, woxui.KeyEscape, 0); err != nil {
			t.Fatalf("close secondary-click Action Panel: %v", err)
		}
		snapshot, err = client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			_, searchFound := automationdriver.Find(snapshot, "action-search")
			return !searchFound && len(actionPanelResultNodes(snapshot)) == 0
		})
		if err != nil {
			t.Fatalf("wait for secondary-click Action Panel close: %v", err)
		}
		smoke.AssertNoDiagnostics(t, snapshot)
	})
}
