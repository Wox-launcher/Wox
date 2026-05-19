//go:build filesearch_real_index

package filesearch

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
	"wox/plugin/system/file_search/indexpolicy"
	"wox/util"
)

const (
	actualIndexCaptureEnv       = "WOX_CAPTURE_FILESEARCH_REAL_INDEX"
	actualIndexArtifactPathEnv  = "WOX_FILESEARCH_REAL_INDEX_ARTIFACT_PATH"
	actualIndexRootPathEnv      = "WOX_FILESEARCH_REAL_INDEX_ROOT"
	actualIndexSearchKeywordEnv = "WOX_FILESEARCH_REAL_INDEX_KEYWORD"
	actualIndexFdPathEnv        = "WOX_FILESEARCH_REAL_INDEX_FD"
	actualIndexRgPathEnv        = "WOX_FILESEARCH_REAL_INDEX_RG"
	actualIndexGCFlagsEnv       = "WOX_FILESEARCH_REAL_INDEX_GCFLAGS"
	actualIndexDiagnosticsEnv   = "WOX_FILESEARCH_REAL_INDEX_DIAGNOSTICS"
	actualIndexDefaultRootPath  = "~/Projects"
	actualIndexDefaultKeyword   = "default-cover"
	actualIndexTimeout          = 30 * time.Minute
	actualIndexToolTimeout      = 10 * time.Minute
	actualIndexSearchTimeout    = 1 * time.Minute
	actualIndexSearchLimit      = 50
	actualIndexSearchPreview    = 20
)

var (
	actualIndexCaptureFlag      = flag.Bool("filesearch-real-index", false, "capture a real filesearch index baseline")
	actualIndexArtifactPathFlag = flag.String("filesearch-real-index-artifact", "", "write the real filesearch index baseline artifact to this path")
	actualIndexRootPathFlag     = flag.String("filesearch-real-index-root", "", "root path to capture; defaults to ~/Projects")
	actualIndexKeywordFlag      = flag.String("filesearch-real-index-keyword", "", "file search keyword to query after indexing; defaults to default-cover")
	actualIndexFdPathFlag       = flag.String("filesearch-real-index-fd", "", "fd executable path for the real filesearch baseline")
	actualIndexRgPathFlag       = flag.String("filesearch-real-index-rg", "", "ripgrep executable path for the real filesearch baseline")
)

type realIndexStageMetric struct {
	Stage           string `json:"stage"`
	ElapsedMillis   int64  `json:"elapsed_millis"`
	TransitionCount int    `json:"transition_count"`
}

type realIndexTransition struct {
	OffsetMillis       int64  `json:"offset_millis"`
	Stage              string `json:"stage"`
	RunStatus          string `json:"run_status"`
	ActiveRootPath     string `json:"active_root_path"`
	ActiveScopePath    string `json:"active_scope_path"`
	RunProgressCurrent int64  `json:"run_progress_current"`
	RunProgressTotal   int64  `json:"run_progress_total"`
}

type realIndexRootArtifact struct {
	Path            string `json:"path"`
	Status          string `json:"status"`
	LastFullScanAt  int64  `json:"last_full_scan_at"`
	ProgressCurrent int64  `json:"progress_current"`
	ProgressTotal   int64  `json:"progress_total"`
	DirectoryCount  int    `json:"directory_count"`
	FileCount       int    `json:"file_count"`
	EntryCount      int    `json:"entry_count"`
}

type realIndexIndexedCounts struct {
	DirectoryCount int
	FileCount      int
	EntryCount     int
}

type realIndexPolicyArtifact struct {
	Mode                string   `json:"mode"`
	IgnoredPatternCount int      `json:"ignored_pattern_count"`
	IgnoredPatterns     []string `json:"ignored_patterns"`
	SkipHiddenFiles     bool     `json:"skip_hidden_files"`
	DiagnosticsEnabled  bool     `json:"diagnostics_enabled"`
}

type realIndexArtifact struct {
	CapturedAt         string                   `json:"captured_at"`
	GoGCFlags          string                   `json:"go_gcflags,omitempty"`
	Root               realIndexRootArtifact    `json:"root"`
	IndexPolicy        realIndexPolicyArtifact  `json:"index_policy"`
	FdBaseline         realIndexToolBaseline    `json:"fd_baseline"`
	RgBaseline         realIndexToolBaseline    `json:"rg_baseline"`
	SearchBenchmark    realIndexSearchBenchmark `json:"search_benchmark"`
	TotalElapsedMillis int64                    `json:"total_elapsed_millis"`
	StageMetrics       []realIndexStageMetric   `json:"stage_metrics"`
	Transitions        []realIndexTransition    `json:"transitions"`
	SQLiteSnapshot     string                   `json:"sqlite_snapshot"`
	SQLiteTopRoots     string                   `json:"sqlite_top_roots"`
	ExecutionStats     realIndexExecutionStats  `json:"execution_stats"`
}

type realIndexToolBaseline struct {
	Available     bool     `json:"available"`
	Tool          string   `json:"tool,omitempty"`
	Mode          string   `json:"mode,omitempty"`
	ResultKind    string   `json:"result_kind,omitempty"`
	SearchKeyword string   `json:"search_keyword,omitempty"`
	RunOrder      string   `json:"run_order,omitempty"`
	Executable    string   `json:"executable,omitempty"`
	Args          []string `json:"args,omitempty"`
	RootPath      string   `json:"root_path"`
	StartedAt     string   `json:"started_at,omitempty"`
	ElapsedMillis int64    `json:"elapsed_millis"`
	ResultCount   int      `json:"result_count"`
	StdoutBytes   int64    `json:"stdout_bytes"`
	ExitCode      int      `json:"exit_code,omitempty"`
	Error         string   `json:"error,omitempty"`
	Stderr        string   `json:"stderr,omitempty"`
}

type realIndexSearchBenchmark struct {
	Keyword             string                  `json:"keyword"`
	Limit               int                     `json:"limit"`
	StartedAt           string                  `json:"started_at"`
	SearchStartedAt     string                  `json:"search_started_at"`
	ElapsedMillis       int64                   `json:"elapsed_millis"`
	IndexElapsedMillis  int64                   `json:"index_elapsed_millis"`
	SearchElapsedMillis int64                   `json:"search_elapsed_millis"`
	ResultCount         int                     `json:"result_count"`
	ResultPreview       []realIndexSearchResult `json:"result_preview"`
	Error               string                  `json:"error,omitempty"`
}

