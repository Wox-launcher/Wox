//go:build wox_ui_smoke

package query

import (
	"context"
	"runtime"
	"strings"
	"testing"
	"time"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// Test025LauncherRepaintInteractions checks every frame, including resize and buffer-repair frames.
// Flow: type/filter/pause/clear in the action panel, with and without repaint highlights.
// Evidence: action damage stays inside the panel and blur halo, including restored result rows.
func Test025LauncherRepaintInteractions(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows retained-buffer repaint contract")
	}
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.ShowLauncher(t, ctx, client)
		smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, "> "+actionPanelShellCommand)
		snapshot := smoke.OpenResultActionPanel(t, ctx, client)
		var panel woxui.Rect
		for _, node := range snapshot.Tree.Nodes {
			if strings.HasPrefix(node.AutomationID, "action-") {
				panel = unionRect(panel, node.Bounds)
			}
		}
		panel = expandRect(panel, 96)
		for _, mode := range []woxwidget.RepaintDebugMode{woxwidget.RepaintDebugOff, woxwidget.RepaintDebugRainbow} {
			if err := client.SetRepaintDebugMode(ctx, mode); err != nil {
				t.Fatal(err)
			}
			for cycle := 0; cycle < 2; cycle++ {
				assertInteractionDamage(t, ctx, client, panel, "filter", func() error { return client.EnterText(ctx, "__missing__") })
				assertInteractionDamage(t, ctx, client, panel, "continued typing", func() error { return client.EnterText(ctx, "x") })
				filtered, err := client.Snapshot(ctx)
				if err != nil {
					t.Fatal(err)
				}
				input, found := automationdriver.Find(filtered, "action-search")
				if !found || input.Value != "__missing__x" || len(actionPanelResultNodes(filtered)) != 0 {
					t.Fatal("filter did not produce the expected empty panel")
				}
				assertInteractionDamage(t, ctx, client, panel, "clear after buffer history settles", func() error {
					if err := client.PressKey(ctx, woxui.Key("a"), woxui.KeyModifierControl); err != nil {
						return err
					}
					return client.PressKey(ctx, woxui.KeyBackspace, 0)
				})
				cleared, err := client.Snapshot(ctx)
				if err != nil {
					t.Fatal(err)
				}
				input, found = automationdriver.Find(cleared, "action-search")
				if !found || input.Value != "" || len(actionPanelResultNodes(cleared)) != len(actionPanelResultNodes(snapshot)) {
					t.Fatal("clearing did not restore the action rows")
				}
			}
		}
		if err := client.PressKey(ctx, woxui.KeyEscape, 0); err != nil {
			t.Fatal(err)
		}
		if err := client.SetRepaintDebugMode(ctx, woxwidget.RepaintDebugOff); err != nil {
			t.Fatal(err)
		}
	})
}

// interactionFrameQuiet is how long completed frames must stop arriving before the
// stream is settled. The next present carries retained-buffer repair, so this stays
// wider than a vsync, and it stays under the 500ms caret blink so a focused editor
// can still go quiet. The budgets are the old fixed sleeps: a burst that never rests
// still waits that long, and an idle stream returns as soon as it is quiet.
const interactionFrameQuiet = 200 * time.Millisecond

const (
	interactionSettleBudget  = 1200 * time.Millisecond
	interactionObserveBudget = 900 * time.Millisecond
)

// assertInteractionDamage drains frames already in flight, then checks every frame the action presents.
func assertInteractionDamage(t *testing.T, ctx context.Context, client *automationdriver.Client, allowed woxui.Rect, label string, action func() error) {
	t.Helper()
	if err := drainInteractionFrames(ctx, client); err != nil {
		t.Fatal(err)
	}
	if err := action(); err != nil {
		t.Fatal(err)
	}
	if err := waitForInteractionFrames(ctx, client, interactionObserveBudget, true); err != nil {
		t.Fatal(err)
	}
	metrics, err := client.FrameMetrics(ctx)
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, frame := range metrics.Recent {
		if !frame.HostCompleted {
			continue
		}
		damage := frame.LogicalDamage
		// An idle completion can record an empty rect. That is not a repaint; a real
		// full-window paint still carries a rect and is rejected below.
		if damage.Width <= 0 || damage.Height <= 0 {
			continue
		}
		checked++
		if !containsRect(allowed, damage) {
			t.Fatalf("%s frame %d damage=%+v allowed=%+v", label, frame.FrameID, damage, allowed)
		}
	}
	if checked == 0 {
		t.Fatalf("%s produced no observed frames", label)
	}
}

// drainInteractionFrames waits until completed frames stop arriving, then forgets them.
func drainInteractionFrames(ctx context.Context, client *automationdriver.Client) error {
	if err := waitForInteractionFrames(ctx, client, interactionSettleBudget, false); err != nil {
		return err
	}
	return client.ResetFrameMetrics(ctx)
}

// waitForInteractionFrames polls until completed frames stop arriving.
// requireFrame waits for the action to present at least one frame before accepting quiet.
func waitForInteractionFrames(ctx context.Context, client *automationdriver.Client, budget time.Duration, requireFrame bool) error {
	deadline := time.Now().Add(budget)
	quietSince := time.Time{}
	var lastID uint64
	seen := false
	for {
		metrics, err := client.FrameMetrics(ctx)
		if err != nil {
			return err
		}
		frameID := newestCompletedFrameID(metrics, requireFrame)
		now := time.Now()
		if !seen || frameID != lastID {
			seen = true
			lastID = frameID
			quietSince = now
		}
		if now.Sub(quietSince) >= interactionFrameQuiet && (!requireFrame || frameID > 0) {
			return nil
		}
		if !now.Before(deadline) {
			return nil
		}
		timer := time.NewTimer(25 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

// newestCompletedFrameID returns the newest finished frame. When damaged is set, empty
// idle completions do not count, so a caret tick cannot close the wait before the action paints.
func newestCompletedFrameID(metrics woxui.FrameMetricsSnapshot, damaged bool) uint64 {
	frameID := uint64(0)
	for _, frame := range metrics.Recent {
		if !frame.HostCompleted || frame.FrameID <= frameID {
			continue
		}
		if damaged && (frame.LogicalDamage.Width <= 0 || frame.LogicalDamage.Height <= 0) {
			continue
		}
		frameID = frame.FrameID
	}
	return frameID
}
