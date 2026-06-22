package system

import (
	"context"
	"fmt"
	"strings"
	"time"
	"wox/common"
	"wox/plugin"
	"wox/util"
	"wox/util/filesearch"
)

const maxFileSearchStatusRootRows = 50

func (c *FileSearchPlugin) isStatusQuery(query plugin.Query) bool {
	if !util.IsDev() {
		return false
	}
	// Bug fix: only the parser's command slot should activate the diagnostic
	// status command. The previous search fallback made `f status` shadow a normal
	// file search for "status", while Wox command syntax requires `f status ` to
	// promote the second token from search text to command text.
	return strings.EqualFold(strings.TrimSpace(query.Command), fileSearchStatusCommand)
}

func (c *FileSearchPlugin) queryStatus(ctx context.Context) plugin.QueryResponse {
	if c.engine == nil {
		return plugin.QueryResponse{}
	}

	// Feature addition: status is diagnostic, not a normal search. Use a bounded
	// context so an expensive SQLite snapshot cannot make the launcher feel
	// frozen while still returning the partial engine state captured before any
	// snapshot timeout.
	statusCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	diagnostics, err := c.engine.GetDiagnostics(statusCtx)
	if err != nil {
		c.api.Log(ctx, plugin.LogLevelError, "Failed to get file search status: "+err.Error())
		return plugin.QueryResponse{
			Results: []plugin.QueryResult{
				{
					Title:    "File Search Status",
					SubTitle: err.Error(),
					Icon:     fileIcon,
					Preview: plugin.WoxPreview{
						PreviewType: plugin.WoxPreviewTypeMarkdown,
						PreviewData: "## File Search Status\n\n" + err.Error(),
					},
				},
			},
		}
	}

	return c.buildStatusQueryResponse(diagnostics)
}

func (c *FileSearchPlugin) buildStatusQueryResponse(diagnostics filesearch.DiagnosticSnapshot) plugin.QueryResponse {
	report := formatFileSearchStatusMarkdown(diagnostics)
	widthRatio := 0.0
	return plugin.QueryResponse{
		Results: []plugin.QueryResult{
			{
				Title:    "File Search Status",
				SubTitle: fileSearchStatusSubtitle(diagnostics),
				Icon:     fileIcon,
				Preview: plugin.WoxPreview{
					PreviewType: plugin.WoxPreviewTypeMarkdown,
					PreviewData: report,
				},
				Actions: []plugin.QueryResultAction{
					{
						Name:                   "Copy Report",
						Icon:                   common.CopyIcon,
						PreventHideAfterAction: true,
						Action: func(ctx context.Context, actionContext plugin.ActionContext) {
							c.api.Copy(ctx, plugin.CopyParams{Type: plugin.CopyTypePlainText, Text: report})
						},
					},
				},
			},
		},
		// Feature addition: the status report is meant for diagnosis, not result
		// browsing. A non-nil zero ratio tells the launcher to give the markdown
		// preview the full result area while keeping this behavior scoped to the
		// File Search status command.
		Layout: plugin.QueryLayout{ResultPreviewWidthRatio: &widthRatio},
	}
}

func fileSearchStatusSubtitle(diagnostics filesearch.DiagnosticSnapshot) string {
	// Feature addition: the result row is the first visible surface for `f status`.
	// Keep persisted index counts there so diagnostics can distinguish an empty
	// database from a live-progress display problem without opening the preview.
	indexSummary := formatFileSearchIndexCountSummary(diagnostics.Index)
	return fmt.Sprintf(
		"roots: %d (%d dynamic), indexed: %s, pending: %d roots / %d paths, indexing: %t",
		diagnostics.RootCount,
		diagnostics.DynamicRootCount,
		indexSummary,
		diagnostics.DirtyQueue.PendingRootCount,
		diagnostics.DirtyQueue.PendingPathCount,
		diagnostics.Status.IsIndexing,
	)
}

