//go:build wox_ui_smoke

package perf

import (
	"context"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// Test006LocalInteractionDamage verifies hover, selection, and resize still produce completed frames.
// Flow: query a small fixture -> hover a result -> extend query selection -> resize the launcher.
// Evidence: each interaction yields presented frames with work counters and no dropped frames.
func Test006LocalInteractionDamage(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		snapshot := runQueryFixture(t, ctx, client, fixtureCommandQuery("warm-cache"))
		result, found := automationdriver.Find(snapshot, "launcher.result.perf-warm-0")
		if !found {
			t.Fatal("expected warm-cache result for local interaction")
		}

		if _, err := client.MovePointerTo(ctx, result.AutomationID); err != nil {
			t.Fatalf("hover result: %v", err)
		}
		assertSettledWork(t, waitForPresentedSamples(t, ctx, client))

		if err := client.Perform(ctx, "launcher.query.input", woxui.AccessibilityActionSetValue, "wox-smoke warm-cache selected"); err != nil {
			t.Fatalf("update query for selection: %v", err)
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			input, inputFound := automationdriver.Find(snapshot, "launcher.query.input")
			return inputFound && input.Value == "wox-smoke warm-cache selected"
		}); err != nil {
			t.Fatalf("wait for selected query text: %v", err)
		}
		assertSettledWork(t, waitForPresentedSamples(t, ctx, client))

		bounds, err := client.Bounds(ctx)
		if err != nil {
			t.Fatalf("read launcher bounds: %v", err)
		}
		bounds.Width += 48
		if err := client.SetBounds(ctx, bounds); err != nil {
			t.Fatalf("resize launcher: %v", err)
		}
		assertSettledWork(t, waitForPresentedSamples(t, ctx, client))
		assertNoDroppedFrames(t, ctx, client)
	})
}
