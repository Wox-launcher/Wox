//go:build wox_ui_smoke

package queryhint

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxwidget "wox/ui/widget"
)

// enterVolumeHint uses real typing so the command matcher, hint and native editor participate.
func enterVolumeHint(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	smoke.ShowLauncher(t, ctx, client)
	if err := client.EnterText(ctx, "set volume"); err != nil {
		t.Fatal(err)
	}
	snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		input, found := automationdriver.Find(snapshot, "launcher.query.input")
		_, shown := automationdriver.Find(snapshot, "launcher.query.completion")
		return found && input.Value == "set volume" && !shown
	})
	if err != nil {
		t.Fatalf("bare command must not show a parameter hint: %v", err)
	}
	smoke.AssertNoDiagnostics(t, snapshot)
	if err := client.EnterText(ctx, " "); err != nil {
		t.Fatal(err)
	}
	waitInput(t, ctx, client, "set volume ", 11, 11)
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		hint, shown := automationdriver.Find(snapshot, "launcher.query.completion")
		return shown && strings.Contains(hint.Value, "100")
	}); err != nil {
		t.Fatalf("wait for volume hint after context separator: %v", err)
	}
}

// waitInput verifies the continuous document and its native selection range.
func waitInput(t *testing.T, ctx context.Context, client *automationdriver.Client, text string, start, end int) {
	t.Helper()
	snapshot, err := client.WaitForReason(ctx, func(snapshot woxwidget.AutomationSnapshot) (bool, string) {
		input, found := automationdriver.Find(snapshot, "launcher.query.input")
		valid := found && input.Value == text && input.HasTextSelection && input.SelectionStart == start && input.SelectionEnd == end
		return valid, fmt.Sprintf("input found=%t value=%q selection=%d:%d", found, input.Value, input.SelectionStart, input.SelectionEnd)
	})
	if err != nil {
		t.Fatalf("wait for %q selection %d:%d: %v", text, start, end, err)
	}
	smoke.AssertNoDiagnostics(t, snapshot)
}

// waitVolume verifies the current input generation produced the expected percentage without executing it.
func waitVolume(t *testing.T, ctx context.Context, client *automationdriver.Client, percent string) {
	t.Helper()
	snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		input, found := automationdriver.Find(snapshot, "launcher.query.input")
		results, complete := automationdriver.Find(snapshot, "launcher.results")
		if !found || input.Value != "set volume "+percent || !complete || results.Value != "complete" {
			return false
		}
		for _, node := range snapshot.Tree.Nodes {
			if strings.HasPrefix(node.AutomationID, "launcher.result.") && strings.Contains(node.Label, percent+"%") {
				return true
			}
		}
		return false
	})
	if err != nil {
		t.Fatalf("wait for volume %s result: %v", percent, err)
	}
	smoke.AssertNoDiagnostics(t, snapshot)
}
