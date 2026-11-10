package launcher

import (
	"slices"
	"testing"
	"time"

	"wox/common"
	"wox/plugin/system/shell/interactive"
)

// TestShellSessionHistory covers grouping, status updates and replacement by the saved row.
func TestShellSessionHistory(t *testing.T) {
	const queryID = "query"
	now := time.Now()
	app := &App{
		previews: previewController{pluginPreview: pluginPreviewState{
			pluginID: common.ShellPluginID, queryID: queryID, preview: &queryPreview{},
		}},
	}
	shell, _ := app.previews.pluginPreview.result()
	app.shellPreview = shellPreviewState{queryID: queryID, resultID: shell.ID, run: &shellPreviewRun{
		frame: &interactive.Frame{History: interactive.HistoryEntry{
			ID: "interactive:recent", Title: "recent interactive", StartedAt: now.Add(time.Minute),
		}},
	}}
	results := []queryResult{
		{ID: "compose", groupScore: 200, Actions: []resultAction{{ID: interactive.FocusActionID}}},
		{ID: "saved", IsGroup: true, groupScore: 100},
		{ID: "saved-command", groupScore: 100},
		{ID: "today", IsGroup: true, groupScore: 90},
		{ID: "newer-one-shot", groupScore: 90, score: now.Add(2 * time.Minute).UnixMilli()},
		{ID: "older-one-shot", groupScore: 90, score: now.UnixMilli()},
	}
	checkOrder := func(rows []queryResult, want []string) {
		t.Helper()
		ids := make([]string, 0, len(rows))
		for _, row := range rows {
			ids = append(ids, row.ID)
		}
		if !slices.Equal(ids, want) {
			t.Fatalf("rows = %v, want %v", ids, want)
		}
	}
	want := []string{"compose", "saved", "saved-command", "today", "newer-one-shot", "interactive:recent", "older-one-shot"}
	rows := app.withShellSessionHistory(queryID, results)
	checkOrder(rows, want)
	withNewGroup := app.withShellSessionHistory(queryID, results[:3])
	checkOrder(withNewGroup, []string{"compose", "saved", "saved-command", "shell-session:group:90", "interactive:recent"})
	if !withNewGroup[3].IsGroup {
		t.Fatal("the new history group header is selectable")
	}
	entry := app.shellPreview.run.frame.History
	entry.Status = interactive.CommandCompleted
	entry.Saved = true
	app.shellPreview.run.frame = &interactive.Frame{History: entry}
	completed := app.withShellSessionHistory(queryID, rows)
	checkOrder(completed, want)
	if completed[5].SubTitle == rows[5].SubTitle || completed[5].Preview.PreviewData == "" {
		t.Fatal("the existing session row did not pick up its saved state")
	}
	persisted := slices.Clone(results)
	persisted = slices.Insert(persisted, 5, queryResult{ID: "interactive:recent", groupScore: 90, score: now.Add(time.Minute).UnixMilli()})
	withPersisted := app.withShellSessionHistory(queryID, persisted)
	checkOrder(withPersisted, want)
	if withPersisted[5].localShellRow {
		t.Fatal("the saved result should replace the session row")
	}
	checkOrder(app.withShellSessionHistory(queryID, results), slices.Delete(slices.Clone(want), 5, 6))
	app.shellPreview.run = nil
	checkOrder(app.withShellSessionHistory(queryID, rows), []string{"compose", "saved", "saved-command", "today", "newer-one-shot", "older-one-shot"})
}
