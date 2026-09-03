//go:build wox_ui_smoke

package singlefile

import (
	"context"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
)

// Test001LauncherQuerySinglefilePython verifies a Python single-file SDK plugin can query and run its default action.
// Flow: write the plugin file -> query its trigger keyword with a trailing space -> activate the result.
// Evidence: the launcher shows the plugin title and the clipboard receives the action copy payload.
func Test001LauncherQuerySinglefilePython(t *testing.T) {
	writeSingleFilePlugin(t, "Wox.Plugin.SmokePython.py", pythonPluginSource("single-file python v1"))
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.PreserveClipboard(t)
		smoke.ShowLauncher(t, ctx, client)
		snapshot := waitForPluginResult(t, ctx, client, pythonTrigger+" ", "single-file python v1")
		result, found := resultByLabel(snapshot, "single-file python v1")
		if !found {
			t.Fatalf("python single-file results = %+v, want title %q", launcherResults(snapshot), "single-file python v1")
		}
		smoke.AssertNoDiagnostics(t, snapshot)
		if err := client.Perform(ctx, result.AutomationID, woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("activate python single-file result: %v", err)
		}
		waitForClipboardText(t, ctx, "single-file-python-copied")
	})
}