func formatFileSearchStatusMarkdown(diagnostics filesearch.DiagnosticSnapshot) string {
	var builder strings.Builder
	builder.WriteString("# File Search Status\n\n")
	builder.WriteString(fmt.Sprintf("- captured_at: `%s`\n", diagnostics.CapturedAt.Format(time.RFC3339)))
	builder.WriteString(fmt.Sprintf("- roots: total=%d visible=%d default=%d user=%d dynamic=%d\n", diagnostics.RootCount, diagnostics.UserVisibleRootCount, diagnostics.DefaultRootCount, diagnostics.UserRootCount, diagnostics.DynamicRootCount))
	// Feature addition: mirror the subtitle's persisted count in the copied
	// markdown summary so the first lines carry the current index volume.
	builder.WriteString(fmt.Sprintf("- index: %s\n", formatFileSearchIndexCountSummary(diagnostics.Index)))
	builder.WriteString(fmt.Sprintf("- pending roots=%d paths=%d root_signals=%d\n", diagnostics.DirtyQueue.PendingRootCount, diagnostics.DirtyQueue.PendingPathCount, diagnostics.DirtyQueue.PendingRootSignalCount))
	builder.WriteString(fmt.Sprintf("- indexing: %t\n", diagnostics.Status.IsIndexing))
	builder.WriteString("\n")

	builder.WriteString("## Root Counters\n\n")
	builder.WriteString(fmt.Sprintf("- kinds: %s\n", formatRootKindCounts(diagnostics.RootKindCounts)))
	builder.WriteString(fmt.Sprintf("- statuses: %s\n", formatRootStatusCounts(diagnostics.RootStatusCounts)))
	builder.WriteString(fmt.Sprintf("- feed types: %s\n", formatRootFeedTypeCounts(diagnostics.RootFeedTypeCounts)))
	builder.WriteString(fmt.Sprintf("- feed states: %s\n", formatRootFeedStateCounts(diagnostics.RootFeedStateCounts)))
	builder.WriteString("\n")

	builder.WriteString("## Dirty Queue\n\n")
	builder.WriteString(fmt.Sprintf("- pending: roots=%d paths=%d root_signals=%d\n", diagnostics.DirtyQueue.PendingRootCount, diagnostics.DirtyQueue.PendingPathCount, diagnostics.DirtyQueue.PendingRootSignalCount))
	builder.WriteString(fmt.Sprintf("- latest_signal_age: `%s`\n", formatSignalAge(diagnostics.DirtyQueue.LatestSignal, diagnostics.CapturedAt)))
	builder.WriteString(fmt.Sprintf("- earliest_signal_age: `%s`\n", formatSignalAge(diagnostics.DirtyQueue.EarliestSignal, diagnostics.CapturedAt)))
	builder.WriteString(fmt.Sprintf("- current_debounce: `%s`\n", formatFileSearchStatusDuration(diagnostics.DirtyQueue.CurrentDebounceWindow)))
	builder.WriteString(fmt.Sprintf("- next_flush_in: `%s`\n", formatFileSearchStatusDuration(diagnostics.DirtyQueue.NextFlushIn)))
	builder.WriteString(fmt.Sprintf("- last_dirty_run: `%s`\n", formatFileSearchStatusDuration(diagnostics.DirtyQueue.LastDirtyRunElapsed)))
	builder.WriteString(fmt.Sprintf("- config: debounce=`%s`, max_pending_wait=`%s`, max_debounce=`%s`, path_threshold=%d, root_threshold=%d, sibling_merge=%d\n",
		formatFileSearchStatusDuration(diagnostics.DirtyQueue.Config.DebounceWindow),
		formatFileSearchStatusDuration(diagnostics.DirtyQueue.Config.MaxPendingWaitWindow),
		formatFileSearchStatusDuration(diagnostics.DirtyQueue.Config.MaxDebounceWindow),
		diagnostics.DirtyQueue.Config.BackpressurePathThreshold,
		diagnostics.DirtyQueue.Config.BackpressureRootThreshold,
		diagnostics.DirtyQueue.Config.SiblingMergeThreshold,
	))
	builder.WriteString("\n")

	builder.WriteString("## Active Run\n\n")
	builder.WriteString(fmt.Sprintf("- run: kind=`%s`, status=`%s`, job=`%s`, stage=`%s`\n", diagnostics.Status.ActiveRunKind, diagnostics.Status.ActiveRunStatus, diagnostics.Status.ActiveJobKind, diagnostics.Status.ActiveStage))
	builder.WriteString(fmt.Sprintf("- root: `%s`\n", emptyStatusValue(diagnostics.Status.ActiveRootPath)))
	builder.WriteString(fmt.Sprintf("- scope: `%s`\n", emptyStatusValue(diagnostics.Status.ActiveScopePath)))
	builder.WriteString(fmt.Sprintf("- progress: %d/%d root=%d/%d item=%d/%d\n",
		diagnostics.Status.RunProgressCurrent,
		diagnostics.Status.RunProgressTotal,
		diagnostics.Status.ActiveRootIndex,
		diagnostics.Status.ActiveRootTotal,
		diagnostics.Status.ActiveItemCurrent,
		diagnostics.Status.ActiveItemTotal,
	))
	builder.WriteString(fmt.Sprintf("- counted: files=%s entries=%s elapsed=`%s`\n", formatFileSearchCount(diagnostics.Status.ActiveRunFileCount), formatFileSearchCount(diagnostics.Status.ActiveRunEntryCount), formatFileSearchStatusDuration(time.Duration(diagnostics.Status.ActiveRunElapsedMs)*time.Millisecond)))
	if diagnostics.Status.LastError != "" {
		builder.WriteString(fmt.Sprintf("- last_error: `%s`\n", statusMarkdownCell(diagnostics.Status.LastError)))
	}
	builder.WriteString("\n")

	builder.WriteString("## Index\n\n")
	if diagnostics.Index.Error != "" {
		if diagnostics.Index.CountsAvailable {
			builder.WriteString(fmt.Sprintf("- counts: %s\n", formatFileSearchIndexVolume(diagnostics.Index)))
		}
		builder.WriteString(fmt.Sprintf("- snapshot_error: `%s`\n", statusMarkdownCell(diagnostics.Index.Error)))
	} else {
		builder.WriteString(fmt.Sprintf("- entries=%s files=%s roots=%d bigram_rows=%s\n", formatFileSearchCount(diagnostics.Index.EntryCount), formatFileSearchCount(diagnostics.Index.FileCount), diagnostics.Index.RootCount, formatFileSearchCount(diagnostics.Index.BigramRowCount)))
		builder.WriteString(fmt.Sprintf("- fts_vocab: name=%s path=%s pinyin_full=%s initials=%s\n", formatFileSearchCount(diagnostics.Index.NameFTSVocab), formatFileSearchCount(diagnostics.Index.PathFTSVocab), formatFileSearchCount(diagnostics.Index.PinyinFullFTSVocab), formatFileSearchCount(diagnostics.Index.InitialsFTSVocab)))
		builder.WriteString(fmt.Sprintf("- bytes_est: fact=%s fts_source=%s bigram=%s total=%s db_total=%s\n", formatFileSearchCount(diagnostics.Index.FactBytesEstimate), formatFileSearchCount(diagnostics.Index.FTSSourceBytesEstimate), formatFileSearchCount(diagnostics.Index.BigramBytesEstimate), formatFileSearchCount(diagnostics.Index.TotalBytesEstimate), formatFileSearchCount(diagnostics.Index.DBTotalFileBytes)))
		if len(diagnostics.Index.TopRoots) > 0 {
			builder.WriteString("\n")
			builder.WriteString("| top root | docs | bigrams | bytes_est |\n")
			builder.WriteString("| --- | ---: | ---: | ---: |\n")
			for _, root := range diagnostics.Index.TopRoots {
				builder.WriteString(fmt.Sprintf("| `%s` | %s | %s | %s |\n", statusMarkdownCell(root.Path), formatFileSearchCount(root.Docs), formatFileSearchCount(root.BigramRows), formatFileSearchCount(root.TotalBytesEstimate)))
			}
		}
	}
	builder.WriteString("\n")

	builder.WriteString("## Roots\n\n")
	writeFileSearchRootTable(&builder, diagnostics.Roots)
	return builder.String()
}

