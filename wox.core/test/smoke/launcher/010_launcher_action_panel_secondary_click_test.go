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
		// The wait below is satisfied by any open panel, so an Action Panel left open by
		// an earlier case would pass it without the secondary click doing anything.
		if _, alreadyOpen := automationdriver.Find(snapshot, "action-search"); alreadyOpen {
			t.Fatal("Action Panel was already open before the secondary click, so this case cannot prove the secondary-click path")
		}

		// Query completion can precede the native resize. The result already has
		// semantics then, but its center is outside the clipped results viewport.
		snapshot, err := client.WaitFor(ctx, func(current woxwidget.AutomationSnapshot) bool {
			result, resultFound := automationdriver.Find(current, resultID)
			viewport, viewportFound := automationdriver.Find(current, "launcher.results")
			return resultFound && viewportFound && result.Bounds.Width > 0 && result.Bounds.Height > 0 &&
				result.Bounds.X >= viewport.Bounds.X && result.Bounds.Y >= viewport.Bounds.Y &&
				result.Bounds.X+result.Bounds.Width <= viewport.Bounds.X+viewport.Bounds.Width &&
				result.Bounds.Y+result.Bounds.Height <= viewport.Bounds.Y+viewport.Bounds.Height
		})
		if err != nil {
			t.Fatalf("wait for clickable result after resize: %v", err)
		}
		result, _ := automationdriver.Find(snapshot, resultID)
		// Both semantic bounds and injected pointer positions are logical client coordinates.
		position := woxui.Point{X: result.Bounds.X + result.Bounds.Width/2, Y: result.Bounds.Y + result.Bounds.Height/2}
		if err := client.Pointer(ctx, woxui.PointerEvent{Kind: woxui.PointerDown, Button: woxui.PointerButtonSecondary, Position: position}); err != nil {
			t.Fatalf("secondary-click Shell result: %v", err)
		}

		snapshot, err = client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
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
