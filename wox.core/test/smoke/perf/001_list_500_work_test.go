//go:build wox_ui_smoke

package perf

import (
	"context"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
)

// Test001List500Work verifies the 500-item list fixture records per-frame work without dropped frames.
// Flow: show launcher -> query wox-smoke list-500 -> collect settled presented frames.
// Evidence: each settled frame reports layout/paint/identity work and zero dropped frames.
func Test001List500Work(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		snapshot := runQueryFixture(t, ctx, client, fixtureCommandQuery("list-500"))
		if _, found := automationdriver.Find(snapshot, "launcher.result.perf-list-0000"); !found {
			t.Fatal("expected first list fixture result")
		}
		samples := waitForPresentedSamples(t, ctx, client)
		assertSettledWork(t, samples)
		assertNoDroppedFrames(t, ctx, client)
	})
}
