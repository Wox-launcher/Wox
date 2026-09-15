//go:build wox_ui_smoke

package queryhint

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

// Test005Suggestions covers command discovery, nested choices, native Tab and undo.
// Evidence: hints never enter the editable value until accepted, and free text stays editable.
func Test005Suggestions(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.ShowLauncher(t, ctx, client)
		typeText := func(text string) {
			t.Helper()
			if err := client.EnterText(ctx, text); err != nil {
				t.Fatal(err)
			}
		}
		waitHint := func(text string, tab bool) {
			t.Helper()
			snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
				hint, found := automationdriver.Find(snapshot, "launcher.query.completion")
				_, shown := automationdriver.Find(snapshot, "launcher.query.tab-hint")
				return found && strings.Contains(hint.Value, text) && shown == tab
			})
			if err != nil {
				t.Fatalf("wait for %q, Tab=%t: %v", text, tab, err)
			}
			smoke.AssertNoDiagnostics(t, snapshot)
		}
		pressTab := func() {
			t.Helper()
			if err := client.PressKey(ctx, woxui.KeyTab, 0); err != nil {
				t.Fatal(err)
			}
		}
		typeText("wox-smoke ")
		waitHint("…", false)
		typeText("query-h")
		waitHint("int", true)
		pressTab()
		waitInput(t, ctx, client, "wox-smoke query-hint ", 21, 21)
		waitHint("created / assigned / search", false)
		typeText("cr")
		waitHint("eated", true)
		pressTab()
		waitInput(t, ctx, client, "wox-smoke query-hint created", 28, 28)
		modifier := woxui.KeyModifierControl
		if runtime.GOOS == "darwin" {
			modifier = woxui.KeyModifierMeta
		}
		if err := client.PressKey(ctx, woxui.Key("z"), modifier); err != nil {
			t.Fatal(err)
		}
		waitInput(t, ctx, client, "wox-smoke query-hint cr", 23, 23)
		waitHint("eated", true)
		typeText("ustom")
		waitInput(t, ctx, client, "wox-smoke query-hint crustom", 28, 28)
	})
}
