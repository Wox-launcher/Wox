//go:build wox_ui_smoke

package query

import (
	"context"
	"runtime"
	"testing"
	"time"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// Test006LauncherInputRepaintDamage verifies idle caret blinking stays local in both launcher editors.
// Flow: settle a completed query -> observe query-box caret frames -> open the action panel -> observe its filter caret frames.
// Evidence: every settled frame reports non-empty logical damage contained by the focused input instead of full-window damage.
func Test006LauncherInputRepaintDamage(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.ShowLauncher(t, ctx, client)
		snapshot := smoke.ReplaceLauncherQuery(t, ctx, client, "1+1")
		assertIdleInputDamage(t, ctx, client, snapshot, "launcher.query.input")

		modifier := woxui.KeyModifierControl
		if runtime.GOOS == "darwin" {
			modifier = woxui.KeyModifierMeta
		}
		if err := client.PressKey(ctx, woxui.Key("j"), modifier); err != nil {
			t.Fatalf("open launcher action panel: %v", err)
		}
		snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			input, found := automationdriver.Find(snapshot, "action-search")
			return found && input.Focused
		})
		if err != nil {
			t.Fatalf("wait for focused action filter: %v", err)
		}
		assertIdleInputDamage(t, ctx, client, snapshot, "action-search")
		smoke.AssertNoDiagnostics(t, snapshot)
	})
}

// assertIdleInputDamage waits through one settling caret frame, then checks two complete blink phases.
func assertIdleInputDamage(t *testing.T, ctx context.Context, client *automationdriver.Client, snapshot woxwidget.AutomationSnapshot, inputID string) {
	t.Helper()
	input, found := automationdriver.Find(snapshot, inputID)
	if !found || !input.Focused {
		t.Fatalf("idle repaint input %q = found %v focused %v", inputID, found, input.Focused)
	}

	settled, err := client.WaitForChange(ctx, snapshot.Tree.Generation)
	if err != nil {
		t.Fatalf("wait for %q to settle on a caret frame: %v", inputID, err)
	}
	if err := client.ResetFrameMetrics(ctx); err != nil {
		t.Fatalf("reset frame metrics for %q: %v", inputID, err)
	}
	// Host damage includes a 4px paint outset; one extra logical pixel absorbs fractional layout bounds.
	allowed := expandRect(input.Bounds, 5)
	generation := settled.Tree.Generation
	lastFrameID := uint64(0)
	consecutiveLocal := 0
	observed := make([]woxui.FrameMetricsSample, 0, 16)
	idleCtx, cancelIdle := context.WithTimeout(ctx, 6*time.Second)
	defer cancelIdle()
	for idleCtx.Err() == nil {
		next, waitErr := client.WaitForChange(idleCtx, generation)
		if waitErr != nil {
			break
		}
		generation = next.Tree.Generation
		metrics, metricsErr := client.FrameMetrics(ctx)
		if metricsErr != nil {
			t.Fatalf("read idle frame metrics for %q: %v", inputID, metricsErr)
		}
		for _, sample := range metrics.Recent {
			if sample.FrameID <= lastFrameID || !sample.HostCompleted || !sample.Presented {
				continue
			}
			lastFrameID = sample.FrameID
			observed = append(observed, sample)
			local := sample.LogicalDamage.Width > 0 && sample.LogicalDamage.Height > 0 && containsRect(allowed, sample.LogicalDamage)
			if local {
				consecutiveLocal++
				if consecutiveLocal >= 2 {
					return
				}
			} else {
				consecutiveLocal = 0
			}
		}
	}
	t.Fatalf("idle repaint for %q never settled to two consecutive local frames within %+v; allowed bounds %+v", inputID, observed, allowed)
}

func expandRect(rect woxui.Rect, outset float32) woxui.Rect {
	return woxui.Rect{X: rect.X - outset, Y: rect.Y - outset, Width: rect.Width + 2*outset, Height: rect.Height + 2*outset}
}

func containsRect(outer, inner woxui.Rect) bool {
	return inner.X >= outer.X && inner.Y >= outer.Y && inner.X+inner.Width <= outer.X+outer.Width && inner.Y+inner.Height <= outer.Y+outer.Height
}
