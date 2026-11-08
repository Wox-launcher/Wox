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

// Test043LauncherResultSelectionStability exercises keyboard selection while results arrive
// and keeps the highlight through both phases of a refresh of the same query.
func Test043LauncherResultSelectionStability(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		const targetID = "launcher.result.selection-target"
		const lateID = "launcher.result.selection-late"
		waitBatch := func(phase string, complete, keepSelection bool) woxwidget.AutomationSnapshot {
			t.Helper()
			snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
				results, resultsFound := automationdriver.Find(snapshot, "launcher.results")
				target, targetFound := automationdriver.Find(snapshot, targetID)
				_, lateFound := automationdriver.Find(snapshot, lateID)
				if keepSelection && targetFound && !target.Selected {
					t.Fatalf("%s lost the highlight on %s", phase, targetID)
				}
				return resultsFound && targetFound && (results.Value == "complete") == complete && lateFound == complete
			})
			if err != nil {
				t.Fatalf("wait for %s: %v", phase, err)
			}
			return snapshot
		}
		enter := func() {
			t.Helper()
			if err := client.PressKey(ctx, woxui.KeyEnter, 0); err != nil {
				t.Fatal(err)
			}
		}

		smoke.ShowLauncher(t, ctx, client)
		for _, mode := range []string{"reorder", "refresh"} {
			if err := client.Perform(ctx, "launcher.query.input", woxui.AccessibilityActionSetValue, "wox-smoke selection-stability "+mode); err != nil {
				t.Fatal(err)
			}
			waitBatch("initial query batch", false, false)
			if err := client.PressKey(ctx, woxui.KeyArrowDown, 0); err != nil {
				t.Fatal(err)
			}
			if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
				target, found := automationdriver.Find(snapshot, targetID)
				return found && target.Selected
			}); err != nil {
				t.Fatalf("wait for keyboard-selected row: %v", err)
			}
			enter()
			completed := waitBatch("final query batch", true, true)
			if mode == "reorder" {
				target, _ := automationdriver.Find(completed, targetID)
				late, _ := automationdriver.Find(completed, lateID)
				if late.Bounds.Y >= target.Bounds.Y {
					t.Fatal("the late result did not move the highlighted row")
				}
				continue
			}
			enter()
			waitBatch("initial refresh batch", false, true)
			enter()
			completed = waitBatch("final refresh batch", true, true)
			smoke.AssertNoDiagnostics(t, completed)
		}
	})
}
