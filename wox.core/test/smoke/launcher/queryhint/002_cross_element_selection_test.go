//go:build wox_ui_smoke

package queryhint

import (
	"context"
	"runtime"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
)

// Test002CrossElementSelection verifies selection crosses hint boundaries and undo restores semantics.
// Flow: type a volume -> select backwards across argument and command -> replace -> undo -> Tab.
// Evidence: replacement preserves exact text; undo restores the result and Tab selects only the argument range.
func Test002CrossElementSelection(t *testing.T) {
	skipVolumeHintOnLinux(t)
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		enterVolumeHint(t, ctx, client)
		if err := client.EnterText(ctx, "30"); err != nil {
			t.Fatal(err)
		}
		waitVolume(t, ctx, client, "30")
		for i := 0; i < 9; i++ {
			if err := client.PressKey(ctx, woxui.KeyArrowLeft, woxui.KeyModifierShift); err != nil {
				t.Fatal(err)
			}
		}
		waitInput(t, ctx, client, "set volume 30", 4, 13)
		if err := client.EnterText(ctx, "other"); err != nil {
			t.Fatal(err)
		}
		waitInput(t, ctx, client, "set other", 9, 9)
		modifier := woxui.KeyModifierControl
		if runtime.GOOS == "darwin" {
			modifier = woxui.KeyModifierMeta
		}
		if err := client.PressKey(ctx, woxui.Key("z"), modifier); err != nil {
			t.Fatal(err)
		}
		waitVolume(t, ctx, client, "30")
		if err := client.PressKey(ctx, woxui.KeyHome, 0); err != nil {
			t.Fatal(err)
		}
		if err := client.PressKey(ctx, woxui.KeyTab, 0); err != nil {
			t.Fatal(err)
		}
		waitInput(t, ctx, client, "set volume 30", 11, 13)
	})
}