type realIndexSearchResult struct {
	Path       string `json:"path"`
	Name       string `json:"name"`
	ParentPath string `json:"parent_path"`
	IsDir      bool   `json:"is_dir"`
	Mtime      int64  `json:"mtime"`
	Size       int64  `json:"size"`
	Score      int64  `json:"score"`
}

type realIndexExecutionStats struct {
	JobCount                   int                             `json:"job_count"`
	SubtreeApplyTotalP50Millis int64                           `json:"subtree_apply_total_p50_millis"`
	SubtreeApplyTotalP95Millis int64                           `json:"subtree_apply_total_p95_millis"`
	StreamApplyP50Millis       int64                           `json:"stream_apply_p50_millis"`
	StreamApplyP95Millis       int64                           `json:"stream_apply_p95_millis"`
	ApplySnapshotP50Millis     int64                           `json:"apply_snapshot_p50_millis"`
	ApplySnapshotP95Millis     int64                           `json:"apply_snapshot_p95_millis"`
	OperationMetrics           []realIndexOperation            `json:"operation_metrics"`
	SlowestScopes              []realIndexSlowScope            `json:"slowest_scopes"`
	ScopeMetrics               []realIndexScopeMetric          `json:"scope_metrics"`
	Unreadable                 realIndexUnreadableSummary      `json:"unreadable"`
	PolicyDiagnostics          indexpolicy.DiagnosticsSnapshot `json:"policy_diagnostics"`
}

type realIndexOperation struct {
	Name              string `json:"name"`
	Count             int    `json:"count"`
	TotalMillis       int64  `json:"total_millis"`
	P50Millis         int64  `json:"p50_millis"`
	P95Millis         int64  `json:"p95_millis"`
	MaxMillis         int64  `json:"max_millis"`
	TotalWorkCount    int64  `json:"total_work_count,omitempty"`
	MaxWorkCount      int    `json:"max_work_count,omitempty"`
	AverageWorkMillis int64  `json:"average_work_millis,omitempty"`
}

type realIndexSlowScope struct {
	Scope         string `json:"scope"`
	ElapsedMillis int64  `json:"elapsed_millis"`
}

type realIndexScopeMetric struct {
	Scope            string `json:"scope"`
	ElapsedMillis    int64  `json:"elapsed_millis"`
	EntryCount       int    `json:"entry_count,omitempty"`
	DirectoryCount   int    `json:"directory_count,omitempty"`
	UnreadableCount  int64  `json:"unreadable_count,omitempty"`
	OperationSamples int    `json:"operation_samples,omitempty"`
}

type realIndexUnreadableSummary struct {
	Count    int64    `json:"count"`
	Examples []string `json:"examples,omitempty"`
}

type realIndexTimelineEvent struct {
	recordedAt         time.Time
	stage              RunStage
	runStatus          RunStatus
	activeRootPath     string
	activeScopePath    string
	runProgressCurrent int64
	runProgressTotal   int64
}

var (
	realIndexJobPhasePattern          = regexp.MustCompile(`filesearch job phase: phase=([^ ]+) elapsed=(\d+)ms .* scope=(.+) units=\d+$`)
	realIndexSQLiteMaintenancePattern = regexp.MustCompile(`filesearch sqlite maintenance: operation=([^ ]+) scope=(.+) elapsed=(\d+)ms work_count=(\d+)$`)
	realIndexScanDiagnosticPattern    = regexp.MustCompile(`filesearch scan diagnostic: operation=([^ ]+) scope=(.+) elapsed=(\d+)ms work_count=(\d+)$`)
	realIndexUnreadablePattern        = regexp.MustCompile(`filesearch unreadable traversal summary: scope=(.+) count=(\d+) examples=(.*)$`)
)

func TestSummarizeRealIndexExecutionLog(t *testing.T) {
	logContent := strings.Join([]string{
		"2026-04-23 19:14:16.293 G0000008 [DBG] [Wox] filesearch sqlite maintenance: operation=subtree_apply_total scope=batches=1 elapsed=19ms work_count=1",
		"2026-04-23 19:14:16.293 G0000008 [DBG] [Wox] filesearch job phase: phase=apply_snapshot elapsed=19ms root=real-index-c-dev root_path=C:\\dev job=job-1 job_kind=subtree scope=C:\\dev\\scope-a units=64",
		"2026-04-23 19:14:16.309 G0000008 [DBG] [Wox] filesearch sqlite maintenance: operation=subtree_apply_total scope=batches=1 elapsed=15ms work_count=1",
		"2026-04-23 19:14:16.309 G0000008 [DBG] [Wox] filesearch job phase: phase=apply_snapshot elapsed=15ms root=real-index-c-dev root_path=C:\\dev job=job-2 job_kind=subtree scope=C:\\dev\\scope-b units=16",
		"2026-04-23 19:14:16.329 G0000008 [DBG] [Wox] filesearch sqlite maintenance: operation=subtree_apply_total scope=batches=1 elapsed=27ms work_count=1",
		"2026-04-23 19:14:16.329 G0000008 [DBG] [Wox] filesearch job phase: phase=apply_snapshot elapsed=27ms root=real-index-c-dev root_path=C:\\dev job=job-3 job_kind=subtree scope=C:\\dev\\scope-c units=18",
		"2026-04-23 19:14:16.349 G0000008 [DBG] [Wox] filesearch job phase: phase=apply_snapshot elapsed=20ms root=real-index-c-dev root_path=C:\\dev job=job-4 job_kind=subtree scope=C:\\dev\\scope-a units=36",
	}, "\n")

	stats := summarizeRealIndexExecutionLog(logContent)
	if stats.JobCount != 4 {
		t.Fatalf("expected 4 jobs, got %d", stats.JobCount)
	}
	if stats.SubtreeApplyTotalP50Millis != 19 {
		t.Fatalf("expected subtree apply p50 19ms, got %d", stats.SubtreeApplyTotalP50Millis)
	}
	if stats.SubtreeApplyTotalP95Millis != 27 {
		t.Fatalf("expected subtree apply p95 27ms, got %d", stats.SubtreeApplyTotalP95Millis)
	}
	if stats.ApplySnapshotP50Millis != 20 {
		t.Fatalf("expected apply snapshot p50 20ms, got %d", stats.ApplySnapshotP50Millis)
	}
	if stats.ApplySnapshotP95Millis != 27 {
		t.Fatalf("expected apply snapshot p95 27ms, got %d", stats.ApplySnapshotP95Millis)
	}
	if len(stats.OperationMetrics) == 0 {
		t.Fatal("expected operation metrics")
	}
	if len(stats.SlowestScopes) != 3 {
		t.Fatalf("expected 3 unique slow scopes, got %d", len(stats.SlowestScopes))
	}
	if stats.SlowestScopes[0].Scope != `C:\dev\scope-c` || stats.SlowestScopes[0].ElapsedMillis != 27 {
		t.Fatalf("expected slowest scope C:\\dev\\scope-c at 27ms, got %#v", stats.SlowestScopes[0])
	}
	if stats.SlowestScopes[1].Scope != `C:\dev\scope-a` || stats.SlowestScopes[1].ElapsedMillis != 20 {
		t.Fatalf("expected second scope C:\\dev\\scope-a at 20ms, got %#v", stats.SlowestScopes[1])
	}
}

