package filesearch

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"wox/util"
)

const (
	maxLoggedPaths                                = 8
	maxLoggedRoots                                = 5
	slowFilesearchRunPreparationThresholdMs int64 = 250
	slowFilesearchRunExecutionThresholdMs   int64 = 500
	slowFilesearchJobPhaseThresholdMs       int64 = 150
	slowFilesearchSQLiteMaintenanceMs       int64 = 250
	slowFilesearchScanDiagnosticMs          int64 = 250
)

// Diagnostic logging is dev-only because several File Search traces intentionally
// read full SQLite index state; production should keep only the core warning and
// status paths instead of paying for investigation artifacts.
var fileSearchDiagnosticLoggingEnabled = false

func shouldCollectFileSearchDiagnosticSnapshot() bool {
	// Optimization: diagnostic snapshots create fts5vocab tables and scan SQLite
	// state, so the guard must run before callers start goroutines or query the DB.
	// Dev mode still gets the detailed traces needed for local tuning, while
	// production avoids paying for logging-only snapshot work.
	return fileSearchDiagnosticLoggingEnabled
}

func summarizeLogPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "<empty>"
	}
	return path
}

func summarizeLogPaths(paths []string) string {
	if len(paths) == 0 {
		return "[]"
	}

	limit := len(paths)
	if limit > maxLoggedPaths {
		limit = maxLoggedPaths
	}

	visible := make([]string, 0, limit)
	for _, path := range paths[:limit] {
		visible = append(visible, summarizeLogPath(path))
	}

	if len(paths) <= limit {
		return "[" + strings.Join(visible, ", ") + "]"
	}

	return fmt.Sprintf("[%s, ... +%d more]", strings.Join(visible, ", "), len(paths)-limit)
}

func summarizeDirtySignal(signal DirtySignal) string {
	return fmt.Sprintf(
		"kind=%s root=%s trace=%s path=%s path_is_dir=%t path_type_known=%t",
		signal.Kind,
		signal.RootID,
		strings.TrimSpace(signal.TraceID),
		summarizeLogPath(signal.Path),
		signal.PathIsDir,
		signal.PathTypeKnown,
	)
}

func contextWithTraceID(ctx context.Context, traceID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	traceID = strings.TrimSpace(traceID)
	if traceID == "" {
		return ctx
	}
	if util.GetContextTraceId(ctx) == traceID {
		return ctx
	}
	return util.NewTraceContextWith(traceID)
}

func logSQLiteIndexSnapshot(ctx context.Context, stage string, snapshot sqliteIndexSnapshot, info bool) {
	if !fileSearchDiagnosticLoggingEnabled {
		return
	}

	// Diagnostic switch: file indexing performance now needs concrete phase
	// evidence instead of coarse start/end logs. Keeping every expensive trace
	// behind one constant lets us ship a noisy investigation build and turn it
	// off later without hunting through scanner, executor, and SQLite code paths.
	summary := formatSQLiteIndexSnapshotSummary(stage, snapshot)
	topRoots := formatSQLiteIndexTopRoots(stage, snapshot)
	if info {
		util.GetLogger().Info(ctx, summary)
		if topRoots != "" {
			util.GetLogger().Info(ctx, topRoots)
		}
		return
	}

	util.GetLogger().Debug(ctx, summary)
	if topRoots != "" {
		util.GetLogger().Debug(ctx, topRoots)
	}
}

func logFilesearchRunStage(ctx context.Context, kind RunKind, stage RunStage, root RootRecord, job Job, rootIndex int, rootTotal int, current int64, total int64) {
	if !fileSearchDiagnosticLoggingEnabled {
		return
	}

	msg := fmt.Sprintf(
		"filesearch run stage: kind=%s stage=%s root=%s root_path=%s root_index=%d/%d job=%s job_kind=%s scope=%s progress=%d/%d",
		kind,
		stage,
		root.ID,
		summarizeLogPath(root.Path),
		rootIndex,
		rootTotal,
		strings.TrimSpace(job.JobID),
		job.Kind,
		summarizeLogPath(job.ScopePath),
		current,
		total,
	)

	switch stage {
	case RunStagePlanning, RunStageFinalizing:
		util.GetLogger().Info(ctx, msg)
	default:
		util.GetLogger().Debug(ctx, msg)
	}
}

