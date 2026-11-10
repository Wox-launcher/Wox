package launcher

import (
	"encoding/json"
	"slices"
	"strconv"
	"time"

	"wox/common"
	"wox/common/icons"
	"wox/plugin"
	"wox/plugin/system/shell/history"
	"wox/plugin/system/shell/interactive"
	"wox/plugin/system/shell/terminal"
	"wox/util"
)

// withShellSessionHistory inserts the active shell's row until it appears in the Core snapshot.
func (a *App) withShellSessionHistory(queryID string, results []queryResult) []queryResult {
	if a.previews.pluginPreview.pluginID != common.ShellPluginID || len(results) == 0 ||
		len(results[0].Actions) == 0 || results[0].Actions[0].ID != interactive.FocusActionID {
		return results
	}
	var run *shellPreviewRun
	shell, ok := a.previews.pluginPreview.result()
	if ok && a.shellPreview.queryID == shell.QueryID && a.shellPreview.resultID == shell.ID {
		run = a.shellPreview.run
	}
	var entry interactive.HistoryEntry
	if run != nil && run.frame != nil {
		entry = run.frame.History
	}
	if entry.ID == "" && !slices.ContainsFunc(results, func(row queryResult) bool {
		return row.localShellRow
	}) {
		return results
	}
	rows := make([]queryResult, 0, len(results)+2)
	for _, row := range results {
		if !row.localShellRow {
			rows = append(rows, row)
		}
	}
	if entry.ID == "" {
		return rows
	}
	// The database owns the row after it first appears, including any later deletion.
	if resultIndexByID(rows, entry.ID) >= 0 {
		run.historyPublished = true
	}
	if run.historyPublished {
		return rows
	}

	status := "i18n:plugin_shell_status_running"
	switch entry.Status {
	case interactive.CommandCompleted:
		status = "i18n:plugin_shell_status_success"
	case interactive.CommandFailed:
		status = "i18n:plugin_shell_status_failed"
	}
	group, groupScore := history.Group(entry.StartedAt, time.Now())
	score := entry.StartedAt.UnixMilli()
	index := 1
	for ; index < len(rows); index++ {
		row := rows[index]
		if row.groupScore < groupScore || (row.groupScore == groupScore && !row.IsGroup && row.score <= score) {
			break
		}
	}
	if rows[index-1].groupScore != groupScore {
		rows = slices.Insert(rows, index, queryResult{
			QueryID: queryID, ID: "shell-session:group:" + strconv.FormatInt(groupScore, 10),
			Title: a.language.translate(group), IsGroup: true, groupScore: groupScore, localShellRow: true,
		})
		index++
	}
	row := queryResult{
		QueryID: queryID, ID: entry.ID,
		Title: entry.Title, SubTitle: a.language.translate(status), Icon: results[0].Icon, Actions: results[0].Actions[:1],
		score: score, groupScore: groupScore, localShellRow: true,
	}
	if entry.Saved {
		options, _ := json.Marshal(interactive.Options{HistoryID: entry.ID})
		row.Preview = queryPreview{PreviewType: plugin.WoxPreviewTypeShell, PreviewData: string(options), PreviewTags: shell.Preview.PreviewTags}
		row.Actions = []resultAction{{
			ID: terminal.ShowHistoryActionID, Name: a.language.translate("i18n:plugin_shell_interactive_open"), Icon: fromCoreImage(icons.Get(icons.ActionPreview)),
			IsDefault: true, PreventHideAfterAction: true,
		}}
	}
	return slices.Insert(rows, index, row)
}

// syncShellSessionHistory updates the session row while keeping keyboard selection stable.
func (a *App) syncShellSessionHistory() {
	list := &a.resultList
	if len(list.results) == 0 || list.resultsQueryID != a.querySession.query.QueryID {
		return
	}
	rows := a.withShellSessionHistory(list.resultsQueryID, list.results)
	if &rows[0] == &list.results[0] {
		return
	}
	selected := ""
	if list.selected >= 0 && list.selected < len(list.results) {
		selected = list.results[list.selected].ID
	}
	sizeChanged := len(rows) != len(list.results)
	if a.host.HasFocus(shellPreviewFocusKey) {
		a.shellPreview.focusPending = true
	}
	list.publishRows(list.resultsQueryID, rows)
	list.selected = max(0, resultIndexByID(rows, selected))
	if sizeChanged && a.window != nil {
		if err := a.applyWindowBounds(); err != nil {
			util.GetLogger().Warn(a.lifecycleCtx, err.Error())
		}
	}
	a.invalidateShellPreview()
}
