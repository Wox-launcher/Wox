//go:build wox_ui_smoke

package singlefile

import (
	"context"
	"os"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
)

// Test003LauncherSinglefileReload verifies saving a single-file SDK plugin reloads it before the next query.
// Flow: write v1 -> query -> overwrite the same file with v2 -> query again.
// Evidence: the second completed result generation shows the new title.
func Test003LauncherSinglefileReload(t *testing.T) {
	path := writeSingleFilePlugin(t, "Wox.Plugin.SmokePython.py", pythonPluginSource("single-file python v1"))
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.ShowLauncher(t, ctx, client)
		snapshot := waitForPluginResult(t, ctx, client, pythonTrigger, "single-file python v1")
		smoke.AssertNoDiagnostics(t, snapshot)

		if err := os.WriteFile(path, []byte(pythonPluginSource("single-file python v2")), 0644); err != nil {
			t.Fatalf("rewrite python single-file plugin: %v", err)
		}

		snapshot = waitForPluginResult(t, ctx, client, pythonTrigger, "single-file python v2")
		if _, found := resultByLabel(snapshot, "single-file python v2"); !found {
			t.Fatalf("reloaded python results = %+v, want title %q", launcherResults(snapshot), "single-file python v2")
		}
		smoke.AssertNoDiagnostics(t, snapshot)
	})
}
