//go:build wox_ui_smoke

package shell

import (
	"context"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
)

// Test002LauncherQueryShellFailureHistory covers failed execution and its visible history entry.
func Test002LauncherQueryShellFailureHistory(t *testing.T) {
	const command = "exit 7"

	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.ShowLauncher(t, ctx, client)
		if err := client.Perform(ctx, "launcher.query.input", woxui.AccessibilityActionSetValue, "> "+command); err != nil {
			t.Fatalf("enter failing Shell query: %v", err)
		}
		resultID := waitForShellResult(t, ctx, client, command)
		if err := client.Perform(ctx, resultID, woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("execute failing Shell result: %v", err)
		}
		waitForTerminalStatus(t, ctx, client, "failed")

		if err := client.Perform(ctx, "launcher.query.input", woxui.AccessibilityActionSetValue, "> "); err != nil {
			t.Fatalf("open Shell history: %v", err)
		}
		waitForShellResult(t, ctx, client, command)
	})
}
