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

// Test004TabHintVisibility verifies the Tab hint advertises an available argument target.
// Flow: enter the last argument -> return to the command -> Tab into the argument -> Tab again.
// Evidence: the hint follows the available target, Tab selects the argument, and another Tab preserves it.
func Test004TabHintVisibility(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		enterVolumeHint(t, ctx, client)
		waitTabHint := func(visible bool) {
			t.Helper()
			snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
				_, shown := automationdriver.Find(snapshot, "launcher.query.tab-hint")
				return shown == visible
			})
			if err != nil {
				t.Fatalf("wait for Tab hint visible=%t: %v", visible, err)
			}
			smoke.AssertNoDiagnostics(t, snapshot)
		}
		waitTabHint(false)
		if err := client.EnterText(ctx, "30"); err != nil {
			t.Fatal(err)
		}
		waitVolume(t, ctx, client, "30")
		waitTabHint(false)
		if err := client.PressKey(ctx, woxui.KeyHome, 0); err != nil {
			t.Fatal(err)
		}
		waitInput(t, ctx, client, "set volume 30", 0, 0)
		waitTabHint(true)
		for i := 0; i < 2; i++ {
			if err := client.PressKey(ctx, woxui.KeyTab, 0); err != nil {
				t.Fatal(err)
			}
			waitInput(t, ctx, client, "set volume 30", 11, 13)
			waitTabHint(false)
		}
	})
}
