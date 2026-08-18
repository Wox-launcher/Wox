//go:build wox_ui_smoke

package perf

import (
	"context"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxwidget "wox/ui/widget"
)

// Test003Catalog500Work verifies injected 500-item plugin and theme catalogs record frame work.
// Flow: install catalog-500 -> open plugins -> measure -> open themes -> measure.
// Evidence: both catalogs expose the first fixture row and settled frames report work counters without unexpected drops.
func Test003Catalog500Work(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		if err := client.OpenSettings(ctx, "/plugins"); err != nil {
			t.Fatalf("open plugin catalog: %v", err)
		}
		if err := client.InstallPerfFixture(ctx, "catalog-500"); err != nil {
			t.Fatalf("install plugin catalog fixture: %v", err)
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			_, found := automationdriver.Find(snapshot, "plugin-list-perf-plugin-0000")
			return found
		}); err != nil {
			t.Fatalf("wait for plugin catalog fixture: %v", err)
		}
		assertSettledWork(t, waitForPresentedSamples(t, ctx, client))

		if err := client.OpenSettings(ctx, "/themes"); err != nil {
			t.Fatalf("open theme catalog: %v", err)
		}
		// openSettings("/themes") reloads the real catalog, so reinstall after the route change.
		if err := client.InstallPerfFixture(ctx, "catalog-500"); err != nil {
			t.Fatalf("install theme catalog fixture: %v", err)
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			_, found := automationdriver.Find(snapshot, "theme-list-perf-theme-0000")
			return found
		}); err != nil {
			t.Fatalf("wait for theme catalog fixture: %v", err)
		}
		assertSettledWork(t, waitForPresentedSamples(t, ctx, client))
		assertUnexpectedDroppedFramesAtMost(t, ctx, client, 0)
	})
}