func writeFileSearchRootTable(builder *strings.Builder, roots []filesearch.RootDiagnostic) {
	builder.WriteString("| kind | status | feed | state | progress | cursor | last_reconcile | last_full | dynamic_parent | path | last_error |\n")
	builder.WriteString("| --- | --- | --- | --- | ---: | --- | --- | --- | --- | --- | --- |\n")
	limit := len(roots)
	if limit > maxFileSearchStatusRootRows {
		limit = maxFileSearchStatusRootRows
	}
	for _, root := range roots[:limit] {
		progress := "-"
		if root.ProgressTotal > 0 || root.ProgressCurrent > 0 {
			progress = fmt.Sprintf("%d/%d", root.ProgressCurrent, root.ProgressTotal)
		}
		builder.WriteString(fmt.Sprintf(
			"| `%s` | `%s` | `%s` | `%s` | %s | `%s` | `%s` | `%s` | `%s` | `%s` | `%s` |\n",
			root.Kind,
			root.Status,
			root.FeedType,
			root.FeedState,
			progress,
			statusMarkdownCell(root.FeedCursor),
			formatUnixMillis(root.LastReconcileAt),
			formatUnixMillis(root.LastFullScanAt),
			statusMarkdownCell(root.DynamicParentRootID),
			statusMarkdownCell(root.Path),
			statusMarkdownCell(root.LastError),
		))
	}
	if len(roots) > limit {
		builder.WriteString(fmt.Sprintf("\nShowing %d of %d roots.\n", limit, len(roots)))
	}
}

