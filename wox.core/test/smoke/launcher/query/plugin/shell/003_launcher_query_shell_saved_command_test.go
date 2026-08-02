//go:build wox_ui_smoke

package shell

import (
	"context"
	"strings"
	"testing"
	"time"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// Test003LauncherQueryShellSavedCommand covers saving, expanding, executing, and deleting a configured command.
func Test003LauncherQueryShellSavedCommand(t *testing.T) {
	const (
		alias           = "woxsmokesaved"
		commandTemplate = "echo {query}"
		argument        = "wox-shell-saved-output"
	)

	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.ShowLauncher(t, ctx, client)
		if err := client.Perform(ctx, "launcher.query.input", woxui.AccessibilityActionSetValue, "> "+commandTemplate); err != nil {
			t.Fatalf("enter Shell command template: %v", err)
		}
		templateResultID := waitForShellResult(t, ctx, client, commandTemplate)
		activateShellAction(t, ctx, client, "action-result-add_as_command-")

		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			title, titleFound := automationdriver.Find(snapshot, "action-form-field-0")
			command, commandFound := automationdriver.Find(snapshot, "action-form-field-2")
			_, saveFound := automationdriver.Find(snapshot, "form-save")
			return titleFound && commandFound && saveFound && title.Value != "" && command.Value == commandTemplate
		}); err != nil {
			t.Fatalf("wait for add command form: %v", err)
		}
		if err := client.Perform(ctx, "action-form-field-0", woxui.AccessibilityActionSetValue, alias); err != nil {
			t.Fatalf("set saved command alias: %v", err)
		}
		if err := client.Perform(ctx, "form-save", woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("save Shell command: %v", err)
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			refreshedResultID, found := shellResult(snapshot, commandTemplate)
			_, formOpen := automationdriver.Find(snapshot, "form-save")
			results, resultsFound := automationdriver.Find(snapshot, "launcher.results")
			return found && refreshedResultID != templateResultID && !formOpen && resultsFound && results.Value == "complete"
		}); err != nil {
			t.Fatalf("wait for saved command query refresh: %v", err)
		}

		if err := client.Perform(ctx, "launcher.query.input", woxui.AccessibilityActionSetValue, alias+" "+argument); err != nil {
			t.Fatalf("query saved Shell command: %v", err)
		}
		resultID := waitForShellResult(t, ctx, client, alias)
		if err := client.Perform(ctx, resultID, woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("execute saved Shell command: %v", err)
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			result, found := automationdriver.Find(snapshot, resultID)
			if !found {
				return false
			}
			_, parseErr := time.Parse("2006-01-02 15:04:05", result.Description)
			return parseErr == nil
		}); err != nil {
			t.Fatalf("wait for saved Shell command completion: %v", err)
		}

		activateShellAction(t, ctx, client, "action-result-delete_command-")
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			_, found := shellResult(snapshot, alias)
			return !found
		}); err != nil {
			t.Fatalf("wait for saved Shell command deletion: %v", err)
		}

		if err := client.Perform(ctx, "launcher.query.input", woxui.AccessibilityActionSetValue, "> "); err != nil {
			t.Fatalf("open Shell history: %v", err)
		}
		historyResultID := waitForShellResult(t, ctx, client, alias)
		historyNode, err := client.MovePointerTo(ctx, historyResultID)
		if err != nil {
			t.Fatalf("move pointer to saved Shell command history: %v", err)
		}
		historyPosition := woxui.Point{X: historyNode.Bounds.X + historyNode.Bounds.Width/2, Y: historyNode.Bounds.Y + historyNode.Bounds.Height/2}
		if err := client.Pointer(ctx, woxui.PointerEvent{Kind: woxui.PointerDown, Position: historyPosition, Button: woxui.PointerButtonPrimary}); err != nil {
			t.Fatalf("press saved Shell command history: %v", err)
		}
		if err := client.Pointer(ctx, woxui.PointerEvent{Kind: woxui.PointerUp, Position: historyPosition, Button: woxui.PointerButtonPrimary}); err != nil {
			t.Fatalf("select saved Shell command history: %v", err)
		}
		snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			results, resultsFound := automationdriver.Find(snapshot, "launcher.results")
			status, statusFound := automationdriver.Find(snapshot, "launcher.preview.terminal.status")
			output, outputFound := automationdriver.Find(snapshot, "launcher.preview.terminal.output")
			return resultsFound && results.Value == "complete" && statusFound && status.Value == "completed" && outputFound && strings.Contains(output.Value, argument)
		})
		if err != nil {
			t.Fatalf("wait for saved Shell command history output: %v", err)
		}
		assertShellSnapshot(t, snapshot)
	})
}
