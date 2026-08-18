//go:build wox_ui_smoke

package attention

import (
	"context"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// Test001LauncherAttentionOpen verifies a plugin-pushed item remains actionable through the persistent Attention inbox.
// Flow: push an unread item -> query Attention -> open the item -> follow its configured query.
// Evidence: the item exposes both unread actions, becomes read in the shared database, and the launcher shows the target calculator result.
func Test001LauncherAttentionOpen(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		prepareAttentionFixture(t)
		smoke.ShowLauncher(t, ctx, client)
		activateFixtureAction(t, ctx, client, "action-result-push-fresh-attention-")
		waitForAttentionState(t, ctx, func(state attentionFixtureState) bool {
			return state.count == 1 && !state.isRead && state.fingerprint != ""
		})

		snapshot := openAttentionItem(t, ctx, client)
		snapshot = smoke.OpenResultActionPanel(t, ctx, client)
		openAction, openFound := automationdriver.FindByAutomationIDPrefix(snapshot, "action-result-attention-open-")
		_, markReadFound := automationdriver.FindByAutomationIDPrefix(snapshot, "action-result-attention-mark-read-")
		if !openFound || !markReadFound {
			t.Fatalf("Attention unread actions = open %v mark-read %v, want both", openFound, markReadFound)
		}
		if err := client.Perform(ctx, openAction.AutomationID, woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("open Attention item: %v", err)
		}

		snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			input, inputFound := automationdriver.Find(snapshot, "launcher.query.input")
			results, resultsFound := automationdriver.Find(snapshot, "launcher.results")
			return inputFound && input.Value == "1+1" && resultsFound && results.Value == "complete" && smoke.HasLauncherResultLabel(snapshot, "2")
		})
		if err != nil {
			t.Fatalf("wait for Attention target query: %v", err)
		}
		waitForAttentionState(t, ctx, func(state attentionFixtureState) bool {
			return state.count == 1 && state.isRead
		})
		smoke.AssertNoDiagnostics(t, snapshot)
	})
}
