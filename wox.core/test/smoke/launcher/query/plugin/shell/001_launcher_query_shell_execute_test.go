//go:build wox_ui_smoke

package shell

import (
	"context"
	"strings"
	"testing"

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
		assertShellSnapshot(t, snapshot)
		if _, found := automationdriver.Find(snapshot, "launcher.query.input"); !found {
			t.Fatal("launcher closed after foreground Shell execution")
		}
	})
}
