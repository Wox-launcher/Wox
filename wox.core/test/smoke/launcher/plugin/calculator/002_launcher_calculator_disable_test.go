//go:build wox_ui_smoke

package calculator

import (
	"context"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
)

// Test002LauncherCalculatorDisable verifies disabling Calculator removes its result until it is enabled again.
// Flow: open Calculator settings -> disable -> query 1+1 -> enable -> query 1+1.
// Evidence: result 2 disappears while disabled and returns after enable.
func Test002LauncherCalculatorDisable(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		disabled := false
		t.Cleanup(func() {
			if !disabled {
				return
			}
			cleanupCtx, cancel := context.WithTimeout(context.Background(), smoke.CaseTimeout)
			defer cancel()
			setCalculatorEnabled(t, cleanupCtx, client, true)
		})

		disabled = true
		setCalculatorEnabled(t, ctx, client, false)
		assertCalculatorQuery(t, ctx, client, "1+1", false)

		setCalculatorEnabled(t, ctx, client, true)
		disabled = false
		assertCalculatorQuery(t, ctx, client, "1+1", true)
	})
}
