//go:build wox_ui_smoke

package calculator

import (
	"context"
	"math"
	"strings"
	"testing"
	"time"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// Test001LauncherQueryCalculator covers the native launcher calculator query path.
func Test001LauncherQueryCalculator(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.ShowLauncher(t, ctx, client)
		initialBounds, err := client.Bounds(ctx)
		if err != nil {
			t.Fatalf("read initial launcher bounds: %v", err)
		}
		for _, query := range []string{"s", "sm", "smo", "smok", "smoke"} {
			if err := client.Perform(ctx, "launcher.query.input", woxui.AccessibilityActionSetValue, query); err != nil {
				t.Fatalf("enter rapid query %q: %v", query, err)
			}
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			node, found := automationdriver.Find(snapshot, "launcher.query.input")
			return found && node.Value == "smoke"
		}); err != nil {
			t.Fatalf("wait for rapid query input: %v", err)
		}
		if err := client.Perform(ctx, "launcher.query.input", woxui.AccessibilityActionSetValue, "1+1"); err != nil {
			t.Fatalf("enter calculator query: %v", err)
		}
		snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			_, found := calculatorResult(snapshot)
			return found
		})
		if err != nil {
			t.Fatalf("wait for query result: %v", err)
		}
		if len(snapshot.Diagnostics) > 0 {
			t.Fatalf("launcher semantics diagnostics: %v", snapshot.Diagnostics)
		}
		if count := launcherResultCount(snapshot); count != 1 {
			t.Fatalf("launcher result count = %d, want 1", count)
		}
		resultBounds := waitForExpandedLauncher(t, ctx, client, initialBounds.Height)
		assertWindowOrigin(t, resultBounds, initialBounds)
		if err := client.Hide(ctx); err != nil {
			t.Fatalf("hide launcher: %v", err)
		}
	})
}

// waitForExpandedLauncher waits for native resize to catch up with the published result snapshot.
func waitForExpandedLauncher(t *testing.T, ctx context.Context, client *automationdriver.Client, initialHeight float32) woxui.Rect {
	t.Helper()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		bounds, err := client.Bounds(ctx)
		if err != nil {
			t.Fatalf("read launcher bounds after query: %v", err)
		}
		if bounds.Height > initialHeight+1 {
			return bounds
		}
		select {
		case <-ctx.Done():
			t.Fatalf("wait for launcher result resize: %v", ctx.Err())
		case <-ticker.C:
		}
	}
}

func assertWindowOrigin(t *testing.T, actual, expected woxui.Rect) {
	t.Helper()
	if math.Abs(float64(actual.X-expected.X)) > 1 || math.Abs(float64(actual.Y-expected.Y)) > 1 {
		t.Fatalf("launcher origin = %.1f,%.1f, want %.1f,%.1f", actual.X, actual.Y, expected.X, expected.Y)
	}
}

func calculatorResult(snapshot woxwidget.AutomationSnapshot) (string, bool) {
	for _, node := range snapshot.Tree.Nodes {
		if strings.HasPrefix(node.AutomationID, "launcher.result.") && strings.TrimSpace(node.Label) == "2" {
			return node.AutomationID, true
		}
	}
	return "", false
}

func launcherResultCount(snapshot woxwidget.AutomationSnapshot) int {
	count := 0
	for _, node := range snapshot.Tree.Nodes {
		if strings.HasPrefix(node.AutomationID, "launcher.result.") {
			count++
		}
	}
	return count
}
