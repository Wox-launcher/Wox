//go:build wox_ui_smoke && windows

package query

import (
	"context"
	"testing"
	"time"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
)

const launcherResultResizeQuery = "wox-smoke list-500 "

// Test018LauncherResultResizePreparedFrame verifies result-driven Windows growth is painted before the native window expands.
// Flow: show the empty launcher -> query many deterministic results -> clear the query and results.
// Evidence: native bounds grow with a presented prepared frame, then shrink with only normal presented frames and no diagnostics.
func Test018LauncherResultResizePreparedFrame(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.ShowLauncher(t, ctx, client)
		compactBounds, err := client.Bounds(ctx)
		if err != nil {
			t.Fatalf("read compact launcher bounds: %v", err)
		}

		if err := client.ResetFrameMetrics(ctx); err != nil {
			t.Fatalf("reset frame metrics before launcher growth: %v", err)
		}
		expandedSnapshot := smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, launcherResultResizeQuery)
		if _, found := automationdriver.Find(expandedSnapshot, "launcher.result.perf-list-0000"); !found {
			t.Fatal("deterministic resize result was not found")
		}
		expandedBounds := waitForLauncherResultResize(t, ctx, client, func(height float32) bool {
			return height > compactBounds.Height+1
		})
		assertWindowsResizeFrames(t, ctx, client, true)
		smoke.AssertNoDiagnostics(t, expandedSnapshot)

		if err := client.ResetFrameMetrics(ctx); err != nil {
			t.Fatalf("reset frame metrics before launcher shrink: %v", err)
		}
		collapsedSnapshot := smoke.ReplaceLauncherQuery(t, ctx, client, "")
		waitForLauncherResultResize(t, ctx, client, func(height float32) bool {
			return height < expandedBounds.Height-1
		})
		assertWindowsResizeFrames(t, ctx, client, false)
		smoke.AssertNoDiagnostics(t, collapsedSnapshot)
	})
}

func waitForLauncherResultResize(t *testing.T, ctx context.Context, client *automationdriver.Client, matches func(float32) bool) woxui.Rect {
	t.Helper()
	waitCtx, cancel := context.WithTimeout(ctx, automationdriver.ActionTimeout)
	defer cancel()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	var last woxui.Rect
	for {
		bounds, err := client.Bounds(waitCtx)
		if err != nil {
			t.Fatalf("read launcher bounds during result resize: %v", err)
		}
		last = bounds
		if matches(bounds.Height) {
			return bounds
		}
		select {
		case <-waitCtx.Done():
			t.Fatalf("wait for launcher result resize: last bounds %+v; %v", last, waitCtx.Err())
		case <-ticker.C:
		}
	}
}

func assertWindowsResizeFrames(t *testing.T, ctx context.Context, client *automationdriver.Client, wantPrepared bool) {
	t.Helper()
	metrics, err := client.FrameMetrics(ctx)
	if err != nil {
		t.Fatalf("read Windows result-resize frame metrics: %v", err)
	}
	presented := 0
	prepared := 0
	for _, sample := range metrics.Recent {
		if !sample.Presented {
			continue
		}
		presented++
		if sample.PreparedWindowResize {
			prepared++
		}
	}
	if presented == 0 {
		t.Fatalf("result resize presented no frame: %+v", metrics)
	}
	if wantPrepared && prepared == 0 {
		t.Fatalf("launcher growth presented no prepared frame: %+v", metrics.Recent)
	}
	if !wantPrepared && prepared != 0 {
		t.Fatalf("launcher shrink presented %d prepared frames: %+v", prepared, metrics.Recent)
	}
}
