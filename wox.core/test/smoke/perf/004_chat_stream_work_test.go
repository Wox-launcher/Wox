//go:build wox_ui_smoke

package perf

import (
	"context"
	"testing"
	"time"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// Test004ChatStreamWork verifies active streaming can coalesce obsolete frames and then settle cleanly.
// Flow: query wox-smoke chat-200 -> append answer text -> settle -> scroll historical messages.
// Evidence: streaming and scrolling keep bounded work, and the completed stream satisfies steady budgets.
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
		if chat, ok := automationdriver.Find(snapshot, "chat.messages"); ok {
			// Scroll back while the long reasoning row changes, not only after completion.
			for range 8 {
				if err := client.Pointer(ctx, woxui.PointerEvent{
					Kind: woxui.PointerScroll, Scroll: woxui.Point{Y: 80},
					Position: woxui.Point{X: chat.Bounds.X + chat.Bounds.Width/2, Y: chat.Bounds.Y + chat.Bounds.Height/2},
				}); err != nil {
					t.Fatalf("scroll during reasoning stream: %v", err)
				}
				time.Sleep(120 * time.Millisecond)
			}
		}
		streamSamples := collectPresentedSamples(t, ctx, client)
		assertFrameWork(t, streamSamples)
		assertUnexpectedDroppedFramesAtMost(t, ctx, client, 0)

		// Wait for Stop to change back to Send before applying steady timing budgets.
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

		snapshot, err := client.Snapshot(ctx)
		if err != nil {
			t.Fatal(err)
		}
		chat, found := automationdriver.Find(snapshot, "chat.messages")
		if !found {
			t.Fatal("completed conversation lost its scroll viewport")
		}
		for _, delta := range []float32{-600, 600} {
			if err := client.Pointer(ctx, woxui.PointerEvent{
				Kind: woxui.PointerScroll, Scroll: woxui.Point{Y: delta},
				Position: woxui.Point{X: chat.Bounds.X + chat.Bounds.Width/2, Y: chat.Bounds.Y + chat.Bounds.Height/2},
			}); err != nil {
				t.Fatalf("scroll historical messages: %v", err)
			}
			assertSettledWork(t, waitForPresentedSamples(t, ctx, client))
		}
	})
}
