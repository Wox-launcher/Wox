//go:build wox_ui_smoke

package fileexplorersearch

import (
	"context"
	"math"
	"testing"
	"time"

	"wox/common"
	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const explorerInstance = string(common.ShowSourceExplorer)

// Test001LauncherQueryFileExplorerSearchBottomAnchor verifies the File Explorer
// Search secondary keeps a fixed bottom edge while rapid queries resize it.
// Flow: open explorer secondary -> capture bottom edge -> type successive queries that
// change result height -> reopen the same explorer instance with a new query.
// Evidence: native window bottom (Y+Height) stays within 1px across resizes and re-shows.
func Test001LauncherQueryFileExplorerSearchBottomAnchor(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		if err := client.OpenExplorerQuery(ctx, "a"); err != nil {
			t.Fatalf("open explorer query: %v", err)
		}
		if _, err := client.WaitForWindowState(ctx, explorerInstance, func(state automationdriver.WindowState) bool {
			return state.Exists && state.Visible
		}); err != nil {
			t.Fatalf("wait for explorer window: %v", err)
		}
		if err := client.FocusInstance(ctx, explorerInstance); err != nil {
			t.Fatalf("focus explorer instance: %v", err)
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			input, inputFound := automationdriver.Find(snapshot, "launcher.query.input")
			scopeIcons, scopeFound := automationdriver.Find(snapshot, "launcher.query.scope-icons")
			return inputFound && input.Value == "a" && scopeFound && scopeIcons.Value == "1"
		}); err != nil {
			t.Fatalf("wait for explorer query input: %v", err)
		}

		initialBounds, err := client.Bounds(ctx)
		if err != nil {
			t.Fatalf("read initial explorer bounds: %v", err)
		}
		initialBottom := initialBounds.Y + initialBounds.Height

		for _, query := range []string{"ab", "abc", "1+1", "a", "smoke"} {
			if err := client.Perform(ctx, "launcher.query.input", woxui.AccessibilityActionSetValue, query); err != nil {
				t.Fatalf("enter explorer query %q: %v", query, err)
			}
			if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
				input, inputFound := automationdriver.Find(snapshot, "launcher.query.input")
				scopeIcons, scopeFound := automationdriver.Find(snapshot, "launcher.query.scope-icons")
				results, resultsFound := automationdriver.Find(snapshot, "launcher.results")
				return inputFound && input.Value == query && scopeFound && scopeIcons.Value == "1" && (!resultsFound || results.Value == "complete")
			}); err != nil {
				t.Fatalf("wait for explorer query %q: %v", query, err)
			}
			assertBottomEdgeStable(t, ctx, client, initialBottom)
		}

		if err := client.OpenExplorerQuery(ctx, "reanchor"); err != nil {
			t.Fatalf("reopen explorer query: %v", err)
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			input, inputFound := automationdriver.Find(snapshot, "launcher.query.input")
			scopeIcons, scopeFound := automationdriver.Find(snapshot, "launcher.query.scope-icons")
			return inputFound && input.Value == "reanchor" && scopeFound && scopeIcons.Value == "1"
		}); err != nil {
			t.Fatalf("wait for reopened explorer query: %v", err)
		}
		assertBottomEdgeStable(t, ctx, client, initialBottom)
	})
}

func assertBottomEdgeStable(t *testing.T, ctx context.Context, client *automationdriver.Client, wantBottom float32) {
	t.Helper()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.Now().Add(2 * time.Second)
	var last woxui.Rect
	for {
		bounds, err := client.Bounds(ctx)
		if err != nil {
			t.Fatalf("read explorer bounds: %v", err)
		}
		last = bounds
		if math.Abs(float64((bounds.Y+bounds.Height)-wantBottom)) <= 1 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("explorer bottom edge = %.1f, want %.1f (bounds=%+v)", bounds.Y+bounds.Height, wantBottom, last)
		}
		select {
		case <-ctx.Done():
			t.Fatalf("wait for stable explorer bottom edge: %v", ctx.Err())
		case <-ticker.C:
		}
	}
}
