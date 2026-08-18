//go:build wox_ui_smoke

package attention

import (
	"context"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
)

// Test002LauncherAttentionUpdate verifies plugin-scoped keys deduplicate pushes and changed content reopens a handled item.
// Flow: push and repeat identical content -> mark it read -> push fresh content with the same key -> reopen Attention.
// Evidence: one database row keeps the original fingerprint for the repeat, then changes fingerprint, becomes unread, and exposes Mark as read again.
func Test002LauncherAttentionUpdate(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		prepareAttentionFixture(t)
		smoke.ShowLauncher(t, ctx, client)
		activateFixtureAction(t, ctx, client, "action-result-push-fresh-attention-")
		initial := waitForAttentionState(t, ctx, func(state attentionFixtureState) bool {
			return state.count == 1 && !state.isRead && state.fingerprint != ""
		})

		activateFixtureAction(t, ctx, client, "action-result-repeat-attention-")
		repeated := waitForAttentionState(t, ctx, func(state attentionFixtureState) bool {
			return state.count == 1 && !state.isRead && state.fingerprint == initial.fingerprint
		})
		if repeated.fingerprint != initial.fingerprint {
			t.Fatalf("repeated Attention fingerprint = %q, want %q", repeated.fingerprint, initial.fingerprint)
		}

		openAttentionItem(t, ctx, client)
		snapshot := smoke.OpenResultActionPanel(t, ctx, client)
		markRead, found := automationdriver.FindByAutomationIDPrefix(snapshot, "action-result-attention-mark-read-")
		if !found {
			t.Fatal("Attention Mark as read action was not found")
		}
		if err := client.Perform(ctx, markRead.AutomationID, woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("mark Attention item read: %v", err)
		}
		smoke.WaitForResultActionsClosed(t, ctx, client)
		waitForAttentionState(t, ctx, func(state attentionFixtureState) bool {
			return state.count == 1 && state.isRead && state.fingerprint == initial.fingerprint
		})

		activateFixtureAction(t, ctx, client, "action-result-push-fresh-attention-")
		updated := waitForAttentionState(t, ctx, func(state attentionFixtureState) bool {
			return state.count == 1 && !state.isRead && state.fingerprint != "" && state.fingerprint != initial.fingerprint
		})
		if updated.count != 1 {
			t.Fatalf("updated Attention row count = %d, want 1", updated.count)
		}

		openAttentionItem(t, ctx, client)
		snapshot = smoke.OpenResultActionPanel(t, ctx, client)
		if _, found := automationdriver.FindByAutomationIDPrefix(snapshot, "action-result-attention-mark-read-"); !found {
			t.Fatal("updated Attention item did not become unread again")
		}
		smoke.AssertNoDiagnostics(t, snapshot)
	})
}
