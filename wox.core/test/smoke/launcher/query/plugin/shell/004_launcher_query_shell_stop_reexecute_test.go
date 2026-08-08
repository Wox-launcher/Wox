//go:build wox_ui_smoke

package shell

import (
	"context"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
)

// Test004LauncherQueryShellStopReexecute covers stopping a command and rerunning it from history.
func Test004LauncherQueryShellStopReexecute(t *testing.T) {
	const command = "sleep 2"

	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.ShowLauncher(t, ctx, client)
		smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, "> "+command)
		resultID := waitForShellResult(t, ctx, client, command)
		if err := client.Perform(ctx, resultID, woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("execute long-running Shell result: %v", err)
		}
		waitForTerminalStatus(t, ctx, client, "running")
		if err := client.Perform(ctx, resultID, woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("stop long-running Shell result: %v", err)
		}
		waitForTerminalStatus(t, ctx, client, "killed")

		smoke.ReplaceLauncherQuery(t, ctx, client, "> ")
		historyResultID := waitForShellResult(t, ctx, client, command)
		if err := client.Perform(ctx, historyResultID, woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("reexecute Shell history result: %v", err)
		}
		waitForTerminalStatus(t, ctx, client, "running")
		runningResultID := waitForShellResult(t, ctx, client, command)
		if err := client.Perform(ctx, runningResultID, woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("stop reexecuted Shell result: %v", err)
		}
		waitForTerminalStatus(t, ctx, client, "killed")
	})
}
