//go:build wox_ui_smoke

package perf

import (
	"context"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
)

// Test002Grid500Work verifies the 500-item grouped grid fixture records per-frame work.
// Flow: show launcher -> query wox-smoke grid-500 -> collect settled presented frames.
// Evidence: the first grid item is visible and settled frames report work counters without unexpected drops.
func Test002Grid500Work(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		snapshot := runQueryFixture(t, ctx, client, fixtureCommandQuery("grid-500"))
		if _, found := automationdriver.Find(snapshot, "launcher.result.perf-grid-0000"); !found {
			t.Fatal("expected first grid fixture result")
		}
		samples := waitForPresentedSamples(t, ctx, client)
		assertSettledWork(t, samples)
		assertUnexpectedDroppedFramesAtMost(t, ctx, client, 0)
	})
}
