//go:build wox_ui_smoke

package queryhint

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// Test007ResultSelectionUndo restores offscreen list/grid selections and the list preview through native query editing.
func Test007ResultSelectionUndo(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.ShowLauncher(t, ctx, client)
		modifier := woxui.KeyModifierControl
		if runtime.GOOS == "darwin" {
			modifier = woxui.KeyModifierMeta
		}
		listQuery := "wox-smoke list-500 preview"
		gridQuery := "wox-smoke grid-500 "
		smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, listQuery)
		listID := "launcher.result.perf-list-0000"
		for range 12 {
			if err := client.PressKey(ctx, woxui.KeyArrowDown, 0); err != nil {
				t.Fatal(err)
			}
			listID = selectedUndoFixtureResult(t, ctx, client, "launcher.result.perf-list-", listID)
		}
		smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, gridQuery)
		gridID := "launcher.result.perf-grid-0000"
		for range 12 {
			if err := client.PressKey(ctx, woxui.KeyArrowDown, 0); err != nil {
				t.Fatal(err)
			}
			gridID = selectedUndoFixtureResult(t, ctx, client, "launcher.result.perf-grid-", gridID)
		}
		for range 2 {
			if err := client.PressKey(ctx, woxui.Key("z"), modifier); err != nil {
				t.Fatal(err)
			}
			waitUndoFixtureSelection(t, ctx, client, listQuery, listID, true)
			if err := client.PressKey(ctx, woxui.Key("z"), modifier|woxui.KeyModifierShift); err != nil {
				t.Fatal(err)
			}
			waitUndoFixtureSelection(t, ctx, client, gridQuery, gridID, false)
		}
	})
}

// selectedUndoFixtureResult records a real keyboard selection after the result viewport has scrolled.
func selectedUndoFixtureResult(t *testing.T, ctx context.Context, client *automationdriver.Client, prefix, previousID string) string {
	t.Helper()
	var selectedID string
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		for _, node := range snapshot.Tree.Nodes {
			if node.Selected && strings.HasPrefix(node.AutomationID, prefix) && node.AutomationID != previousID {
				selectedID = node.AutomationID
				return true
			}
		}
		return false
	}); err != nil {
		t.Fatalf("wait for keyboard-selected fixture result: %v", err)
	}
	return selectedID
}

// waitUndoFixtureSelection verifies final query results, selected-row visibility, and preview content together.
func waitUndoFixtureSelection(t *testing.T, ctx context.Context, client *automationdriver.Client, query, resultID string, preview bool) {
	t.Helper()
	snapshot, err := client.WaitForReason(ctx, func(snapshot woxwidget.AutomationSnapshot) (bool, string) {
		input, inputFound := automationdriver.Find(snapshot, "launcher.query.input")
		results, resultsFound := automationdriver.Find(snapshot, "launcher.results")
		row, rowFound := automationdriver.Find(snapshot, resultID)
		visible := rowFound && resultsFound && row.Bounds.Height > 0 && row.Bounds.Y >= results.Bounds.Y && row.Bounds.Y+row.Bounds.Height <= results.Bounds.Y+results.Bounds.Height
		previewFound := !preview
		for _, node := range snapshot.Tree.Nodes {
			if node.Value == "Stable repaint smoke preview" {
				previewFound = true
			}
		}
		return inputFound && input.Value == query && resultsFound && results.Value == "complete" && row.Selected && visible && previewFound,
			"waiting for restored query, visible selected result, and preview: " + resultID
	})
	if err != nil {
		t.Fatalf("restore selection for %q: %v", query, err)
	}
	smoke.AssertNoDiagnostics(t, snapshot)
	waitUndoFixturePresentation(t, ctx, client)
	if directory := os.Getenv("WOX_SMOKE_ARTIFACT_DIR"); directory != "" {
		name := "result-selection-redo-grid.png"
		if preview {
			name = "result-selection-undo-list.png"
		}
		if err := client.Capture(ctx, filepath.Join(directory, name)); err != nil {
			t.Fatal(err)
		}
	}
}

// waitUndoFixturePresentation waits for native presentation after semantics publish the restored result tree.
func waitUndoFixturePresentation(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	waitCtx, cancel := context.WithTimeout(ctx, automationdriver.ActionTimeout)
	defer cancel()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for range 2 {
		before, err := client.FrameMetrics(waitCtx)
		if err != nil {
			t.Fatal(err)
		}
		if err := client.RequestFrame(waitCtx); err != nil {
			t.Fatal(err)
		}
		for {
			select {
			case <-waitCtx.Done():
				t.Fatal("restored results were not presented before the frame deadline")
			case <-ticker.C:
			}
			after, err := client.FrameMetrics(waitCtx)
			if err != nil {
				t.Fatal(err)
			}
			if after.PresentedFrameCount > before.PresentedFrameCount {
				break
			}
		}
	}
}
