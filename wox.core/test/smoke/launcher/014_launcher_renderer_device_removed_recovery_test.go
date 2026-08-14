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

// Test014LauncherRendererDeviceRemovedRecovery verifies the Windows launcher survives a lost Direct3D device.
// Flow: show a completed query -> inject DXGI_ERROR_DEVICE_REMOVED into the next frame -> wait for renderer recovery.
// Evidence: frame metrics contain a dropped frame followed by a presented frame, while the same visible window and query remain usable.
func Test014LauncherRendererDeviceRemovedRecovery(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.ShowLauncher(t, ctx, client)
		before := smoke.ReplaceLauncherQuery(t, ctx, client, "1+1")
		if err := client.ResetFrameMetrics(ctx); err != nil {
			t.Fatalf("reset frame metrics before device removal: %v", err)
		}
		if err := client.SimulateRendererDeviceRemoved(ctx); err != nil {
			t.Fatalf("simulate renderer device removal: %v", err)
		}
		if _, err := client.WaitForChange(ctx, before.Tree.Generation); err != nil {
			t.Fatalf("wait for renderer recovery frame: %v", err)
		}

		metrics := waitForDeviceRemovalRecovery(t, ctx, client)
		state, err := client.WindowState(ctx, "primary")
		if err != nil {
			t.Fatalf("read launcher state after renderer recovery: %v", err)
		}
		if !state.Exists || !state.Visible || state.Lifecycle != "visible" {
			t.Fatalf("launcher did not survive renderer recovery: %+v", state)
		}
		snapshot, err := client.Snapshot(ctx)
		if err != nil {
			t.Fatalf("read launcher after renderer recovery: %v", err)
		}
		input, inputFound := automationdriver.Find(snapshot, "launcher.query.input")
		results, resultsFound := automationdriver.Find(snapshot, "launcher.results")
		if !inputFound || input.Value != "1+1" || !resultsFound || results.Value != "complete" {
			t.Fatalf("launcher state changed during renderer recovery: input=%+v results=%+v metrics=%+v", input, results, metrics)
		}
		smoke.AssertNoDiagnostics(t, snapshot)
	})
}

// waitForDeviceRemovalRecovery waits for one failed native frame and its successful retry.
func waitForDeviceRemovalRecovery(t *testing.T, ctx context.Context, client *automationdriver.Client) woxui.FrameMetricsSnapshot {
	t.Helper()
	waitCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	var last woxui.FrameMetricsSnapshot
	for {
		metrics, err := client.FrameMetrics(waitCtx)
		if err != nil {
			t.Fatalf("read renderer recovery metrics: %v", err)
		}
		last = metrics
		droppedFrameID := uint64(0)
		for _, sample := range metrics.Recent {
			if sample.Dropped {
				droppedFrameID = sample.FrameID
			}
			if droppedFrameID != 0 && sample.FrameID > droppedFrameID && sample.Presented {
				return metrics
			}
		}
		select {
		case <-waitCtx.Done():
			t.Fatalf("renderer recovery did not present a frame after device removal: metrics=%+v; %v", last, waitCtx.Err())
		case <-ticker.C:
		}
	}
}
