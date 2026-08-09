//go:build wox_ui_smoke

package shell

import (
	"context"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxwidget "wox/ui/widget"
)

const shellPluginID = "8a4b5c6d-7e8f-9a0b-1c2d-3e4f5a6b7c8d"

func shellResult(snapshot woxwidget.AutomationSnapshot, command string) (string, bool) {
	return smoke.FindLauncherResult(snapshot, command)
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
	return smoke.SelectLauncherResult(t, ctx, client, resultID)
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
	smoke.ActivateSelectedResultAction(t, ctx, client, actionPrefix)
}

func assertShellSnapshot(t *testing.T, snapshot woxwidget.AutomationSnapshot) {
	t.Helper()
	smoke.AssertNoDiagnostics(t, snapshot)
}

func openShellPluginSettings(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	smoke.OpenInstalledPluginSettings(t, ctx, client, shellPluginID)
}
