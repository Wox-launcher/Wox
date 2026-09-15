//go:build wox_ui_smoke

package queryhint

import (
	"context"
	"testing"
	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// Test006IndicatorEntry checks Indicator entry for both opted-out and ordinary plugins.
func Test006IndicatorEntry(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		for _, entry := range []struct {
			keyword string
			hint    bool
		}{{"cb", false}, {"wox-smoke", true}} {
			smoke.ShowLauncher(t, ctx, client)
			if err := client.EnterText(ctx, entry.keyword); err != nil {
				t.Fatal(err)
			}
			if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
				_, found := smoke.FindLauncherResult(snapshot, entry.keyword)
				return found
			}); err != nil {
				t.Fatal(err)
			}
			if err := client.PressKey(ctx, woxui.KeyEnter, 0); err != nil {
				t.Fatal(err)
			}
			text := entry.keyword + " "
			waitInput(t, ctx, client, text, len(text), len(text))
			snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
				_, found := automationdriver.Find(snapshot, "launcher.query.completion")
				return found == entry.hint
			})
			if err != nil {
				t.Fatalf("Indicator entry %q: expected hint=%t: %v", entry.keyword, entry.hint, err)
			}
			smoke.AssertNoDiagnostics(t, snapshot)
		}
	})
}