func TestCaptureFileSearchRealIndexForProjectsRoot(t *testing.T) {
	// Feature addition: this benchmark now requires both the build tag and an
	// explicit runtime opt-in, so CI and normal package tests never compile or
	// run a workstation-sized crawl by accident.
	if !shouldCaptureRealIndex() {
		t.Skip("run `make filesearch-real-index` from wox.core, or run with -tags filesearch_real_index and pass -args -filesearch-real-index to capture the real filesearch indexing baseline")
	}

	rootPath := realIndexRootPath()
	rootInfo, err := os.Stat(rootPath)
	if err != nil {
		t.Skipf("skip real index capture because %q is unavailable: %v", rootPath, err)
	}
	if !rootInfo.IsDir() {
		t.Skipf("skip real index capture because %q is not a directory", rootPath)
	}

	db, baseCtx := openTestFileSearchDB(t)
	scanCtx, cancel := context.WithTimeout(baseCtx, actualIndexTimeout)
	defer cancel()
	t.Logf("real index test log directory: %s", util.GetLocation().GetLogDirectory())

	rootPath = filepath.Clean(rootPath)
	now := time.Now().UnixMilli()
	root := RootRecord{
		ID:        "real-index-root",
		Path:      rootPath,
		Kind:      RootKindUser,
		Status:    RootStatusIdle,
		CreatedAt: now,
		UpdatedAt: now,
	}
	mustInsertRoot(t, scanCtx, db, root)

	scanner := NewScanner(db)
	indexPolicy, indexPolicyArtifact, policyDiagnostics := realIndexBenchmarkPolicy()
	scanner.policy.Set(indexPolicy)
	engine := &Engine{db: db, scanner: scanner}
	previousDiagnosticLoggingEnabled := fileSearchDiagnosticLoggingEnabled
	if shouldEnableRealIndexDiagnostics() {
		// Diagnostic addition: real-index captures need the existing stage and
		// SQLite timing logs, but normal package tests should stay quiet. The env
		// switch keeps the production default unchanged while making local
		// optimization runs explain where index time is being spent.
		fileSearchDiagnosticLoggingEnabled = true
	}
	defer func() {
		fileSearchDiagnosticLoggingEnabled = previousDiagnosticLoggingEnabled
	}()

	var (
		timelineMu sync.Mutex
		timeline   []realIndexTimelineEvent
		lastKey    string
	)

	// Temp-dir fixtures keep full-scan tests deterministic, but they do not
	// expose the planner churn, SQLite write volume, and stage balance of a real
	// workstation-sized tree. This opt-in test records those production-shaped
	// costs without forcing CI or routine local runs to index ~/Projects.
	scanner.SetStateChangeHandler(func(changeCtx context.Context) {
		status, err := engine.GetStatus(changeCtx)
		if err != nil {
			t.Fatalf("get status during real index capture: %v", err)
		}

		event := realIndexTimelineEvent{
			recordedAt:         time.Now(),
			stage:              status.ActiveStage,
			runStatus:          status.ActiveRunStatus,
			activeRootPath:     status.ActiveRootPath,
			activeScopePath:    status.ActiveScopePath,
			runProgressCurrent: status.RunProgressCurrent,
			runProgressTotal:   status.RunProgressTotal,
		}

		key := fmt.Sprintf(
			"%s|%s|%s|%s",
			event.stage,
			event.runStatus,
			strings.TrimSpace(event.activeRootPath),
			strings.TrimSpace(event.activeScopePath),
		)

		timelineMu.Lock()
		if key == lastKey {
			timelineMu.Unlock()
			return
		}
		lastKey = key
		timeline = append(timeline, event)
		timelineMu.Unlock()
	})

	searchKeyword := realIndexSearchKeyword()
	// Diagnostic correction: run Wox before fd/rg so the comparison does not let
	// external walkers pre-warm the filesystem cache. The previous order made the
	// benchmark much faster than an app-triggered full index and hid the cold
	// traversal cost that users actually feel.
	scanStartedAt := time.Now()
	scanner.scanAllRoots(scanCtx)
	scanFinishedAt := time.Now()

	// Feature addition: release baseline runs are usually read from `go test -v`
	// output, not from the JSON artifact. Count the final persisted index before
	// printing the Wox baseline so the one-line comparison includes the total
	// indexed file/entry volume that explains how large the scan actually was.
	rootArtifact := captureRealIndexRootArtifact(t, scanCtx, db, root.ID)
	searchBenchmark := captureRealIndexSearchBenchmark(t, baseCtx, db, searchKeyword, scanStartedAt, scanFinishedAt, realIndexIndexedCounts{
		DirectoryCount: rootArtifact.DirectoryCount,
		FileCount:      rootArtifact.FileCount,
		EntryCount:     rootArtifact.EntryCount,
	})

	// External baselines still belong in the artifact, but after Wox has already
	// paid the real index cost. Their run_order explicitly records that they are
	// lookup references, not cache warmers for the Wox scan.
	fdBaseline := captureRealIndexFdBaseline(t, baseCtx, rootPath, searchKeyword)
	rgBaseline := captureRealIndexRgBaseline(t, baseCtx, rootPath, searchKeyword)

	sqliteSnapshot, err := db.SearchIndexSnapshot(scanCtx)
	if err != nil {
		t.Fatalf("capture sqlite snapshot after real index capture: %v", err)
	}
	executionStats := loadRealIndexExecutionStats(t)
	executionStats.PolicyDiagnostics = policyDiagnostics.Snapshot()

	timelineMu.Lock()
	artifact := realIndexArtifact{
		CapturedAt:         time.Now().UTC().Format(time.RFC3339),
		GoGCFlags:          strings.TrimSpace(os.Getenv(actualIndexGCFlagsEnv)),
		Root:               rootArtifact,
		IndexPolicy:        indexPolicyArtifact,
		FdBaseline:         fdBaseline,
		RgBaseline:         rgBaseline,
		SearchBenchmark:    searchBenchmark,
		TotalElapsedMillis: searchBenchmark.ElapsedMillis,
		StageMetrics:       buildRealIndexStageMetrics(timeline, scanFinishedAt),
		Transitions:        buildRealIndexTransitions(timeline, scanStartedAt),
		SQLiteSnapshot:     formatSQLiteIndexSnapshotSummary("actual_root", sqliteSnapshot),
		SQLiteTopRoots:     formatSQLiteIndexTopRoots("actual_root", sqliteSnapshot),
		ExecutionStats:     executionStats,
	}
	timelineMu.Unlock()

	writeRealIndexArtifact(t, artifact)
}

