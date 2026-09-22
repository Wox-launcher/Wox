//go:build wox_ui_smoke

package queryhint

import (
	"context"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
)

// Test003ReopenReplace verifies reopening selects the entire hinted query for immediate replacement.
// Flow: type a volume -> hide -> show -> type a fresh query.
// Evidence: the native selection covers command and argument, and the new text replaces both.
func Test003ReopenReplace(t *testing.T) {
	skipVolumeHintOnLinux(t)
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		enterVolumeHint(t, ctx, client)
		if err := client.EnterText(ctx, "30"); err != nil {
			t.Fatal(err)
		}
		waitVolume(t, ctx, client, "30")
		if err := client.Hide(ctx); err != nil {
			t.Fatal(err)
		}
		if _, err := client.WaitForWindowState(ctx, "primary", func(state automationdriver.WindowState) bool { return state.Exists && !state.Visible }); err != nil {
			t.Fatal(err)
		}
		smoke.ShowLauncher(t, ctx, client)
		waitInput(t, ctx, client, "set volume 30", 0, 13)
		if err := client.EnterText(ctx, "new query"); err != nil {
			t.Fatal(err)
		}
		waitInput(t, ctx, client, "new query", 9, 9)
	})
}