func formatFileSearchIndexCountSummary(index filesearch.IndexDiagnosticSnapshot) string {
	if index.Error != "" {
		if !index.CountsAvailable {
			return fmt.Sprintf("error=%s", statusMarkdownCell(index.Error))
		}
		return fmt.Sprintf("%s snapshot_error=%s", formatFileSearchIndexVolume(index), statusMarkdownCell(index.Error))
	}
	return formatFileSearchIndexVolume(index)
}

func formatFileSearchIndexVolume(index filesearch.IndexDiagnosticSnapshot) string {
	directoryCount := index.EntryCount - index.FileCount
	if directoryCount < 0 {
		directoryCount = 0
	}
	return fmt.Sprintf(
		"files=%s entries=%s dirs=%s roots=%d",
		formatFileSearchCount(index.FileCount),
		formatFileSearchCount(index.EntryCount),
		formatFileSearchCount(directoryCount),
		index.RootCount,
	)
}

func formatRootKindCounts(counts map[filesearch.RootKind]int) string {
	return formatStatusCountMap([]string{string(filesearch.RootKindDefault), string(filesearch.RootKindUser), string(filesearch.RootKindDynamic)}, func(key string) int {
		return counts[filesearch.RootKind(key)]
	})
}

func formatRootStatusCounts(counts map[filesearch.RootStatus]int) string {
	return formatStatusCountMap([]string{
		string(filesearch.RootStatusPreparing),
		string(filesearch.RootStatusScanning),
		string(filesearch.RootStatusSyncing),
		string(filesearch.RootStatusWriting),
		string(filesearch.RootStatusFinalizing),
		string(filesearch.RootStatusIdle),
		string(filesearch.RootStatusError),
	}, func(key string) int {
		return counts[filesearch.RootStatus(key)]
	})
}

func formatRootFeedTypeCounts(counts map[filesearch.RootFeedType]int) string {
	return formatStatusCountMap([]string{string(filesearch.RootFeedTypeFSEvents), string(filesearch.RootFeedTypeUSN), string(filesearch.RootFeedTypeFallback)}, func(key string) int {
		return counts[filesearch.RootFeedType(key)]
	})
}

func formatRootFeedStateCounts(counts map[filesearch.RootFeedState]int) string {
	return formatStatusCountMap([]string{string(filesearch.RootFeedStateReady), string(filesearch.RootFeedStateDegraded), string(filesearch.RootFeedStateUnavailable)}, func(key string) int {
		return counts[filesearch.RootFeedState(key)]
	})
}

func formatStatusCountMap(order []string, lookup func(string) int) string {
	parts := make([]string, 0, len(order))
	for _, key := range order {
		parts = append(parts, fmt.Sprintf("%s=%d", key, lookup(key)))
	}
	return strings.Join(parts, ", ")
}

func formatSignalAge(signalAt time.Time, capturedAt time.Time) string {
	if signalAt.IsZero() {
		return "-"
	}
	if capturedAt.IsZero() {
		capturedAt = time.Now()
	}
	return formatFileSearchStatusDuration(capturedAt.Sub(signalAt))
}

func formatFileSearchStatusDuration(duration time.Duration) string {
	if duration < 0 {
		duration = 0
	}
	if duration == 0 {
		return "0ms"
	}
	return duration.Truncate(time.Millisecond).String()
}

func statusMarkdownCell(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "|", "\\|")
	value = strings.ReplaceAll(value, "`", "'")
	return value
}

func formatUnixMillis(value int64) string {
	if value <= 0 {
		return "-"
	}
	return time.UnixMilli(value).Format(time.RFC3339)
}

func emptyStatusValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	return value
}
