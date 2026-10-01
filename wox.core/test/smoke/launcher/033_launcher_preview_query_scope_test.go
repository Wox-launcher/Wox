//go:build wox_ui_smoke

package query

import (
	"context"
	"runtime"
	"strings"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// Test033LauncherPreviewQueryScope verifies plugin previews are forbidden globally and available in plugin context.
// Flow: query the same fixture globally -> press Ctrl/Cmd+P -> update its preview -> enter plugin context -> update again.
// Evidence: global results expose neither preview content nor an opener; plugin results display both initial and updated content.
func Test033LauncherPreviewQueryScope(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.ShowLauncher(t, ctx, client)
		modifier := woxui.KeyModifierControl
		if runtime.GOOS == "darwin" {
			modifier = woxui.KeyModifierMeta
		}
		for _, scenario := range []struct {
			query   string
			preview bool
		}{{"wox-preview-scope-smoke", false}, {"wox-smoke preview-scope ", true}} {
			snapshot := smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, scenario.query)
			if !hasResultTitle(snapshot, "Preview scope initial") {
				t.Fatalf("fixture result missing for %q: %s", scenario.query, automationdriver.DescribeSnapshot(snapshot))
			}
			check := func(title, body string) {
				t.Helper()
				snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
					if !hasResultTitle(snapshot, title) {
						return false
					}
					foundBody := false
					for _, node := range snapshot.Tree.Nodes {
						if strings.HasPrefix(node.AutomationID, "preview-") && node.Value == body {
							foundBody = true
						}
					}
					return !scenario.preview || foundBody
				})
				if err != nil {
					t.Fatalf("wait for %q in %q: %v", title, scenario.query, err)
				}
				smoke.AssertNoDiagnostics(t, snapshot)
				if !scenario.preview {
					for _, node := range snapshot.Tree.Nodes {
						if strings.HasPrefix(node.AutomationID, "preview-") || strings.HasPrefix(node.AutomationID, "result-preview-") {
							t.Fatalf("global query exposed preview node: %#v", node)
						}
					}
				}
			}
			check("Preview scope initial", "Preview scope initial body")
			if !scenario.preview {
				if err := client.PressKey(ctx, "p", modifier); err != nil {
					t.Fatal(err)
				}
			}
			// The changed title is a fresh-frame barrier for the negative assertion after Ctrl/Cmd+P.
			smoke.ActivateSelectedResultAction(t, ctx, client, "action-result-update-preview-scope-")
			check("Preview scope updated", "Preview scope updated body")
		}
	})
}
