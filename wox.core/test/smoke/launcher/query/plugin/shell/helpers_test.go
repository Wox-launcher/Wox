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

const shellPluginID = "8a4b5c6d-7e8f-9a0b-1c2d-3e4f5a6b7c8d"

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

// selectShellResult moves launcher selection through the native keyboard path until the target is selected.
func selectShellResult(t *testing.T, ctx context.Context, client *automationdriver.Client, resultID string) woxwidget.AutomationSnapshot {
	t.Helper()
	snapshot, err := client.Snapshot(ctx)
	if err != nil {
		t.Fatalf("read Shell results before selecting %q: %v", resultID, err)
	}
	resultIDs := make([]string, 0)
	selectedIndex := -1
	targetIndex := -1
	for _, node := range snapshot.Tree.Nodes {
		if !strings.HasPrefix(node.AutomationID, "launcher.result.") {
			continue
		}
		index := len(resultIDs)
		resultIDs = append(resultIDs, node.AutomationID)
		if node.Selected {
			selectedIndex = index
		}
		if node.AutomationID == resultID {
			targetIndex = index
		}
	}
	if targetIndex < 0 || selectedIndex < 0 {
		t.Fatalf("select Shell result %q: target index %d, selected index %d", resultID, targetIndex, selectedIndex)
	}
	if targetIndex == selectedIndex {
		return snapshot
	}
	key := woxui.KeyArrowDown
	if targetIndex < selectedIndex {
		key = woxui.KeyArrowUp
	}
	for range resultIDs {
		if err := client.PressKey(ctx, key, 0); err != nil {
			t.Fatalf("navigate to Shell result %q: %v", resultID, err)
		}
		snapshot, err = client.Snapshot(ctx)
		if err != nil {
			t.Fatalf("read Shell results while selecting %q: %v", resultID, err)
		}
		target, found := automationdriver.Find(snapshot, resultID)
		if found && target.Selected {
			return snapshot
		}
	}
	t.Fatalf("Shell result %q was not selected after keyboard navigation", resultID)
	return woxwidget.AutomationSnapshot{}
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
		node, found := automationdriver.FindByAutomationIDPrefix(snapshot, actionPrefix)
		actionID = node.AutomationID
		return found
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
	smoke.AssertNoDiagnostics(t, snapshot)
}

func openShellPluginSettings(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	smoke.OpenInstalledPluginSettings(t, ctx, client, shellPluginID)
}
