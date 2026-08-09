//go:build wox_ui_smoke

package query

import (
	"context"
	"testing"
	"time"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// Test002LauncherQueryStreamingPreviewResize verifies a streamed Preview expands the native launcher.
// Flow: enter the streaming-preview query -> receive its compact initial result -> receive the first Preview update.
// Evidence: the updated result is visible and the real launcher window grows beyond its pre-Preview height.
func Test002LauncherQueryStreamingPreviewResize(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.ShowLauncher(t, ctx, client)
		const query = "wox-smoke-streaming-preview translate"
		if err := client.Perform(ctx, "launcher.query.input", woxui.AccessibilityActionSetValue, query); err != nil {
			t.Fatalf("enter streaming preview query: %v", err)
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			results, resultsFound := automationdriver.Find(snapshot, "launcher.results")
			return resultsFound && results.Value == "complete" && hasResultTitle(snapshot, "Streaming preview pending")
		}); err != nil {
			t.Fatalf("wait for compact streaming result: %v", err)
		}
		initialBounds, err := client.Bounds(ctx)
		if err != nil {
			t.Fatalf("read compact result bounds: %v", err)
		}

		updatedSnapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			return hasResultTitle(snapshot, "Streaming preview received")
		})
		if err != nil {
			t.Fatalf("wait for first streaming preview update: %v", err)
		}
		waitForPreviewResize(t, ctx, client, initialBounds.Height)
		smoke.AssertNoDiagnostics(t, updatedSnapshot)
	})
}

func hasResultTitle(snapshot woxwidget.AutomationSnapshot, title string) bool {
	for _, node := range snapshot.Tree.Nodes {
		if node.Role == woxui.AccessibilityRoleListItem && node.Label == title {
			return true
		}
	}
	return false
}

func waitForPreviewResize(t *testing.T, ctx context.Context, client *automationdriver.Client, initialHeight float32) {
	t.Helper()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		bounds, err := client.Bounds(ctx)
		if err != nil {
			t.Fatalf("read launcher bounds after streaming preview update: %v", err)
		}
		if bounds.Height > initialHeight+1 {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("wait for preview to expand launcher: initial height %.1f, current height %.1f; %v", initialHeight, bounds.Height, ctx.Err())
		case <-ticker.C:
		}
	}
}
