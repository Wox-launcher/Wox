//go:build wox_ui_smoke

package singlefile

import (
	"context"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
)

// Test002LauncherQuerySinglefileNodejs verifies a CommonJS single-file SDK plugin can query through the Node host.
// Flow: write the plugin file -> query its trigger keyword with a trailing space.
// Evidence: the launcher shows the Node plugin title.
func Test002LauncherQuerySinglefileNodejs(t *testing.T) {
	writeSingleFilePlugin(t, "Wox.Plugin.SmokeNode.js", nodePluginSource("single-file node v1"))
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.ShowLauncher(t, ctx, client)
		snapshot := waitForPluginResult(t, ctx, client, nodeTrigger+" ", "single-file node v1")
		if _, found := resultByLabel(snapshot, "single-file node v1"); !found {
			t.Fatalf("node single-file results = %+v, want title %q", launcherResults(snapshot), "single-file node v1")
		}
		smoke.AssertNoDiagnostics(t, snapshot)
	})
}
