//go:build wox_ui_smoke

package queryhint

import (
	"context"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
)

// Test001ContinuousInput verifies QueryHint supports continuous command and argument typing.
// Flow: type command -> type space to reveal hint -> type 30 -> delete one character.
// Evidence: the full query stays in one editor and completed results change from 30% to 3% without execution.
func Test001ContinuousInput(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		enterVolumeHint(t, ctx, client)
		if err := client.EnterText(ctx, "30"); err != nil {
			t.Fatal(err)
		}
		waitVolume(t, ctx, client, "30")
		if err := client.PressKey(ctx, woxui.KeyBackspace, 0); err != nil {
			t.Fatal(err)
		}
		waitInput(t, ctx, client, "set volume 3", 12, 12)
		waitVolume(t, ctx, client, "3")
	})
}
