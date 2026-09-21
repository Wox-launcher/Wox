//go:build wox_ui_smoke

package shell

import (
	"context"
	"strings"
	"testing"
	"unicode/utf8"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util/clipboard"
)

const terminalCopySample = "wox-shell-copy-output"

// Test010LauncherQueryShellTerminalCopy verifies terminal output can be selected and copied.
// Flow: execute a command -> Select All on the terminal output -> Copy.
// Evidence: the output reports a full text selection and the system clipboard contains the command output.
func Test010LauncherQueryShellTerminalCopy(t *testing.T) {
	command := "echo " + terminalCopySample

	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.PreserveClipboard(t)
		smoke.ShowLauncher(t, ctx, client)
		if err := client.Perform(ctx, "launcher.query.input", woxui.AccessibilityActionSetValue, "> "+command); err != nil {
			t.Fatalf("enter Shell query: %v", err)
		}
		resultID := waitForShellResult(t, ctx, client, command)
		if err := client.Perform(ctx, resultID, woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("execute Shell result: %v", err)
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			status, statusFound := automationdriver.Find(snapshot, "launcher.preview.terminal.status")
			output, outputFound := automationdriver.Find(snapshot, "launcher.preview.terminal.output")
			return statusFound && status.Value == "completed" && outputFound && strings.Contains(output.Value, terminalCopySample)
		}); err != nil {
			t.Fatalf("wait for completed Shell output: %v", err)
		}

		if err := client.Perform(ctx, "launcher.preview.terminal.output", woxui.AccessibilityActionSelectAll, ""); err != nil {
			t.Fatalf("select all terminal output: %v", err)
		}
		snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			output, found := automationdriver.Find(snapshot, "launcher.preview.terminal.output")
			return found && output.HasTextSelection && output.SelectionStart == 0 && output.SelectionEnd == utf8.RuneCountInString(output.Value)
		})
		if err != nil {
			t.Fatalf("wait for full terminal selection: %v", err)
		}
		output, _ := automationdriver.Find(snapshot, "launcher.preview.terminal.output")
		if err := client.Perform(ctx, "launcher.preview.terminal.output", woxui.AccessibilityActionCopy, ""); err != nil {
			t.Fatalf("copy terminal output: %v", err)
		}
		text, err := clipboard.ReadText()
		if err != nil || !strings.Contains(text, terminalCopySample) {
			t.Fatalf("clipboard after terminal copy = %q err %v, want %q from %q", text, err, terminalCopySample, output.Value)
		}
		assertShellSnapshot(t, snapshot)
	})
}
