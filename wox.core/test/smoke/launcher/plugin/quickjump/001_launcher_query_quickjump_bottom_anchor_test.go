//go:build wox_ui_smoke

package quickjump

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

const quickJumpInstance = string(common.ShowSourceQuickJump)

// Test001LauncherQueryQuickJumpBottomAnchor verifies the Quick Jump
// secondary keeps a fixed bottom edge while rapid queries resize it.
// Flow: open Quick Jump secondary -> capture bottom edge -> type successive queries that
// change result height -> reopen the same Quick Jump instance with a new query.
// Evidence: native window bottom (Y+Height) stays within 1px across resizes and re-shows.
func Test001LauncherQueryQuickJumpBottomAnchor(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		if err := client.OpenQuickJumpQuery(ctx, "a"); err != nil {
			t.Fatalf("open quick jump query: %v", err)
		}
		if _, err := client.WaitForWindowState(ctx, quickJumpInstance, func(state automationdriver.WindowState) bool {
			return state.Exists && state.Visible
		}); err != nil {
			t.Fatalf("wait for quick jump window: %v", err)
		}
		if err := client.FocusInstance(ctx, quickJumpInstance); err != nil {
			t.Fatalf("focus quick jump instance: %v", err)
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			input, inputFound := automationdriver.Find(snapshot, "launcher.query.input")
			scopeIcons, scopeFound := automationdriver.Find(snapshot, "launcher.query.scope-icons")
			return inputFound && input.Value == "a" && scopeFound && scopeIcons.Value == "1"
		}); err != nil {
			t.Fatalf("wait for quick jump query input: %v", err)
		}

		initialBounds, err := client.Bounds(ctx)
		if err != nil {
			t.Fatalf("read initial quick jump bounds: %v", err)
		}
		initialBottom := initialBounds.Y + initialBounds.Height

		for _, query := range []string{"ab", "abc", "1+1", "a", "smoke"} {
			if err := client.Perform(ctx, "launcher.query.input", woxui.AccessibilityActionSetValue, query); err != nil {
				t.Fatalf("enter quick jump query %q: %v", query, err)
			}
			if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
				input, inputFound := automationdriver.Find(snapshot, "launcher.query.input")
				scopeIcons, scopeFound := automationdriver.Find(snapshot, "launcher.query.scope-icons")
				results, resultsFound := automationdriver.Find(snapshot, "launcher.results")
				return inputFound && input.Value == query && scopeFound && scopeIcons.Value == "1" && (!resultsFound || results.Value == "complete")
			}); err != nil {
				t.Fatalf("wait for quick jump query %q: %v", query, err)
			}
			assertBottomEdgeStable(t, ctx, client, initialBottom)
		}

		if err := client.OpenQuickJumpQuery(ctx, "reanchor"); err != nil {
			t.Fatalf("reopen quick jump query: %v", err)
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			input, inputFound := automationdriver.Find(snapshot, "launcher.query.input")
			scopeIcons, scopeFound := automationdriver.Find(snapshot, "launcher.query.scope-icons")
			return inputFound && input.Value == "reanchor" && scopeFound && scopeIcons.Value == "1"
		}); err != nil {
			t.Fatalf("wait for reopened quick jump query: %v", err)
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
			t.Fatalf("read quick jump bounds: %v", err)
		}
		last = bounds
		if math.Abs(float64((bounds.Y+bounds.Height)-wantBottom)) <= 1 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("quick jump bottom edge = %.1f, want %.1f (bounds=%+v)", bounds.Y+bounds.Height, wantBottom, last)
		}
		select {
		case <-ctx.Done():
			t.Fatalf("wait for stable quick jump bottom edge: %v", ctx.Err())
		case <-ticker.C:
		}
	}
}
