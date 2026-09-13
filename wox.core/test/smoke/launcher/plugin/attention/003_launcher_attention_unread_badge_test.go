//go:build wox_ui_smoke

package attention

import (
	"context"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
)

// Test003LauncherAttentionUnreadBadge verifies an unread Attention item appears as a query-box badge that opens the inbox.
// Flow: push an unread item -> return to a global query -> activate the unread badge.
// Evidence: the badge reports a non-zero count, then the launcher query becomes "attention " and shows the fixture item.
func Test003LauncherAttentionUnreadBadge(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		prepareAttentionFixture(t)
		smoke.ShowLauncher(t, ctx, client)
		showUnreadAttentionBadge(t, ctx, client)
		if err := client.Perform(ctx, attentionUnreadBadgeID, woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("activate Attention unread badge: %v", err)
		}
		snapshot := waitForAttentionInboxOpened(t, ctx, client)
		smoke.AssertNoDiagnostics(t, snapshot)
	})
}
