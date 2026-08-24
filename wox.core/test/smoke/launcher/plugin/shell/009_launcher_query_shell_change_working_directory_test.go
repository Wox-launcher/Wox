//go:build wox_ui_smoke

package shell

import (
	"context"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// Test009LauncherQueryShellChangeWorkingDirectory verifies the change-directory action binds the next execution.
// Flow: query a Shell command -> change working directory in the action form -> save -> execute.
// Evidence: the preview tag updates to the chosen path and the marker file is written there.
func Test009LauncherQueryShellChangeWorkingDirectory(t *testing.T) {
	const marker = "wox-cwd-action-smoke.txt"
	command := shellWriteCwdMarkerCommand(marker)

	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		workingDirectory := t.TempDir()

		smoke.ShowLauncher(t, ctx, client)
		smoke.ReplaceLauncherQuery(t, ctx, client, "> "+command)
		selectShellResult(t, ctx, client, waitForShellResult(t, ctx, client, command))
		activateShellAction(t, ctx, client, shellChangeWorkingDirectoryAction)

		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			_, fieldFound := automationdriver.Find(snapshot, "action-form-field-0")
			_, saveFound := automationdriver.Find(snapshot, "form-save")
			return fieldFound && saveFound
		}); err != nil {
			t.Fatalf("wait for change working directory form: %v", err)
		}
		if err := client.Perform(ctx, "action-form-field-0", woxui.AccessibilityActionSetValue, workingDirectory); err != nil {
			t.Fatalf("set action working directory: %v", err)
		}
		if err := client.Perform(ctx, "form-save", woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("save action working directory: %v", err)
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			_, formOpen := automationdriver.Find(snapshot, "form-save")
			results, resultsFound := automationdriver.Find(snapshot, "launcher.results")
			return !formOpen && resultsFound && results.Value == "complete"
		}); err != nil {
			t.Fatalf("wait for change working directory save: %v", err)
		}

		waitForShellPreviewWorkingDirectory(t, ctx, client, workingDirectory)
		executeShellCwdMarker(t, ctx, client, command, workingDirectory, marker)
	})
}