func logFilesearchRunPreparation(ctx context.Context, kind RunKind, elapsedMs int64, rootCount int, jobCount int, totalUnits int64) {
	if !fileSearchDiagnosticLoggingEnabled {
		return
	}

	msg := fmt.Sprintf(
		"filesearch run preparation: kind=%s elapsed=%dms roots=%d jobs=%d total_units=%d",
		kind,
		elapsedMs,
		rootCount,
		jobCount,
		totalUnits,
	)
	if elapsedMs >= slowFilesearchRunPreparationThresholdMs {
		util.GetLogger().Info(ctx, "filesearch slow run preparation: "+msg)
		return
	}
	util.GetLogger().Debug(ctx, msg)
}

func logFilesearchRunExecution(ctx context.Context, kind RunKind, elapsedMs int64, jobCount int, totalUnits int64) {
	if !fileSearchDiagnosticLoggingEnabled {
		return
	}

	msg := fmt.Sprintf(
		"filesearch run execution: kind=%s elapsed=%dms jobs=%d total_units=%d",
		kind,
		elapsedMs,
		jobCount,
		totalUnits,
	)
	if elapsedMs >= slowFilesearchRunExecutionThresholdMs {
		util.GetLogger().Info(ctx, "filesearch slow run execution: "+msg)
		return
	}
	util.GetLogger().Debug(ctx, msg)
}

func logFilesearchFullIndexTotal(ctx context.Context, reason string, elapsedMs int64, rootCount int, jobCount int, totalUnits int64) {
	if !fileSearchDiagnosticLoggingEnabled {
		return
	}

	msg := fmt.Sprintf(
		"filesearch full index total: reason=%s elapsed=%dms roots=%d jobs=%d total_units=%d",
		strings.TrimSpace(reason),
		elapsedMs,
		rootCount,
		jobCount,
		totalUnits,
	)
	// This metric is the optimization baseline for one complete full index run,
	// so emit it at info level every time diagnostics are enabled instead of
	// hiding it behind the generic slow-log threshold used by phase details.
	util.GetLogger().Info(ctx, msg)
}

func logFilesearchJobPhase(ctx context.Context, root RootRecord, job Job, phase string, elapsedMs int64) {
	if !fileSearchDiagnosticLoggingEnabled {
		return
	}

	msg := fmt.Sprintf(
		"filesearch job phase: phase=%s elapsed=%dms root=%s root_path=%s job=%s job_kind=%s scope=%s units=%d",
		strings.TrimSpace(phase),
		elapsedMs,
		root.ID,
		summarizeLogPath(root.Path),
		strings.TrimSpace(job.JobID),
		job.Kind,
		summarizeLogPath(job.ScopePath),
		job.PlannedTotalUnits,
	)
	if elapsedMs >= slowFilesearchJobPhaseThresholdMs {
		util.GetLogger().Info(ctx, "filesearch slow job phase: "+msg)
		return
	}
	util.GetLogger().Debug(ctx, msg)
}

func logFilesearchSQLiteMaintenance(ctx context.Context, operation string, scope string, elapsedMs int64, workCount int) {
	if !fileSearchDiagnosticLoggingEnabled {
		return
	}

	msg := fmt.Sprintf(
		"filesearch sqlite maintenance: operation=%s scope=%s elapsed=%dms work_count=%d",
		strings.TrimSpace(operation),
		summarizeLogPath(scope),
		elapsedMs,
		workCount,
	)
	if elapsedMs >= slowFilesearchSQLiteMaintenanceMs {
		util.GetLogger().Info(ctx, "filesearch slow sqlite maintenance: "+msg)
		return
	}
	util.GetLogger().Debug(ctx, msg)
}

// logFilesearchIndexPhase keeps detailed index timing logs behind diagnostics.
func logFilesearchIndexPhase(ctx context.Context, operation string, detail string, elapsedMs int64, fields map[string]any) {
	if !fileSearchDiagnosticLoggingEnabled {
		return
	}

	parts := []string{
		"filesearch index phase:",
		fmt.Sprintf("operation=%s", strings.TrimSpace(operation)),
		fmt.Sprintf("detail=%s", summarizeLogPath(detail)),
		fmt.Sprintf("elapsed=%dms", elapsedMs),
	}
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%v", key, fields[key]))
	}
	util.GetLogger().Info(ctx, strings.Join(parts, " "))
}

