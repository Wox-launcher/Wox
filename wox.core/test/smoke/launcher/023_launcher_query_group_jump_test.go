//go:build wox_ui_smoke

package query

import (
	"context"
	"runtime"
	"strings"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const (
	groupJumpSmokeQuery = "wox-smoke group-jump "
	groupJumpURL        = "Group jump URL"
	groupJumpOpen       = "Group jump open"
	groupJumpAFirst     = "Group jump A first"
	groupJumpALast      = "Group jump A last"
	groupJumpBFirst     = "Group jump B first"
	groupJumpBLast      = "Group jump B last"
)

// Test023LauncherQueryGroupJump verifies group-jump navigation jumps by result group.
// Flow: query ungrouped rows above two named groups -> Option/Ctrl+Down into group A -> Down to that group's last result -> Option/Ctrl+Up -> continue Down/Up across later groups.
// Evidence: Option/Ctrl+Up from a group's last result selects that group's first result instead of the ungrouped top row, then later jumps land on the next group's first, the last group's last, and that last group's first.
func Test023LauncherQueryGroupJump(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.ShowLauncher(t, ctx, client)
		snapshot := smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, groupJumpSmokeQuery)
		if !groupJumpResultsPresent(snapshot) {
			snapshot = waitForGroupJumpResults(t, ctx, client)
		}
		if selectedLauncherResultLabel(snapshot) != groupJumpURL {
			snapshot = waitForSelectedGroupJumpResult(t, ctx, client, groupJumpURL)
		}
		smoke.AssertNoDiagnostics(t, snapshot)

		modifier := groupJumpModifier()
		if err := client.PressKey(ctx, woxui.KeyArrowDown, modifier); err != nil {
			t.Fatalf("jump to first named group: %v", err)
		}
		waitForSelectedGroupJumpResult(t, ctx, client, groupJumpAFirst)

		if err := client.PressKey(ctx, woxui.KeyArrowDown, 0); err != nil {
			t.Fatalf("move to last result of current group: %v", err)
		}
		waitForSelectedGroupJumpResult(t, ctx, client, groupJumpALast)

		if err := client.PressKey(ctx, woxui.KeyArrowUp, modifier); err != nil {
			t.Fatalf("jump to first result of current group: %v", err)
		}
		waitForSelectedGroupJumpResult(t, ctx, client, groupJumpAFirst)

		if err := client.PressKey(ctx, woxui.KeyArrowDown, modifier); err != nil {
			t.Fatalf("jump to next group: %v", err)
		}
		waitForSelectedGroupJumpResult(t, ctx, client, groupJumpBFirst)

		if err := client.PressKey(ctx, woxui.KeyArrowDown, modifier); err != nil {
			t.Fatalf("jump to last result of last group: %v", err)
		}
		waitForSelectedGroupJumpResult(t, ctx, client, groupJumpBLast)

		if err := client.PressKey(ctx, woxui.KeyArrowUp, modifier); err != nil {
			t.Fatalf("jump to first result of last group: %v", err)
		}
		snapshot = waitForSelectedGroupJumpResult(t, ctx, client, groupJumpBFirst)
		smoke.AssertNoDiagnostics(t, snapshot)
	})
}

func groupJumpModifier() woxui.KeyModifiers {
	if runtime.GOOS == "darwin" {
		return woxui.KeyModifierAlt
	}
	return woxui.KeyModifierControl
}

func groupJumpResultsPresent(snapshot woxwidget.AutomationSnapshot) bool {
	for _, label := range []string{groupJumpURL, groupJumpOpen, groupJumpAFirst, groupJumpALast, groupJumpBFirst, groupJumpBLast} {
		if _, found := smoke.FindLauncherResult(snapshot, label); !found {
			return false
		}
	}
	return true
}

func waitForGroupJumpResults(t *testing.T, ctx context.Context, client *automationdriver.Client) woxwidget.AutomationSnapshot {
	t.Helper()
	snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		results, resultsFound := automationdriver.Find(snapshot, "launcher.results")
		return resultsFound && results.Value == "complete" && groupJumpResultsPresent(snapshot)
	})
	if err != nil {
		t.Fatalf("wait for group-jump results: %v", err)
	}
	return snapshot
}

func waitForSelectedGroupJumpResult(t *testing.T, ctx context.Context, client *automationdriver.Client, label string) woxwidget.AutomationSnapshot {
	t.Helper()
	snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		return selectedLauncherResultLabel(snapshot) == label
	})
	if err != nil {
		t.Fatalf("wait for selected group-jump result %q: %v", label, err)
	}
	return snapshot
}

func selectedLauncherResultLabel(snapshot woxwidget.AutomationSnapshot) string {
	for _, node := range snapshot.Tree.Nodes {
		if strings.HasPrefix(node.AutomationID, "launcher.result.") && node.Selected {
			return node.Label
		}
	}
	return ""
}
