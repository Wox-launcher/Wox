//go:build wox_ui_smoke

package query

import (
	"context"
	"math"
	"runtime"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// Test047LauncherReshowZeroStaleFrame verifies that a fresh show paints the compact launcher first.
// Flow: capture the empty launcher -> expand deterministic results and preview -> hide -> show fresh again.
// Evidence: the first submitted native frame matches the compact height and semantics and the old result is gone.
func Test047LauncherReshowZeroStaleFrame(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.ConfigureStartPage(t, ctx, client, 0)
		if err := client.ResetFrameMetrics(ctx); err != nil {
			t.Fatal(err)
		}
		smoke.ShowLauncher(t, ctx, client)
		compact := firstPresentedLauncherFrame(t, ctx, client)
		expanded := smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, "wox-smoke list-500 preview")
		if _, found := automationdriver.Find(expanded, "launcher.result.perf-list-0000"); !found {
			t.Fatal("could not populate launcher results")
		}
		previewID := ""
		for _, node := range expanded.Tree.Nodes {
			if node.Value == "Stable repaint smoke preview" {
				previewID = node.AutomationID
				break
			}
		}
		if previewID == "" {
			t.Fatal("expanded launcher did not open the expected preview")
		}
		if _, err := client.WaitFor(ctx, func(woxwidget.AutomationSnapshot) bool {
			bounds, err := client.Bounds(ctx)
			return err == nil && bounds.Height > compact.LogicalSize.Height+1
		}); err != nil {
			t.Fatalf("wait for expanded launcher: %v", err)
		}
		if err := client.Hide(ctx); err != nil {
			t.Fatal(err)
		}
		if err := client.ResetFrameMetrics(ctx); err != nil {
			t.Fatal(err)
		}
		smoke.ShowLauncher(t, ctx, client)
		fresh := firstPresentedLauncherFrame(t, ctx, client)
		if runtime.GOOS == "windows" && !fresh.PreparedWindowShow {
			t.Fatal("the warm Windows reshow did not synchronize its first frame with DWM before showing the window")
		}
		if math.Abs(float64(fresh.LogicalSize.Height-compact.LogicalSize.Height)) > 1 ||
			fresh.AccessibilityNodeCount != compact.AccessibilityNodeCount {
			t.Fatalf("first reshow frame height/nodes = %.1f/%d, initial compact frame = %.1f/%d",
				fresh.LogicalSize.Height, fresh.AccessibilityNodeCount, compact.LogicalSize.Height, compact.AccessibilityNodeCount)
		}
		snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			input, found := automationdriver.Find(snapshot, "launcher.query.input")
			_, stale := automationdriver.Find(snapshot, "launcher.result.perf-list-0000")
			_, stalePreview := automationdriver.Find(snapshot, previewID)
			return snapshot.FrameID >= fresh.FrameID && found && input.Value == "" && !stale && !stalePreview
		})
		if err != nil {
			t.Fatalf("wait for empty fresh query and results: %v", err)
		}
		smoke.AssertNoDiagnostics(t, snapshot)
	})
}

// firstPresentedLauncherFrame waits for a native submission after frame metrics were reset.
func firstPresentedLauncherFrame(t *testing.T, ctx context.Context, client *automationdriver.Client) woxui.FrameMetricsSample {
	t.Helper()
	var first woxui.FrameMetricsSample
	if _, err := client.WaitFor(ctx, func(woxwidget.AutomationSnapshot) bool {
		metrics, err := client.FrameMetrics(ctx)
		if err == nil {
			for _, sample := range metrics.Recent {
				if sample.Presented {
					first = sample
					return true
				}
			}
		}
		return false
	}); err != nil {
		t.Fatalf("wait for first submitted launcher frame: %v", err)
	}
	return first
}
