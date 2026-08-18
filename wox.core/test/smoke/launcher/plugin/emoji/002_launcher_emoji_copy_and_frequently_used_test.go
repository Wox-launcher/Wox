//go:build wox_ui_smoke

package emoji

import (
	"context"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// Test002LauncherEmojiCopyAndFrequentlyUsed verifies text copying and the frequently-used lifecycle.
// Flow: copy the robot emoji -> query it again -> remove it from frequently used -> reopen its actions.
// Evidence: the clipboard contains the glyph, the removal action appears after copying, then disappears after cleanup.
func Test002LauncherEmojiCopyAndFrequentlyUsed(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.PreserveClipboard(t)
		smoke.ShowLauncher(t, ctx, client)

		snapshot := smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, "emoji 🤖")
		results := emojiResults(snapshot)
		if len(results) != 1 {
			t.Fatalf("robot results = %+v, want one result", results)
		}
		if err := client.Perform(ctx, results[0].AutomationID, woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("copy robot emoji: %v", err)
		}
		waitForClipboardText(t, ctx, "🤖")

		smoke.ShowLauncher(t, ctx, client)
		snapshot = smoke.ReplaceLauncherQuery(t, ctx, client, "emoji 🤖")
		results = emojiResults(snapshot)
		if len(results) != 1 {
			t.Fatalf("frequently-used robot results = %+v, want one result", results)
		}
		previousResultID := results[0].AutomationID
		snapshot = smoke.OpenResultActionPanel(t, ctx, client)
		removeAction, found := emojiActionByLabel(snapshot,
			"Remove from frequently used", "从常用中移除", "Remover dos frequentes", "Удалить из часто используемых")
		if !found {
			t.Fatal("remove-from-frequently-used action was not exposed after copying")
		}
		if err := client.Perform(ctx, removeAction.AutomationID, woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("remove robot from frequently used: %v", err)
		}

		snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			currentResults := emojiResults(snapshot)
			resultsNode, resultsFound := automationdriver.Find(snapshot, "launcher.results")
			_, panelOpen := automationdriver.Find(snapshot, "action-search")
			return resultsFound && resultsNode.Value == "complete" && len(currentResults) == 1 &&
				currentResults[0].AutomationID != previousResultID && !panelOpen
		})
		if err != nil {
			t.Fatalf("wait for robot removal refresh: %v", err)
		}
		snapshot = smoke.OpenResultActionPanel(t, ctx, client)
		if _, found := emojiActionByLabel(snapshot,
			"Remove from frequently used", "从常用中移除", "Remover dos frequentes", "Удалить из часто используемых"); found {
			t.Fatalf("remove-from-frequently-used action remained after cleanup: %+v", emojiActions(snapshot))
		}
		smoke.AssertNoDiagnostics(t, snapshot)
	})
}