func captureRealIndexRootArtifact(t *testing.T, ctx context.Context, db *FileSearchDB, rootID string) realIndexRootArtifact {
	t.Helper()

	rootAfter, err := db.FindRootByID(ctx, rootID)
	if err != nil {
		t.Fatalf("find root after real index capture: %v", err)
	}
	if rootAfter == nil {
		t.Fatalf("expected captured root %q to remain persisted", rootID)
	}

	directoryCount, err := db.CountDirectoriesByRoot(ctx, rootID)
	if err != nil {
		t.Fatalf("count directories after real index capture: %v", err)
	}
	// Use SQLite aggregate counts instead of loading every entry row. Large
	// release baselines should print index volume without adding a second
	// memory-heavy walk over the captured result set.
	fileCount, entryCount, err := db.SearchIndexCounts(ctx)
	if err != nil {
		t.Fatalf("count indexed entries after real index capture: %v", err)
	}

	return realIndexRootArtifact{
		Path:            rootAfter.Path,
		Status:          string(rootAfter.Status),
		LastFullScanAt:  rootAfter.LastFullScanAt,
		ProgressCurrent: rootAfter.ProgressCurrent,
		ProgressTotal:   rootAfter.ProgressTotal,
		DirectoryCount:  directoryCount,
		FileCount:       int(fileCount),
		EntryCount:      int(entryCount),
	}
}

func realIndexBenchmarkPolicy() (Policy, realIndexPolicyArtifact, *indexpolicy.Diagnostics) {
	pluginPolicy := indexpolicy.New()
	diagnostics := indexpolicy.NewDiagnostics()
	pluginPolicy.SetDiagnostics(diagnostics)

	newTraversalContext := func(root RootRecord, scopePath string) TraversalPolicyContext {
		// Benchmark alignment: production full indexing carries ignore state
		// through traversal, so the benchmark exposes that path directly instead
		// of keeping a second per-path matcher alive.
		return realIndexTraversalPolicyContext{
			inner:           pluginPolicy.NewTraversalContext(root.Path, root.PolicyRootPath, scopePath),
			skipHiddenFiles: true,
		}
	}

	ignoredPatterns := indexpolicy.DefaultIgnorePatterns()
	sort.Strings(ignoredPatterns)
	policy := Policy{
		NewTraversalContext: newTraversalContext,
		ShouldProcessChange: func(root RootRecord, change ChangeSignal) bool {
			cleanPath := filepath.Clean(strings.TrimSpace(change.Path))
			if cleanPath == "" || cleanPath == "." || cleanPath == filepath.Clean(root.Path) {
				return true
			}
			isDir := change.PathIsDir
			if !change.PathTypeKnown {
				if info, err := os.Stat(cleanPath); err == nil {
					isDir = info.IsDir()
				}
			}
			return newTraversalContext(root, filepath.Dir(cleanPath)).ShouldIndexPath(cleanPath, isDir)
		},
	}
	artifact := realIndexPolicyArtifact{
		Mode:                "plugin-default-policy",
		IgnoredPatternCount: len(ignoredPatterns),
		IgnoredPatterns:     ignoredPatterns,
		SkipHiddenFiles:     true,
		DiagnosticsEnabled:  true,
	}
	return policy, artifact, diagnostics
}

type realIndexTraversalPolicyContext struct {
	inner           *indexpolicy.TraversalContext
	skipHiddenFiles bool
}

func (c realIndexTraversalPolicyContext) ShouldIndexPath(path string, isDir bool) bool {
	if c.inner == nil {
		return true
	}
	if c.skipHiddenFiles && isHiddenRealIndexPath(path) {
		return false
	}
	return c.inner.ShouldIndexPath(path, isDir)
}

func (c realIndexTraversalPolicyContext) Descend(directoryPath string) TraversalPolicyContext {
	if c.inner == nil {
		return c
	}
	// Benchmark adapter mirrors the production plugin wrapper so the real-index
	// capture exercises the same traversal policy contract as the app.
	return realIndexTraversalPolicyContext{
		inner:           c.inner.Descend(directoryPath),
		skipHiddenFiles: c.skipHiddenFiles,
	}
}

func (c realIndexTraversalPolicyContext) WithDirectoryEntries(directoryPath string, entries []os.DirEntry) TraversalPolicyContext {
	if c.inner == nil {
		return c
	}
	// Benchmark alignment: real-index captures use the same directory-entry hint
	// path as the production plugin wrapper, so .gitignore load reductions are
	// measured in the workstation-sized benchmark instead of only in app runs.
	return realIndexTraversalPolicyContext{
		inner:           c.inner.WithDirectoryEntries(directoryPath, entries),
		skipHiddenFiles: c.skipHiddenFiles,
	}
}

func isHiddenRealIndexPath(path string) bool {
	name := filepath.Base(filepath.Clean(strings.TrimSpace(path)))
	return strings.HasPrefix(name, ".") && name != "." && name != ".."
}

func shouldCaptureRealIndex() bool {
	if *actualIndexCaptureFlag {
		return true
	}

	// WSL-launched Windows `go.exe` runs the test binary on the Windows side, but
	// the shell-to-process environment bridge can drop opt-in test variables.
	// Accepting a `go test -args` flag keeps the real-index capture runnable from
	// either shell without weakening the default skip guard for CI.
	return strings.TrimSpace(os.Getenv(actualIndexCaptureEnv)) == "1"
}

