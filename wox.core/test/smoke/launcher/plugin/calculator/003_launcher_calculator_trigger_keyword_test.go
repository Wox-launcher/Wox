//go:build wox_ui_smoke

package calculator

import (
	"context"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
)

// Test003LauncherCalculatorTriggerKeyword verifies a replaced trigger keyword routes calculator queries.
// Flow: replace Calculator's keywords with one private keyword -> query 1+1 -> query the private keyword with 1+1 -> restore * and calculator.
// Evidence: the bare expression no longer shows 2, the private keyword shows 2, and both restored keywords show 2 again.
func Test003LauncherCalculatorTriggerKeyword(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		keywordsDirty := false
		t.Cleanup(func() {
			if !keywordsDirty {
				return
			}
			cleanupCtx, cancel := context.WithTimeout(context.Background(), smoke.CaseTimeout)
			defer cancel()
			setCalculatorTriggerKeywords(t, cleanupCtx, client, []string{"*", "calculator"})
		})

		keywordsDirty = true
		setCalculatorTriggerKeywords(t, ctx, client, []string{smokeCalculatorKeyword})
		assertCalculatorQuery(t, ctx, client, "1+1", false)
		assertCalculatorQuery(t, ctx, client, smokeCalculatorKeyword+" 1+1", true)

		setCalculatorTriggerKeywords(t, ctx, client, []string{"*", "calculator"})
		assertCalculatorQuery(t, ctx, client, "1+1", true)
		assertCalculatorQuery(t, ctx, client, "calculator 1+1", true)
		keywordsDirty = false
	})
}