func logFilesearchScanDiagnostic(ctx context.Context, operation string, scope string, elapsedMs int64, workCount int) {
	if !fileSearchDiagnosticLoggingEnabled {
		return
	}

	msg := fmt.Sprintf(
		"filesearch scan diagnostic: operation=%s scope=%s elapsed=%dms work_count=%d",
		strings.TrimSpace(operation),
		summarizeLogPath(scope),
		elapsedMs,
		workCount,
	)
	// Diagnostic addition: stream_scan_build used to be one opaque phase, so a
	// 7s root crawl could not distinguish filesystem reads from policy checks or
	// metadata fallback. Emit the same compact operation shape as SQLite timings
	// so the real-index artifact can rank scan costs beside write costs.
	if elapsedMs >= slowFilesearchScanDiagnosticMs {
		util.GetLogger().Info(ctx, "filesearch slow scan diagnostic: "+msg)
		return
	}
	util.GetLogger().Debug(ctx, msg)
}

func logFilesearchUnreadableSummary(ctx context.Context, scope string, count int64, examples []string) {
	if !fileSearchDiagnosticLoggingEnabled || count <= 0 {
		return
	}

	// Diagnostic fix: real home-root runs can hit many macOS privacy-protected
	// folders. One summary keeps the artifact actionable without filling logs with
	// repeated permission-denied warnings for every protected child directory.
	msg := fmt.Sprintf(
		"filesearch unreadable traversal summary: scope=%s count=%d examples=%s",
		summarizeLogPath(scope),
		count,
		strings.Join(examples, " || "),
	)
	util.GetLogger().Warn(ctx, msg)
}

func formatSQLiteIndexSnapshotSummary(stage string, snapshot sqliteIndexSnapshot) string {
	return fmt.Sprintf(
		"filesearch sqlite snapshot: stage=%s roots=%d entries=%d files=%d bigram_rows=%d name_fts_vocab=%d path_fts_vocab=%d pinyin_full_fts_vocab=%d initials_fts_vocab=%d fact_bytes_est=%d fts_source_bytes_est=%d bigram_bytes_est=%d total_bytes_est=%d db_main_file_bytes=%d db_wal_file_bytes=%d db_shm_file_bytes=%d db_total_file_bytes=%d",
		strings.TrimSpace(stage),
		snapshot.RootCount,
		snapshot.EntryCount,
		snapshot.FileCount,
		snapshot.BigramRowCount,
		snapshot.NameFTSVocab,
		snapshot.PathFTSVocab,
		snapshot.PinyinFullFTSVocab,
		snapshot.InitialsFTSVocab,
		snapshot.FactBytesEstimate,
		snapshot.FTSSourceBytesEstimate,
		snapshot.BigramBytesEstimate,
		snapshot.TotalBytesEstimate,
		snapshot.DBMainFileBytes,
		snapshot.DBWALFileBytes,
		snapshot.DBSHMFileBytes,
		snapshot.DBTotalFileBytes,
	)
}

func formatSQLiteIndexTopRoots(stage string, snapshot sqliteIndexSnapshot) string {
	if len(snapshot.TopRoots) == 0 {
		return ""
	}

	visible := make([]string, 0, min(len(snapshot.TopRoots), maxLoggedRoots))
	for _, root := range snapshot.TopRoots[:min(len(snapshot.TopRoots), maxLoggedRoots)] {
		visible = append(visible, fmt.Sprintf(
			"%s(docs=%d,bigram_rows=%d,total_bytes_est=%d,fact_bytes_est=%d,fts_source_bytes_est=%d,bigram_bytes_est=%d)",
			summarizeLogPath(root.Path),
			root.Docs,
			root.BigramRows,
			root.TotalBytesEstimate,
			root.FactBytesEstimate,
			root.FTSSourceBytesEstimate,
			root.BigramBytesEstimate,
		))
	}
	return fmt.Sprintf(
		"filesearch sqlite snapshot roots: stage=%s top_roots=[%s]",
		strings.TrimSpace(stage),
		strings.Join(visible, ", "),
	)
}
