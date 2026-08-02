//go:build wox_ui_smoke

package shell

import (
	"context"
	"runtime"
	"strings"
	"testing"
	"time"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// Test001LauncherQueryShellExecute covers foreground Shell execution and terminal output.
func Test001LauncherQueryShellExecute(t *testing.T) {
	const command = "echo wox-shell-smoke-output"

	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.ShowLauncher(t, ctx, client)
		if err := client.Perform(ctx, "launcher.query.input", woxui.AccessibilityActionSetValue, "> "+command); err != nil {
			t.Fatalf("enter Shell query: %v", err)
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			_, found := shellResult(snapshot, command)
			return found
		}); err != nil {
			t.Fatalf("wait for Shell result: %v", err)
		}
		if err := client.PressKey(ctx, woxui.KeyEnter, 0); err != nil {
			t.Fatalf("execute Shell result: %v", err)
		}

		snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			status, statusFound := automationdriver.Find(snapshot, "launcher.preview.terminal.status")
			output, outputFound := automationdriver.Find(snapshot, "launcher.preview.terminal.output")
			return statusFound && status.Value == "completed" && outputFound && strings.Contains(output.Value, "wox-shell-smoke-output")
		})
		if err != nil {
			t.Fatalf("wait for completed Shell output: %v", err)
		}
		if len(snapshot.Diagnostics) > 0 {
			t.Fatalf("Shell terminal semantics diagnostics: %v", snapshot.Diagnostics)
		}
		if _, found := automationdriver.Find(snapshot, "launcher.query.input"); !found {
			t.Fatal("launcher closed after foreground Shell execution")
		}
	})
}

func shellResult(snapshot woxwidget.AutomationSnapshot, command string) (string, bool) {
	for _, node := range snapshot.Tree.Nodes {
		if strings.HasPrefix(node.AutomationID, "launcher.result.") && strings.TrimSpace(node.Label) == command {
			return node.AutomationID, true
		}
	}
	return "", false
}

func waitForShellResult(t *testing.T, ctx context.Context, client *automationdriver.Client, title string) string {
	t.Helper()
	var resultID string
	snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		resultID, _ = shellResult(snapshot, title)
		results, resultsFound := automationdriver.Find(snapshot, "launcher.results")
		return resultID != "" && resultsFound && results.Value == "complete"
	})
	if err != nil {
		t.Fatalf("wait for Shell result %q: %v", title, err)
	}
	assertShellSnapshot(t, snapshot)
	return resultID
}

func waitForTerminalStatus(t *testing.T, ctx context.Context, client *automationdriver.Client, status string) woxwidget.AutomationSnapshot {
	t.Helper()
	snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		node, found := automationdriver.Find(snapshot, "launcher.preview.terminal.status")
		return found && node.Value == status
	})
	if err != nil {
		t.Fatalf("wait for terminal status %q: %v", status, err)
	}
	assertShellSnapshot(t, snapshot)
	return snapshot
}

func activateShellAction(t *testing.T, ctx context.Context, client *automationdriver.Client, actionPrefix string) {
	t.Helper()
	modifier := woxui.KeyModifierControl
	if runtime.GOOS == "darwin" {
		modifier = woxui.KeyModifierMeta
	}
	if err := client.PressKey(ctx, woxui.Key("j"), modifier); err != nil {
		t.Fatalf("open Shell result actions: %v", err)
	}
	waitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var actionID string
	snapshot, err := client.WaitFor(waitCtx, func(snapshot woxwidget.AutomationSnapshot) bool {
		for _, node := range snapshot.Tree.Nodes {
			if strings.HasPrefix(node.AutomationID, actionPrefix) {
				actionID = node.AutomationID
				return true
			}
		}
		return false
	})
	if err != nil {
		current, snapshotErr := client.Snapshot(ctx)
		if snapshotErr != nil {
			t.Fatalf("wait for Shell action %q: %v; snapshot: %v", actionPrefix, err, snapshotErr)
		}
		ids := make([]string, 0)
		for _, node := range current.Tree.Nodes {
			if node.AutomationID != "" {
				ids = append(ids, node.AutomationID)
			}
		}
		t.Fatalf("wait for Shell action %q: %v; automation IDs: %v", actionPrefix, err, ids)
	}
	assertShellSnapshot(t, snapshot)
	if err := client.Perform(ctx, actionID, woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("activate Shell action %q: %v", actionPrefix, err)
	}
}

func assertShellSnapshot(t *testing.T, snapshot woxwidget.AutomationSnapshot) {
	t.Helper()
	if len(snapshot.Diagnostics) > 0 {
		t.Fatalf("Shell semantics diagnostics: %v", snapshot.Diagnostics)
	}
}
