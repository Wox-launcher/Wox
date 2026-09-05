//go:build wox_ui_smoke

package perf

import (
	"context"
	"testing"
	"time"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxwidget "wox/ui/widget"
)

// Test004ChatStreamWork verifies active streaming can coalesce obsolete frames and then settle cleanly.
// Flow: query wox-smoke chat-200 -> measure active updates -> wait for updates to become quiet -> measure steady frames.
// Evidence: pressure drops are only coalesced frames, and the completed stream satisfies steady budgets.
func Test004ChatStreamWork(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		snapshot := runQueryFixture(t, ctx, client, fixtureCommandQuery("chat-200"))
		_, chatVisible := automationdriver.Find(snapshot, "chat.messages")
		_, resultVisible := automationdriver.Find(snapshot, "launcher.result.perf-chat-result")
		if !chatVisible && !resultVisible {
			t.Fatal("expected chat messages or streamed chat result")
		}
		streamAction, found := automationdriver.FindByAutomationIDPrefix(snapshot, "chat-send-")
		if !found {
			t.Fatal("streaming chat control was not exposed")
		}
		streamSamples := collectPresentedSamples(t, ctx, client)
		assertFrameWork(t, streamSamples)
		assertUnexpectedDroppedFramesAtMost(t, ctx, client, 0)

		// Title-only updates are hidden in chat mode. Wait for the fixture's final
		// preview to change Stop back to Send before applying steady timing budgets.
		if _, err := client.WaitFor(ctx, func(current woxwidget.AutomationSnapshot) bool {
			action, found := automationdriver.FindByAutomationIDPrefix(current, "chat-send-")
			return found && action.Label != streamAction.Label
		}); err != nil {
			t.Fatalf("wait for chat stream completion: %v", err)
		}
		waitForSnapshotQuiet(t, ctx, client, 350*time.Millisecond)
		steadySamples := waitForPresentedSamples(t, ctx, client)
		assertSettledWork(t, steadySamples)
		assertUnexpectedDroppedFramesAtMost(t, ctx, client, 0)
	})
}
