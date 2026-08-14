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

// Test006LauncherQueryShellPythonInterpreter verifies that the Shell interpreter setting controls real execution.
// Flow: select Python in plugin settings -> reopen settings to confirm persistence -> execute a Python expression.
// Evidence: the persisted dropdown value is Python and the completed terminal output is exactly 42.
func Test006LauncherQueryShellPythonInterpreter(t *testing.T) {
	const command = "print(6 * 7)"

	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		original := selectShellInterpreter(t, ctx, client, "Python")
		defer func() {
			if original != "Python" {
				selectShellInterpreter(t, ctx, client, original)
			}
			if err := client.Hide(ctx); err != nil {
				t.Errorf("close Shell plugin settings after restoring interpreter: %v", err)
			}
		}()
		if err := client.Hide(ctx); err != nil {
			t.Fatalf("close Shell plugin settings after selecting Python: %v", err)
		}

		if current := openShellInterpreterSetting(t, ctx, client); current != "Python" {
			t.Fatalf("persisted Shell interpreter = %q, want Python", current)
		}
		if err := client.Hide(ctx); err != nil {
			t.Fatalf("close Shell plugin settings after persistence check: %v", err)
		}

		smoke.ShowLauncher(t, ctx, client)
		if err := client.Perform(ctx, "launcher.query.input", woxui.AccessibilityActionSetValue, "> "+command); err != nil {
			t.Fatalf("enter Python Shell query: %v", err)
		}
		resultID := waitForShellResult(t, ctx, client, command)
		if err := client.Perform(ctx, resultID, woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("execute Python Shell result: %v", err)
		}
		snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			status, statusFound := automationdriver.Find(snapshot, "launcher.preview.terminal.status")
			output, outputFound := automationdriver.Find(snapshot, "launcher.preview.terminal.output")
			return statusFound && status.Value == "completed" && outputFound && strings.TrimSpace(output.Value) == "42"
		})
		if err != nil {
			t.Fatalf("wait for Python terminal output: %v", err)
		}
		assertShellSnapshot(t, snapshot)
	})
}

// openShellInterpreterSetting opens the Shell plugin and returns its persisted interpreter label.
func openShellInterpreterSetting(t *testing.T, ctx context.Context, client *automationdriver.Client) string {
	t.Helper()
	openShellPluginSettings(t, ctx, client)
	var current string
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		field, found := automationdriver.Find(snapshot, "plugin-settings-field-1")
		if found {
			current = field.Value
		}
		return found && current != ""
	}); err != nil {
		t.Fatalf("wait for Shell interpreter setting: %v", err)
	}
	return current
}

// selectShellInterpreter chooses one visible interpreter option and returns the previous value.
func selectShellInterpreter(t *testing.T, ctx context.Context, client *automationdriver.Client, expected string) string {
	t.Helper()
	previous := openShellInterpreterSetting(t, ctx, client)
	if previous == expected {
		return previous
	}
	smoke.SelectSettingChoiceByLabel(t, ctx, client, "plugin-settings-field-1", expected)
	return previous
}
