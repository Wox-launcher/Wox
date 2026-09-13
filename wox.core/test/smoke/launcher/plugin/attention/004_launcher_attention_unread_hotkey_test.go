//go:build wox_ui_smoke

package attention

import (
	"context"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
)

// Test004LauncherAttentionUnreadHotkey verifies primary+U opens Attention only while the unread badge is visible.
// Flow: show the unread badge -> press primary+U -> stay on the Attention query and press primary+U again.
// Evidence: the first press opens the inbox and hides the badge; the second press is not consumed and the query stays on Attention.
func Test004LauncherAttentionUnreadHotkey(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		prepareAttentionFixture(t)
		smoke.ShowLauncher(t, ctx, client)
		showUnreadAttentionBadge(t, ctx, client)

		handled, err := client.PressKeyHandled(ctx, woxui.Key("u"), primaryModifier())
		if err != nil {
			t.Fatalf("press primary+U with unread badge: %v", err)
		}
		if !handled {
			t.Fatal("primary+U was not handled while the unread badge was visible")
		}
		waitForAttentionInboxOpened(t, ctx, client)
		waitForAttentionUnreadBadgeHidden(t, ctx, client)

		handled, err = client.PressKeyHandled(ctx, woxui.Key("u"), primaryModifier())
		if err != nil {
			t.Fatalf("press primary+U without unread badge: %v", err)
		}
		if handled {
			t.Fatal("primary+U was consumed after the unread badge hid")
		}
		snapshot, err := client.Snapshot(ctx)
		if err != nil {
			t.Fatalf("read launcher after unused primary+U: %v", err)
		}
		input, found := automationdriver.Find(snapshot, "launcher.query.input")
		if !found || input.Value != "attention " {
			t.Fatalf("query after unused primary+U = %q, want attention inbox", input.Value)
		}
		smoke.AssertNoDiagnostics(t, snapshot)
	})
}
