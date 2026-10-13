//go:build wox_ui_smoke

package perf

import (
	"context"
	"os"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	"wox/test/smokeperf"
)

// Test008PerformanceReference records show-to-visible latency and Wox process memory.
// Flow: warmed show cycles -> twenty calculator queries -> hide and wait 46s.
// Evidence: the smoke summary prints the P95 and both memory readings after the case table.
// A full suite records the same readings on the fresh process before any case, and this case skips.
func Test008PerformanceReference(t *testing.T) {
	if os.Getenv(smokeperf.CollectedEnvironment) == "1" {
		t.Skip("performance reference was recorded before the cases")
	}
	connectCtx, connectCancel := context.WithTimeout(context.Background(), automationdriver.ActionTimeout)
	client := smoke.SharedClient(t, connectCtx)
	connectCancel()

	resetCtx, resetCancel := context.WithTimeout(context.Background(), automationdriver.ActionTimeout)
	err := client.Reset(resetCtx)
	resetCancel()
	if err != nil {
		t.Fatalf("reset shared Wox before performance reference: %v", err)
	}
	t.Cleanup(func() {
		afterCtx, afterCancel := context.WithTimeout(context.Background(), automationdriver.ActionTimeout)
		defer afterCancel()
		if resetErr := client.Reset(afterCtx); resetErr != nil {
			t.Errorf("reset shared Wox after performance reference: %v", resetErr)
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), smokeperf.CollectionBudget)
	defer cancel()
	ref := smokeperf.Collect(ctx, client, os.Stderr)
	ref.When = "perf/008"
	t.Log(ref.Line())
}
