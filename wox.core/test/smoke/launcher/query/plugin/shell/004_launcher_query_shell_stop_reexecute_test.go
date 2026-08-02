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
	const command = "sleep 10"

	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.ShowLauncher(t, ctx, client)
		if err := client.Perform(ctx, "launcher.query.input", woxui.AccessibilityActionSetValue, "> "+command); err != nil {
			t.Fatalf("enter long-running Shell query: %v", err)
		}
		resultID := waitForShellResult(t, ctx, client, command)
		if err := client.Perform(ctx, resultID, woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("execute long-running Shell result: %v", err)
		}
		waitForTerminalStatus(t, ctx, client, "running")
		if err := client.Perform(ctx, resultID, woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("stop long-running Shell result: %v", err)
		}
		waitForTerminalStatus(t, ctx, client, "killed")

		if err := client.Perform(ctx, "launcher.query.input", woxui.AccessibilityActionSetValue, "> "); err != nil {
			t.Fatalf("open Shell history: %v", err)
		}
		historyResultID := waitForShellResult(t, ctx, client, command)
		if err := client.Perform(ctx, historyResultID, woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("reexecute Shell history result: %v", err)
		}
		waitForTerminalStatus(t, ctx, client, "running")
		if err := client.Perform(ctx, historyResultID, woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("stop reexecuted Shell result: %v", err)
		}
		waitForTerminalStatus(t, ctx, client, "killed")
	})
}