func shouldEnableRealIndexDiagnostics() bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(actualIndexDiagnosticsEnv)))
	return value == "1" || value == "true" || value == "yes" || value == "on"
}

func realIndexRootPath() string {
	if path := strings.TrimSpace(*actualIndexRootPathFlag); path != "" {
		return filepath.Clean(expandRealIndexRootPath(path))
	}
	if path := strings.TrimSpace(os.Getenv(actualIndexRootPathEnv)); path != "" {
		return filepath.Clean(expandRealIndexRootPath(path))
	}
	return filepath.Clean(expandRealIndexRootPath(actualIndexDefaultRootPath))
}

func realIndexSearchKeyword() string {
	if keyword := strings.TrimSpace(*actualIndexKeywordFlag); keyword != "" {
		return keyword
	}
	if keyword := strings.TrimSpace(os.Getenv(actualIndexSearchKeywordEnv)); keyword != "" {
		return keyword
	}
	return actualIndexDefaultKeyword
}

func expandRealIndexRootPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "~" || strings.HasPrefix(path, "~/") || strings.HasPrefix(path, `~\`) {
		// Go does not expand shell-style "~" paths before os.Stat, so the local
		// default is normalized here while still allowing absolute CI-free roots.
		home, err := os.UserHomeDir()
		if err == nil && strings.TrimSpace(home) != "" {
			if path == "~" {
				return home
			}
			return filepath.Join(home, strings.TrimLeft(path[2:], `/\`))
		}
	}
	return path
}

func realIndexToolExecutable(flagValue *string, envName string, binaryNames ...string) (string, error) {
	// External traversal tools can be pinned for repeatable baseline captures
	// across shells. Falling back to PATH keeps the common local workflow one
	// flag/env lighter while still recording the resolved executable.
	if flagValue != nil {
		if path := strings.TrimSpace(*flagValue); path != "" {
			return path, nil
		}
	}
	if path := strings.TrimSpace(os.Getenv(envName)); path != "" {
		return path, nil
	}
	var lastErr error
	for _, binaryName := range binaryNames {
		if strings.TrimSpace(binaryName) == "" {
			continue
		}
		path, err := exec.LookPath(binaryName)
		if err == nil {
			return path, nil
		}
		lastErr = err
	}
	return "", lastErr
}

func captureRealIndexSearchBenchmark(t *testing.T, parentCtx context.Context, db *FileSearchDB, keyword string, indexStartedAt time.Time, indexFinishedAt time.Time, indexCounts realIndexIndexedCounts) realIndexSearchBenchmark {
	t.Helper()

	benchmark := realIndexSearchBenchmark{
		Keyword:            strings.TrimSpace(keyword),
		Limit:              actualIndexSearchLimit,
		StartedAt:          indexStartedAt.UTC().Format(time.RFC3339),
		IndexElapsedMillis: indexFinishedAt.Sub(indexStartedAt).Milliseconds(),
	}
	if benchmark.Keyword == "" {
		benchmark.Error = "empty search keyword"
		benchmark.ElapsedMillis = benchmark.IndexElapsedMillis
		return benchmark
	}

	ctx, cancel := context.WithTimeout(parentCtx, actualIndexSearchTimeout)
	defer cancel()

	// Bug fix: the fd/rg comparison needs Wox's user-visible end-to-end cost.
	// The old elapsed field timed only the final SQLite query, which could report
	// 0ms and hide the real indexing cost. Keep search_elapsed_millis as the
	// query-only breakdown, but make elapsed_millis mean index + search.
	startedAt := time.Now()
	benchmark.SearchStartedAt = startedAt.UTC().Format(time.RFC3339)
	results, err := NewSQLiteSearchProvider(db).Search(ctx, SearchQuery{Raw: benchmark.Keyword}, actualIndexSearchLimit)
	benchmark.SearchElapsedMillis = time.Since(startedAt).Milliseconds()
	benchmark.ElapsedMillis = benchmark.IndexElapsedMillis + benchmark.SearchElapsedMillis
	if err != nil {
		benchmark.Error = err.Error()
	} else if ctx.Err() != nil {
		benchmark.Error = ctx.Err().Error()
	}
	benchmark.ResultCount = len(results)
	benchmark.ResultPreview = buildRealIndexSearchPreview(results)

	t.Logf(
		"wox baseline: keyword=%q results=%d limit=%d elapsed=%dms index_elapsed=%dms search_elapsed=%dms indexed_files=%d indexed_entries=%d indexed_dirs=%d error=%q",
		benchmark.Keyword,
		benchmark.ResultCount,
		benchmark.Limit,
		benchmark.ElapsedMillis,
		benchmark.IndexElapsedMillis,
		benchmark.SearchElapsedMillis,
		indexCounts.FileCount,
		indexCounts.EntryCount,
		indexCounts.DirectoryCount,
		benchmark.Error,
	)
	return benchmark
}

func buildRealIndexSearchPreview(results []SearchResult) []realIndexSearchResult {
	limit := actualIndexSearchPreview
	if len(results) < limit {
		limit = len(results)
	}
	preview := make([]realIndexSearchResult, 0, limit)
	for _, result := range results[:limit] {
		preview = append(preview, realIndexSearchResult{
			Path:       result.Path,
			Name:       result.Name,
			ParentPath: result.ParentPath,
			IsDir:      result.IsDir,
			Mtime:      result.Mtime,
			Size:       result.Size,
			Score:      result.Score,
		})
	}
	return preview
}

func captureRealIndexFdBaseline(t *testing.T, parentCtx context.Context, rootPath string, keyword string) realIndexToolBaseline {
	t.Helper()

	keyword = strings.TrimSpace(keyword)
	return captureRealIndexToolBaseline(t, parentCtx, realIndexToolCapture{
		Tool:          "fd",
		BinaryNames:   []string{"fd", "fdfind"},
		EnvName:       actualIndexFdPathEnv,
		FlagValue:     actualIndexFdPathFlag,
		Args:          []string{regexp.QuoteMeta(keyword), rootPath},
		RootPath:      rootPath,
		Mode:          "name-regex",
		ResultKind:    "entries",
		SearchKeyword: keyword,
	})
}

func captureRealIndexRgBaseline(t *testing.T, parentCtx context.Context, rootPath string, keyword string) realIndexToolBaseline {
	t.Helper()

	keyword = strings.TrimSpace(keyword)
	return captureRealIndexToolBaseline(t, parentCtx, realIndexToolCapture{
		Tool:          "rg",
		BinaryNames:   []string{"rg"},
		EnvName:       actualIndexRgPathEnv,
		FlagValue:     actualIndexRgPathFlag,
		Args:          []string{"--files", "-g", "*" + realIndexGlobLiteral(keyword) + "*", rootPath},
		RootPath:      rootPath,
		Mode:          "files-glob",
		ResultKind:    "files",
		SearchKeyword: keyword,
	})
}

func realIndexGlobLiteral(value string) string {
	replacer := strings.NewReplacer(
		`\`, `\\`,
		`*`, `\*`,
		`?`, `\?`,
		`[`, `\[`,
		`]`, `\]`,
	)
	return replacer.Replace(value)
}

type realIndexToolCapture struct {
	Tool          string
	BinaryNames   []string
	EnvName       string
	FlagValue     *string
	Args          []string
	RootPath      string
	Mode          string
	ResultKind    string
	SearchKeyword string
	RunOrder      string
}

func captureRealIndexToolBaseline(t *testing.T, parentCtx context.Context, capture realIndexToolCapture) realIndexToolBaseline {
	t.Helper()

	baseline := realIndexToolBaseline{
		Tool:          strings.TrimSpace(capture.Tool),
		RootPath:      filepath.Clean(capture.RootPath),
		Mode:          strings.TrimSpace(capture.Mode),
		ResultKind:    strings.TrimSpace(capture.ResultKind),
		SearchKeyword: strings.TrimSpace(capture.SearchKeyword),
	}
	if baseline.SearchKeyword == "" {
		baseline.Error = "empty search keyword"
		t.Logf("%s baseline unavailable: %v", baseline.Tool, baseline.Error)
		return baseline
	}

	executable, err := realIndexToolExecutable(capture.FlagValue, capture.EnvName, capture.BinaryNames...)
	if err != nil {
		baseline.Error = err.Error()
		t.Logf("%s baseline unavailable: %v", baseline.Tool, err)
		return baseline
	}

	baseline.Available = true
	baseline.RunOrder = strings.TrimSpace(capture.RunOrder)
	if baseline.RunOrder == "" {
		baseline.RunOrder = "after_wox_scan"
	}
	baseline.Executable = executable
	baseline.Args = append([]string(nil), capture.Args...)
	baseline.StartedAt = time.Now().UTC().Format(time.RFC3339)

	ctx, cancel := context.WithTimeout(parentCtx, actualIndexToolTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, executable, capture.Args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		baseline.Error = err.Error()
		return baseline
	}
	stderr := &realIndexLimitedBuffer{limit: 32 << 10}
	cmd.Stderr = stderr

	startedAt := time.Now()
	if err := cmd.Start(); err != nil {
		baseline.Error = err.Error()
		return baseline
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		baseline.ResultCount++
		baseline.StdoutBytes += int64(len(scanner.Bytes()) + 1)
	}
	scanErr := scanner.Err()
	waitErr := cmd.Wait()
	baseline.ElapsedMillis = time.Since(startedAt).Milliseconds()
	baseline.Stderr = stderr.String()

	if waitErr != nil {
		baseline.Error = waitErr.Error()
		if exitErr, ok := waitErr.(*exec.ExitError); ok {
			baseline.ExitCode = exitErr.ExitCode()
		}
	}
	if scanErr != nil {
		if baseline.Error == "" {
			baseline.Error = scanErr.Error()
		} else {
			baseline.Error += "; stdout scan: " + scanErr.Error()
		}
	}
	if ctx.Err() != nil {
		if baseline.Error == "" {
			baseline.Error = ctx.Err().Error()
		} else {
			baseline.Error += "; context: " + ctx.Err().Error()
		}
	}

	t.Logf(
		"%s baseline: root=%s keyword=%q result_kind=%s results=%d elapsed=%dms error=%q",
		baseline.Tool,
		baseline.RootPath,
		baseline.SearchKeyword,
		baseline.ResultKind,
		baseline.ResultCount,
		baseline.ElapsedMillis,
		baseline.Error,
	)
	return baseline
}

type realIndexLimitedBuffer struct {
	limit     int
	truncated bool
	builder   strings.Builder
}

func (b *realIndexLimitedBuffer) Write(data []byte) (int, error) {
	// rg can report many permission or encoding messages on real workstation
	// roots. Keep enough stderr for diagnosis without letting a failed traversal
	// dominate the in-memory test artifact.
	if b == nil {
		return len(data), nil
	}
	if b.limit <= 0 {
		b.truncated = true
		return len(data), nil
	}
	remaining := b.limit - b.builder.Len()
	if remaining <= 0 {
		b.truncated = true
		return len(data), nil
	}
	if len(data) > remaining {
		b.builder.Write(data[:remaining])
		b.truncated = true
		return len(data), nil
	}
	b.builder.Write(data)
	return len(data), nil
}

func (b *realIndexLimitedBuffer) String() string {
	if b == nil {
		return ""
	}
	value := strings.TrimSpace(b.builder.String())
	if b.truncated {
		if value != "" {
			value += "\n"
		}
		value += "<truncated>"
	}
	return value
}

func loadRealIndexExecutionStats(t *testing.T) realIndexExecutionStats {
	t.Helper()

	logPath := filepath.Join(util.GetLocation().GetLogDirectory(), "log")
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read real index log %q: %v", logPath, err)
	}

	// The first version of this real-root baseline only emitted stage timelines
	// and SQLite snapshots, which still left the real hotspot hidden inside the
	// giant debug log. Parsing the existing log file here keeps the feature
	// test-only while turning one long trace into a compact performance summary.
	return summarizeRealIndexExecutionLog(string(content))
}

func buildRealIndexStageMetrics(timeline []realIndexTimelineEvent, scanFinishedAt time.Time) []realIndexStageMetric {
	if len(timeline) == 0 {
		return nil
	}

	stageTotals := map[string]int64{}
	stageTransitions := map[string]int{}
	stageOrder := []string{}

	for index, event := range timeline {
		stage := string(event.stage)
		if strings.TrimSpace(stage) == "" {
			continue
		}
		if _, seen := stageTransitions[stage]; !seen {
			stageOrder = append(stageOrder, stage)
		}
		stageTransitions[stage]++

		nextAt := scanFinishedAt
		if index+1 < len(timeline) {
			nextAt = timeline[index+1].recordedAt
		}
		stageTotals[stage] += nextAt.Sub(event.recordedAt).Milliseconds()
	}

	metrics := make([]realIndexStageMetric, 0, len(stageOrder))
	for _, stage := range stageOrder {
		metrics = append(metrics, realIndexStageMetric{
			Stage:           stage,
			ElapsedMillis:   stageTotals[stage],
			TransitionCount: stageTransitions[stage],
		})
	}

	return metrics
}

func buildRealIndexTransitions(timeline []realIndexTimelineEvent, scanStartedAt time.Time) []realIndexTransition {
	if len(timeline) == 0 {
		return nil
	}

	transitions := make([]realIndexTransition, 0, len(timeline))
	for _, event := range timeline {
		transitions = append(transitions, realIndexTransition{
			OffsetMillis:       event.recordedAt.Sub(scanStartedAt).Milliseconds(),
			Stage:              string(event.stage),
			RunStatus:          string(event.runStatus),
			ActiveRootPath:     event.activeRootPath,
			ActiveScopePath:    event.activeScopePath,
			RunProgressCurrent: event.runProgressCurrent,
			RunProgressTotal:   event.runProgressTotal,
		})
	}
	return transitions
}

func writeRealIndexArtifact(t *testing.T, artifact realIndexArtifact) {
	t.Helper()

	payload, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		t.Fatalf("marshal real index artifact: %v", err)
	}

	// The artifact is machine-specific, so only write a file when the caller asks
	// for one. Logging the JSON by default keeps the test useful for quick local
	// measurements without leaving workstation-specific output in the repo tree.
	artifactPath := realIndexArtifactPath()
	if artifactPath != "" {
		if err := os.MkdirAll(filepath.Dir(artifactPath), 0o755); err != nil {
			t.Fatalf("create real index artifact directory: %v", err)
		}
		if err := os.WriteFile(artifactPath, payload, 0o644); err != nil {
			t.Fatalf("write real index artifact %q: %v", artifactPath, err)
		}
		t.Logf("real index artifact written to %s", artifactPath)
	}

	t.Log(string(payload))
}

func realIndexArtifactPath() string {
	if path := strings.TrimSpace(*actualIndexArtifactPathFlag); path != "" {
		return path
	}
	return strings.TrimSpace(os.Getenv(actualIndexArtifactPathEnv))
}

func summarizeRealIndexExecutionLog(logContent string) realIndexExecutionStats {
	lines := strings.Split(logContent, "\n")
	subtreeApplyTotals := make([]int64, 0)
	streamApplies := make([]int64, 0)
	applySnapshots := make([]int64, 0)
	slowestScopeByPath := map[string]int64{}
	operationSamples := map[string][]int64{}
	operationWorkCounts := map[string][]int{}
	scopeMetricsByPath := map[string]*realIndexScopeMetric{}
	unreadable := realIndexUnreadableSummary{}
	jobPhaseCount := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if matches := realIndexJobPhasePattern.FindStringSubmatch(line); len(matches) == 4 {
			jobPhaseCount++
			phase := strings.TrimSpace(matches[1])
			elapsed := mustParseRealIndexMillis(matches[2])
			scope := strings.TrimSpace(matches[3])
			metricName := "job_phase:" + phase
			operationSamples[metricName] = append(operationSamples[metricName], elapsed)
			if phase == "stream_apply" {
				streamApplies = append(streamApplies, elapsed)
			}
			if phase == "apply_snapshot" {
				applySnapshots = append(applySnapshots, elapsed)
			}
			if elapsed > slowestScopeByPath[scope] {
				slowestScopeByPath[scope] = elapsed
			}
			scopeMetric := ensureRealIndexScopeMetric(scopeMetricsByPath, scope)
			if elapsed > scopeMetric.ElapsedMillis {
				scopeMetric.ElapsedMillis = elapsed
			}
		}

		if matches := realIndexSQLiteMaintenancePattern.FindStringSubmatch(line); len(matches) == 5 {
			operation := strings.TrimSpace(matches[1])
			scope := strings.TrimSpace(matches[2])
			elapsed := mustParseRealIndexMillis(matches[3])
			workCount := mustParseRealIndexInt(matches[4])
			metricName := "sqlite:" + operation
			operationSamples[metricName] = append(operationSamples[metricName], elapsed)
			operationWorkCounts[metricName] = append(operationWorkCounts[metricName], workCount)
			if operation == "subtree_apply_total" {
				subtreeApplyTotals = append(subtreeApplyTotals, elapsed)
			}
			recordRealIndexScopeSQLiteMetric(scopeMetricsByPath, scope, operation, elapsed, workCount)
		}

		if matches := realIndexScanDiagnosticPattern.FindStringSubmatch(line); len(matches) == 5 {
			operation := strings.TrimSpace(matches[1])
			scope := strings.TrimSpace(matches[2])
			elapsed := mustParseRealIndexMillis(matches[3])
			workCount := mustParseRealIndexInt(matches[4])
			metricName := "scan:" + operation
			operationSamples[metricName] = append(operationSamples[metricName], elapsed)
			operationWorkCounts[metricName] = append(operationWorkCounts[metricName], workCount)
			if strings.HasPrefix(operation, "subtree_stream_") {
				ensureRealIndexScopeMetric(scopeMetricsByPath, scope).OperationSamples++
			}
		}

		if matches := realIndexUnreadablePattern.FindStringSubmatch(line); len(matches) == 4 {
			scope := strings.TrimSpace(matches[1])
			count := mustParseRealIndexMillis(matches[2])
			examples := parseRealIndexUnreadableExamples(matches[3])
			unreadable.Count += count
			unreadable.Examples = appendLimitedRealIndexExamples(unreadable.Examples, examples, 20)
			scopeMetric := ensureRealIndexScopeMetric(scopeMetricsByPath, scope)
			scopeMetric.UnreadableCount += count
		}
	}

	stats := realIndexExecutionStats{
		JobCount:                   jobPhaseCount,
		SubtreeApplyTotalP50Millis: percentileMillis(subtreeApplyTotals, 0.50),
		SubtreeApplyTotalP95Millis: percentileMillis(subtreeApplyTotals, 0.95),
		StreamApplyP50Millis:       percentileMillis(streamApplies, 0.50),
		StreamApplyP95Millis:       percentileMillis(streamApplies, 0.95),
		ApplySnapshotP50Millis:     percentileMillis(applySnapshots, 0.50),
		ApplySnapshotP95Millis:     percentileMillis(applySnapshots, 0.95),
		OperationMetrics:           buildRealIndexOperationMetrics(operationSamples, operationWorkCounts),
		Unreadable:                 unreadable,
	}

	slowestScopes := make([]realIndexSlowScope, 0, len(slowestScopeByPath))
	for scope, elapsed := range slowestScopeByPath {
		slowestScopes = append(slowestScopes, realIndexSlowScope{
			Scope:         scope,
			ElapsedMillis: elapsed,
		})
	}

	sort.Slice(slowestScopes, func(left int, right int) bool {
		if slowestScopes[left].ElapsedMillis == slowestScopes[right].ElapsedMillis {
			return slowestScopes[left].Scope < slowestScopes[right].Scope
		}
		return slowestScopes[left].ElapsedMillis > slowestScopes[right].ElapsedMillis
	})
	if len(slowestScopes) > 20 {
		slowestScopes = slowestScopes[:20]
	}
	stats.SlowestScopes = slowestScopes
	stats.ScopeMetrics = buildRealIndexScopeMetrics(scopeMetricsByPath)

	return stats
}

func ensureRealIndexScopeMetric(metrics map[string]*realIndexScopeMetric, scope string) *realIndexScopeMetric {
	scope = strings.TrimSpace(scope)
	if scope == "" {
		scope = "<empty>"
	}
	if existing := metrics[scope]; existing != nil {
		return existing
	}
	metric := &realIndexScopeMetric{Scope: scope}
	metrics[scope] = metric
	return metric
}

func recordRealIndexScopeSQLiteMetric(metrics map[string]*realIndexScopeMetric, scope string, operation string, elapsed int64, workCount int) {
	if !strings.HasPrefix(operation, "subtree_stream_") && !strings.HasPrefix(operation, "direct_files_") {
		return
	}
	metric := ensureRealIndexScopeMetric(metrics, scope)
	metric.OperationSamples++
	if elapsed > metric.ElapsedMillis {
		metric.ElapsedMillis = elapsed
	}
	switch operation {
	case "subtree_stream_fresh_entries":
		metric.EntryCount += workCount
	case "subtree_stream_fresh_directories":
		metric.DirectoryCount += workCount
	case "direct_files_bulk_fresh_root_insert", "direct_files_bulk_empty_scope_insert":
		metric.EntryCount += workCount
	case "direct_files_stage_entries":
		if metric.EntryCount == 0 {
			metric.EntryCount = workCount
		}
	}
}

func buildRealIndexScopeMetrics(metricsByPath map[string]*realIndexScopeMetric) []realIndexScopeMetric {
	metrics := make([]realIndexScopeMetric, 0, len(metricsByPath))
	for _, metric := range metricsByPath {
		if metric == nil {
			continue
		}
		metrics = append(metrics, *metric)
	}
	sort.Slice(metrics, func(left int, right int) bool {
		if metrics[left].ElapsedMillis == metrics[right].ElapsedMillis {
			return metrics[left].Scope < metrics[right].Scope
		}
		return metrics[left].ElapsedMillis > metrics[right].ElapsedMillis
	})
	if len(metrics) > 40 {
		metrics = metrics[:40]
	}
	return metrics
}

func parseRealIndexUnreadableExamples(raw string) []string {
	parts := strings.Split(raw, " || ")
	examples := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		examples = append(examples, part)
	}
	return examples
}

func appendLimitedRealIndexExamples(existing []string, examples []string, limit int) []string {
	if limit <= 0 {
		return existing
	}
	seen := make(map[string]struct{}, len(existing)+len(examples))
	for _, example := range existing {
		seen[example] = struct{}{}
	}
	for _, example := range examples {
		if len(existing) >= limit {
			return existing
		}
		if _, ok := seen[example]; ok {
			continue
		}
		seen[example] = struct{}{}
		existing = append(existing, example)
	}
	return existing
}

func buildRealIndexOperationMetrics(samples map[string][]int64, workCounts map[string][]int) []realIndexOperation {
	metrics := make([]realIndexOperation, 0, len(samples))
	for name, values := range samples {
		if len(values) == 0 {
			continue
		}
		metric := realIndexOperation{
			Name:        name,
			Count:       len(values),
			TotalMillis: sumRealIndexMillis(values),
			P50Millis:   percentileMillis(values, 0.50),
			P95Millis:   percentileMillis(values, 0.95),
			MaxMillis:   maxRealIndexMillis(values),
		}
		for _, workCount := range workCounts[name] {
			metric.TotalWorkCount += int64(workCount)
			if workCount > metric.MaxWorkCount {
				metric.MaxWorkCount = workCount
			}
		}
		if metric.Count > 0 {
			metric.AverageWorkMillis = metric.TotalMillis / int64(metric.Count)
		}
		metrics = append(metrics, metric)
	}
	sort.Slice(metrics, func(left int, right int) bool {
		if metrics[left].TotalMillis == metrics[right].TotalMillis {
			return metrics[left].Name < metrics[right].Name
		}
		return metrics[left].TotalMillis > metrics[right].TotalMillis
	})
	return metrics
}

func sumRealIndexMillis(values []int64) int64 {
	total := int64(0)
	for _, value := range values {
		total += value
	}
	return total
}

func maxRealIndexMillis(values []int64) int64 {
	maximum := int64(0)
	for _, value := range values {
		if value > maximum {
			maximum = value
		}
	}
	return maximum
}

func mustParseRealIndexMillis(value string) int64 {
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		panic(fmt.Sprintf("parse real index millis %q: %v", value, err))
	}
	return parsed
}

func mustParseRealIndexInt(value string) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		panic(fmt.Sprintf("parse real index int %q: %v", value, err))
	}
	return parsed
}

func percentileMillis(values []int64, percentile float64) int64 {
	if len(values) == 0 {
		return 0
	}

	sorted := append([]int64(nil), values...)
	sort.Slice(sorted, func(left int, right int) bool {
		return sorted[left] < sorted[right]
	})

	index := int(math.Round(percentile * float64(len(sorted)-1)))
	return sorted[index]
}
