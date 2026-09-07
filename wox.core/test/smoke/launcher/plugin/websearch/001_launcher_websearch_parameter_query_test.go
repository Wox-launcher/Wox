//go:build wox_ui_smoke

package websearch

import (
	"context"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
)

// Test001LauncherWebsearchParameterQuery verifies the default Google parameter becomes a query slot.
// Flow: type g -> type space to reveal the query hint -> type a search value.
// Evidence: the launcher keeps one editor and shows a result titled with that value.
func Test001LauncherWebsearchParameterQuery(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		typeWebSearchQuery(t, ctx, client, "g")
		waitForWebSearchHint(t, ctx, client, "g ", "query")
		if err := client.EnterText(ctx, "woxsmoke"); err != nil {
			t.Fatal(err)
		}
		waitForWebSearchResult(t, ctx, client, "g woxsmoke", "Search Google for woxsmoke")
	})
}
