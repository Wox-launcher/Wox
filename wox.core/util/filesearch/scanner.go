package filesearch

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"wox/util"
)

const (
	defaultFTSOptimizeInterval = 12 * time.Hour
	// Bug fix: small interactive edits should reconcile quickly. Burst
	// backpressure below still protects generated-output storms.
	defaultDirtyDebounceWindow        = 2 * time.Second
	defaultDirtyPressureLowWindow     = 2 * time.Minute
	defaultDirtyPressureHighWindow    = 5 * time.Minute
	defaultMaxDirtyDebounceWindow     = 15 * time.Minute
	defaultMaxPendingDirtyWaitWindow  = 5 * time.Second
	defaultDirtyBackpressurePathCount = 64
	defaultDirtyBackpressureRootCount = 2
	progressBatchSize                 = 256
	progressUpdateGap                 = 250 * time.Millisecond
)

var (
	woxFileSearchStoragePathOnce sync.Once
	woxFileSearchStoragePath     string
)

// cachedWoxFileSearchStoragePath returns Wox's internal filesearch storage path
// without recomputing the home-relative path for every scanned entry.
func cachedWoxFileSearchStoragePath() string {
	woxFileSearchStoragePathOnce.Do(func() {
		homeDir := cachedUserHomeDir()
		if homeDir == "" {
			return
		}
		woxFileSearchStoragePath = filepath.Clean(filepath.Join(homeDir, ".wox", "filesearch"))
	})
	return woxFileSearchStoragePath
}

type Scanner struct {
	db                     *FileSearchDB
	policy                 *policyState
	onStateChange          func(ctx context.Context)
	stopOnce               sync.Once
	wg                     sync.WaitGroup
	stopCh                 chan struct{}
	requestCh              chan scanRequest
	dirtyCh                chan struct{}
	runningMu              sync.Mutex
	scanRunning            bool
	changeFeed             ChangeFeed
	dirtyQueue             *DirtyQueue
	dirtyQueueConfig       DirtyQueueConfig
	dynamicRootConfig      DynamicRootConfig
	dynamicHeat            *dynamicRootHeatTracker
	dynamicFlushGeneration int
	reconciler             *Reconciler
	// Optimization: watcher signals resolve root state through this Scanner-local
	// cache once refreshChangeFeedWithRoots has installed a complete snapshot,
	// avoiding SQLite lookups in the high-frequency change-signal path.
	rootCacheMu         sync.RWMutex
	rootCacheByID       map[string]RootRecord
	rootCacheLoaded     bool
	transientRunMu      sync.RWMutex
	transientRunState   *StatusSnapshot
	transientRootMu     sync.RWMutex
	transientRootState  *TransientRootState
	transientSyncMu     sync.RWMutex
	transientSyncState  *TransientSyncState
	dirtyBackpressureMu sync.Mutex
	lastDirtyRunElapsed time.Duration
	// Tests override the preparation budget so run-based smoke coverage can force
	// job splitting without manufacturing thousands of files just to cross the
	// production thresholds.
	plannerBudgetOverride *splitBudget
}

type scanRequest struct {
	Reason     string
	TraceID    string
	ResetIndex bool
	ResetReady chan error
}

func NewScanner(db *FileSearchDB) *Scanner {
	return newScannerWithPolicyState(db, newPolicyState(Policy{}))
}

// newScannerWithPolicyState keeps the engine policy object stable while the
// scanner and database are rebuilt. A full storage reset closes and replaces the
// SQLite database, and reusing the policy state avoids losing plugin-owned
// ignore rules during that handoff.
func newScannerWithPolicyState(db *FileSearchDB, policy *policyState) *Scanner {
	if policy == nil {
		policy = newPolicyState(Policy{})
	}

	dirtyQueueConfig := DirtyQueueConfig{
		DebounceWindow:            defaultDirtyDebounceWindow,
		MaxPendingWaitWindow:      defaultMaxPendingDirtyWaitWindow,
		MaxDebounceWindow:         defaultMaxDirtyDebounceWindow,
		BackpressurePathThreshold: defaultDirtyBackpressurePathCount,
		BackpressureRootThreshold: defaultDirtyBackpressureRootCount,
		SiblingMergeThreshold:     8,
		// Dirty bursts used to promote to a configured-root reconcile after a
		// fixed count/ratio. With scoped planning in place that made noisy but
		// localized changes under large roots scan far more than they touched, so
		// default escalation is disabled and only explicit root signals stay broad.
		RootEscalationPathThreshold:  0,
		RootEscalationDirectoryRatio: 0,
	}

	return &Scanner{
		db:                db,
		policy:            policy,
		stopCh:            make(chan struct{}),
		requestCh:         make(chan scanRequest, 1),
		dirtyCh:           make(chan struct{}, 1),
		changeFeed:        newPlatformChangeFeed(),
		dirtyQueueConfig:  dirtyQueueConfig,
		dirtyQueue:        NewDirtyQueue(dirtyQueueConfig),
		dynamicRootConfig: defaultDynamicRootConfig(),
		dynamicHeat:       newDynamicRootHeatTracker(),
		reconciler:        NewReconciler(db, policy),
	}
}

func (s *Scanner) SetStateChangeHandler(handler func(ctx context.Context)) {
	s.onStateChange = handler
}

func (s *Scanner) Start(ctx context.Context) {
	s.wg.Add(1)
	util.Go(ctx, "filesearch change feed loop", func() {
		defer s.wg.Done()
		s.changeFeedLoop(ctx)
	})

	s.wg.Add(1)
	util.Go(ctx, "filesearch scan loop", func() {
		defer s.wg.Done()
		util.GetLogger().Info(ctx, "filesearch scanner started")
		if err := s.db.EnsureForegroundEntryIndexes(ctx); err != nil {
			util.GetLogger().Warn(ctx, "filesearch failed to recover foreground entry indexes: "+err.Error())
		}
		s.startupRestore(ctx)
		s.buildMaintenanceEntryIndexesAsync(util.NewTraceContext(), false)

		ftsOptimizeTimer := time.NewTimer(defaultFTSOptimizeInterval)
		defer ftsOptimizeTimer.Stop()

		dirtyTimer := time.NewTimer(time.Hour)
		if !dirtyTimer.Stop() {
			<-dirtyTimer.C
		}
		defer dirtyTimer.Stop()

		for {
			select {
			case <-ftsOptimizeTimer.C:
				// Optimization: FTS optimize is global table maintenance, not a
				// correctness step for each file change. Running it on a fixed
				// 12-hour cadence keeps segment compaction available without making
				// every incremental root finalize pay for all four FTS tables.
				optimizeCtx := util.NewTraceContext()
				if err := s.db.OptimizeFTSTables(optimizeCtx); err != nil {
					util.GetLogger().Warn(optimizeCtx, "filesearch scheduled FTS optimize failed: "+err.Error())
				}
				ftsOptimizeTimer.Reset(defaultFTSOptimizeInterval)
			case request := <-s.requestCh:
				rescanCtx := contextWithTraceID(util.NewTraceContext(), request.TraceID)
				util.GetLogger().Info(rescanCtx, fmt.Sprintf("filesearch full rescan triggered: reason=%s", request.Reason))
				s.resetDirtyQueueWithReason(rescanCtx, "full_rescan")
				if request.ResetIndex {
					// Feature addition: manual reindex requests are executed inside
					// the scanner loop so reset and full-scan writes stay ordered.
					// Resetting from the caller goroutine could race with an active
					// scan and briefly repopulate rows that the user asked to drop.
					if err := s.db.ResetIndex(rescanCtx); err != nil {
						request.completeReset(err)
						util.GetLogger().Warn(rescanCtx, "filesearch failed to reset index: "+err.Error())
						continue
					}
					request.completeReset(nil)
				}
				s.scanAllRootsWithReason(rescanCtx, request.Reason)
			case <-s.dirtyCh:
				s.resetDirtyTimer(dirtyTimer)
			case <-dirtyTimer.C:
				if err := s.processDirtyQueue(util.NewTraceContext(), time.Now()); err != nil {
					util.GetLogger().Warn(ctx, "filesearch failed to process dirty queue: "+err.Error())
				}
				if pendingRootCount, pendingPathCount := s.pendingDirtyCounts(); pendingRootCount > 0 || pendingPathCount > 0 {
					s.resetDirtyTimer(dirtyTimer)
				}
			case <-s.stopCh:
				s.closeChangeFeed()
				return
			}
		}
	})
}

func (request scanRequest) completeReset(err error) {
	if request.ResetReady == nil {
		return
	}
	request.ResetReady <- err
}

func (s *Scanner) Stop() {
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
}

func (s *Scanner) StopAndWait() {
	if s == nil {
		return
	}

	// Bug fix: callers that need to remove the filesearch directory must wait for
	// the scanner goroutines to leave before closing SQLite. Stop alone only
	// signals the loops, which could leave a scan or change-feed refresh still
	// using the database while the reset deletes its files.
	s.Stop()
	s.wg.Wait()
}

func (s *Scanner) RequestRescan(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	traceID := util.GetContextTraceId(ctx)
	select {
	case s.requestCh <- scanRequest{Reason: "request", TraceID: traceID}:
		util.GetLogger().Debug(contextWithTraceID(ctx, traceID), "filesearch rescan requested")
	default:
	}
}

func (s *Scanner) RequestResetRescan(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	traceID := util.GetContextTraceId(ctx)
	resetReady := make(chan error, 1)
	request := scanRequest{Reason: "manual_reset", TraceID: traceID, ResetIndex: true, ResetReady: resetReady}
	// Feature addition: the visible "Index Files" action must not be dropped
	// just because a regular rescan request is already buffered. Wait in the
	// background action goroutine until the scanner can serialize the reset.
	select {
	case s.requestCh <- request:
		util.GetLogger().Debug(contextWithTraceID(ctx, traceID), "filesearch reset rescan requested")
	case <-s.stopCh:
		return fmt.Errorf("filesearch scanner stopped")
	case <-ctx.Done():
		return ctx.Err()
	}

	select {
	case err := <-resetReady:
		return err
	case <-s.stopCh:
		return fmt.Errorf("filesearch scanner stopped")
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Scanner) scanAllRoots(ctx context.Context) {
	s.scanAllRootsWithReason(ctx, "unspecified")
}

func (s *Scanner) scanAllRootsWithReason(ctx context.Context, reason string) {
	s.runningMu.Lock()
	if s.scanRunning {
		s.runningMu.Unlock()
		util.GetLogger().Debug(ctx, fmt.Sprintf("filesearch scan cycle skipped: reason=%s active=true", reason))
		return
	}
	s.scanRunning = true
	s.runningMu.Unlock()

	defer func() {
		s.runningMu.Lock()
		s.scanRunning = false
		s.runningMu.Unlock()
	}()

	roots, err := s.listPolicyAllowedRoots(ctx)
	if err != nil {
		util.GetLogger().Warn(ctx, "filesearch failed to load roots: "+err.Error())
		return
	}
	util.GetLogger().Info(ctx, fmt.Sprintf("filesearch scan cycle started: reason=%s roots=%d", reason, len(roots)))

	if len(roots) == 0 {
		s.clearTransientRunState()
		return
	}

	if err := s.executePlannedRun(ctx, RunKindFull, reason, roots, nil); err != nil {
		util.GetLogger().Warn(ctx, "filesearch full run failed: "+err.Error())
		return
	}

	util.GetLogger().Info(ctx, fmt.Sprintf("filesearch scan cycle completed: reason=%s", reason))
	if shouldCollectFileSearchDiagnosticSnapshot() {
		// Optimization: the full diagnostic snapshot is useful, but it reads FTS
		// vocab and file-size stats that do not affect search readiness. Check the
		// diagnostic switch before starting the goroutine so production builds do not
		// schedule logging-only SQLite work.
		snapshotCtx := util.NewTraceContext()
		util.Go(snapshotCtx, "filesearch full scan diagnostic snapshot", func() {
			snapshot, err := s.db.SearchIndexSnapshot(snapshotCtx)
			if err != nil {
				util.GetLogger().Warn(snapshotCtx, "filesearch failed to capture sqlite snapshot after full scan: "+err.Error())
				return
			}
			logSQLiteIndexSnapshot(snapshotCtx, "full_scan_complete", snapshot, true)
		})
	}
}

func (s *Scanner) executePlannedRun(ctx context.Context, kind RunKind, reason string, roots []RootRecord, batches []ReconcileBatch) error {
	totalStartedAt := util.GetSystemTimestamp()
	allRoots := roots
	if s != nil && s.db != nil {
		var err error
		allRoots, err = s.listPolicyAllowedRoots(ctx)
		if err != nil {
			return err
		}
	}
	// Dynamic roots are hidden but still own entries under their promoted path.
	// The production path must inject the same per-parent exclusions into both
	// run preparation and snapshot execution; otherwise a parent reconcile can generate
	// a batch that SQLite upserts back over the dynamic root's path ownership.
	rootExclusions := buildDynamicRootExclusions(allRoots)
	planner := NewRunPlanner(s.policy)
	planner.SetRootExclusions(rootExclusions)
	if s != nil && s.plannerBudgetOverride != nil {
		planner.budget = *s.plannerBudgetOverride
	}
	planner.SetProgressCallback(func(progress RunPlannerProgress) {
		snapshot := buildPreparationStatusSnapshot(progress, kind)
		// The toolbar now shows live indexing throughput for full runs, so even
		// preparation snapshots carry elapsed time from the same user-visible
		// boundary that the final summary uses.
		snapshot.ActiveRunElapsedMs = util.GetSystemTimestamp() - totalStartedAt
		s.setTransientRunState(snapshot)
		s.emitStateChange(ctx)
		logFilesearchRunStage(ctx, kind, progress.Stage, progress.Root, Job{}, progress.RootIndex, progress.RootTotal, 0, int64(progress.RootTotal))
	})

	var (
		plan RunPlan
		err  error
	)
	preparationStartedAt := util.GetSystemTimestamp()
	switch kind {
	case RunKindIncremental:
		plan, err = planner.PlanIncrementalRun(ctx, roots, batches)
	default:
		plan, err = planner.PlanFullRun(ctx, roots)
	}
	if err != nil {
		s.handleRunPlanningFailure(ctx, roots, err)
		return err
	}
	s.markPlannerSkippedRoots(ctx, planner.SkippedRoots())
	// Run preparation can still touch the filesystem while sealing root-level
	// execution scopes, but it no longer owns recursive file counting. Record the
	// sealed workload timing so slow runs reveal whether the stall starts before
	// any SQLite write.
	logFilesearchRunPreparation(ctx, kind, util.GetSystemTimestamp()-preparationStartedAt, len(plan.RootPlans), len(plan.Jobs), plan.TotalWorkUnits)

	snapshotBuilder := NewSnapshotBuilder(s.policy)
	snapshotBuilder.SetRootExclusions(rootExclusions)
	if s.plannerBudgetOverride != nil && s.plannerBudgetOverride.DirectFileBatchSize > 0 {
		// Tests and local tuning already override the direct-files batch budget.
		// Run preparation now keeps one direct-files job per directory, so this value
		// only caps the internal staging batch size of that single job.
		snapshotBuilder.SetDirectFileBatchSize(s.plannerBudgetOverride.DirectFileBatchSize)
	}
	executor := NewJobExecutor(snapshotBuilder)
	executor.SetDirectFilesStreamFunc(func(runCtx context.Context, root RootRecord, job Job, snapshot *SnapshotBuilder, onProgress func(JobApplyStats)) (JobApplyStats, error) {
		return s.db.ApplyDirectFilesJobStream(runCtx, root, job, snapshot, onProgress)
	})
	executor.SetSubtreeStreamFunc(func(runCtx context.Context, root RootRecord, job Job, snapshot *SnapshotBuilder, onProgress func(JobApplyStats)) (JobApplyStats, error) {
		return s.db.ApplySubtreeJobStream(runCtx, root, job, snapshot, onProgress)
	})
	if kind == RunKindFull {
		// Full runs are the only place where we deliberately coalesce multiple
		// small subtree snapshots into one SQLite transaction. Incremental work
		// keeps its per-batch apply boundary so dirty-path retries and deletes
		// continue to map 1:1 to the prepared reconcile scopes.
		executor.SetSubtreeBatchConfig(defaultFullRunSubtreeApplyBatchConfig())
		executor.SetSubtreeBatchApplyFunc(func(runCtx context.Context, _ RootRecord, batches []SubtreeSnapshotBatch) error {
			return s.db.ReplaceSubtreeSnapshots(runCtx, batches)
		})
	}
	executor.SetApplyFunc(func(runCtx context.Context, root RootRecord, job Job, batch *SubtreeSnapshotBatch) error {
		if snapshot, ok := s.GetTransientRunState(); ok {
			snapshot.ActiveRootStatus = RootStatusWriting
			snapshot.ActiveStage = RunStageExecuting
			snapshot.ActiveJobKind = job.Kind
			snapshot.ActiveScopePath = job.ScopePath
			snapshot.ActiveProgressCurrent = 0
			snapshot.ActiveProgressTotal = 1
			snapshot.ActiveRunElapsedMs = util.GetSystemTimestamp() - totalStartedAt
			s.setTransientRunState(snapshot)
			s.emitStateChange(runCtx)
		}
		return s.applyRunJob(runCtx, kind, root, job, batch)
	})

	bulkSyncStarted := false
	runSucceeded := false
	if kind == RunKindFull {
		s.db.BeginBulkSync()
		bulkSyncStarted = true
		for _, root := range roots {
			// Full-run leaf scopes are sealed before execution and then applied once
			// each. Preparing the root baseline up front lets the DB layer reuse one
			// "was this root empty?" answer across the whole run instead of issuing
			// the same scope-level empty checks for every fresh subtree.
			if err := s.db.prepareBulkSyncFullRunRoot(ctx, root.ID); err != nil {
				return err
			}
		}
	}
	defer func() {
		if !bulkSyncStarted {
			return
		}
		if runSucceeded {
			s.emitFullRunBulkFinalizingSnapshot(ctx, plan, totalStartedAt)
		}
		bulkFinalizeStartedAt := util.GetSystemTimestamp()
		if err := s.db.EndBulkSync(ctx); err != nil {
			util.GetLogger().Warn(ctx, "filesearch failed to finalize bulk sqlite search sync: "+err.Error())
			if runSucceeded {
				// Bug fix: the saving-index snapshot is emitted before EndBulkSync
				// starts. If SQLite finalization fails, clear that transient state so
				// the toolbar does not stay on an active spinner for a run that will not
				// publish the normal completion summary.
				s.clearTransientRunState()
				s.emitStateChange(ctx)
			}
			return
		}
		// Full runs intentionally defer the global FTS rebuild until the facts
		// settle, so log the boundary explicitly instead of hiding that cost in a
		// generic "scan cycle completed" message.
		logFilesearchSQLiteMaintenance(ctx, "bulk_finalize", string(kind), util.GetSystemTimestamp()-bulkFinalizeStartedAt, len(filesearchFTSTables))
		logFilesearchIndexPhase(ctx, "bulk_finalize", string(kind), util.GetSystemTimestamp()-bulkFinalizeStartedAt, map[string]any{
			"fts_tables": len(filesearchFTSTables),
		})
		// Run preparation, execution, and deferred bulk finalize all belong to one
		// user-visible full-index attempt. Record that outer elapsed time here,
		// after bulk finalize succeeds, so later optimizations can compare one
		// stable end-to-end metric instead of manually summing phase logs.
		elapsedMs := util.GetSystemTimestamp() - totalStartedAt
		logFilesearchFullIndexTotal(ctx, reason, elapsedMs, len(plan.RootPlans), len(plan.Jobs), plan.TotalWorkUnits)
		if runSucceeded {
			s.emitCompletedFullRunSummary(ctx, plan, elapsedMs)
			s.buildMaintenanceEntryIndexesAsync(util.NewTraceContext(), true)
		}
	}()

	executionStartedAt := util.GetSystemTimestamp()
	run, _, err := executor.ExecuteRun(ctx, plan, roots, func(snapshot StatusSnapshot, job Job) {
		// Job executor snapshots own the monotonic run progress, but the scanner
		// owns the full-run start boundary. Attach elapsed time here so toolbar
		// rate calculations do not need plugin-local timers that can drift from
		// the actual index run.
		snapshot.ActiveRunElapsedMs = util.GetSystemTimestamp() - totalStartedAt
		if kind == RunKindFull && snapshot.ActiveRunStatus == RunStatusCompleted {
			// Bug fix: the executor finishes before deferred full-run SQLite/FTS
			// finalization. Publishing that intermediate completion made the toolbar
			// show an early duration, then replace it with the scanner-owned final
			// summary. Keep progress snapshots live, but reserve full completion for
			// emitCompletedFullRunSummary after bulk finalize has finished.
			logFilesearchRunStage(ctx, kind, snapshot.ActiveStage, rootRecordForRunLog(roots, job.RootID), job, snapshot.ActiveRootIndex, snapshot.ActiveRootTotal, snapshot.RunProgressCurrent, snapshot.RunProgressTotal)
			return
		}
		s.setTransientRunState(snapshot)
		s.emitStateChange(ctx)
		logFilesearchRunStage(ctx, kind, snapshot.ActiveStage, rootRecordForRunLog(roots, job.RootID), job, snapshot.ActiveRootIndex, snapshot.ActiveRootTotal, snapshot.RunProgressCurrent, snapshot.RunProgressTotal)
	})
	executionElapsedMs := util.GetSystemTimestamp() - executionStartedAt
	logFilesearchRunExecution(ctx, kind, executionElapsedMs, len(plan.Jobs), plan.TotalWorkUnits)
	logFilesearchIndexPhase(ctx, "run_execution", string(kind), executionElapsedMs, map[string]any{
		"jobs":  len(plan.Jobs),
		"units": plan.TotalWorkUnits,
	})
	if err != nil {
		s.handleRunFailure(ctx, run, roots)
		return err
	}
	runSucceeded = true

	s.clearTransientRunState()
	return nil
}

func (s *Scanner) emitFullRunBulkFinalizingSnapshot(ctx context.Context, plan RunPlan, startedAt int64) {
	if plan.Kind != RunKindFull {
		return
	}

	fileCount := plan.EstimatedTotals.FileCount
	entryCount := plan.EstimatedTotals.IndexableEntryCount
	if snapshot, ok := s.GetTransientRunState(); ok {
		fileCount = snapshot.ActiveRunFileCount
		entryCount = snapshot.ActiveRunEntryCount
	}

	// Feature addition: full indexing can spend a visible amount of time in the
	// deferred SQLite/FTS finalize after scan jobs have stopped emitting progress.
	// Publish a scanner-owned finalizing snapshot before EndBulkSync so the
	// toolbar keeps showing that the same full-index run is saving the persisted
	// index instead of going quiet until the completion summary appears.
	s.setTransientRunState(StatusSnapshot{
		RootCount:           len(plan.RootPlans),
		ProgressCurrent:     plan.TotalWorkUnits,
		ProgressTotal:       plan.TotalWorkUnits,
		ActiveRootStatus:    RootStatusFinalizing,
		ActiveRunStatus:     RunStatusFinalizing,
		ActiveRunKind:       RunKindFull,
		ActiveStage:         RunStageFinalizing,
		RunProgressCurrent:  plan.TotalWorkUnits,
		RunProgressTotal:    plan.TotalWorkUnits,
		ActiveRunFileCount:  fileCount,
		ActiveRunEntryCount: entryCount,
		ActiveRunElapsedMs:  util.GetSystemTimestamp() - startedAt,
		IsIndexing:          true,
	})
	s.emitStateChange(ctx)
}

func (s *Scanner) emitCompletedFullRunSummary(ctx context.Context, plan RunPlan, elapsedMs int64) {
	if plan.Kind != RunKindFull {
		return
	}

	fileCount := plan.EstimatedTotals.FileCount
	entryCount := plan.EstimatedTotals.IndexableEntryCount
	if s.db != nil {
		if countedFiles, countedEntries, err := s.db.SearchIndexCounts(ctx); err == nil {
			// Bug fix: full scans now use streaming estimates to avoid a duplicate
			// filesystem walk. Those estimates intentionally do not know the real
			// file count, so the completion summary reads final persisted counts
			// without paying for a full diagnostic SQLite snapshot.
			fileCount = countedFiles
			entryCount = countedEntries
		} else {
			util.GetLogger().Warn(ctx, "filesearch failed to count completed full index: "+err.Error())
		}
		if fileCount <= 0 && entryCount <= 0 {
			if snapshot, err := s.db.SearchIndexSnapshot(ctx); err == nil && snapshot.EntryCount > 0 {
				// Bug fix: immediately after a fresh streaming full run, the cheap
				// summary count can report an empty fact table even though the
				// diagnostic snapshot moments later sees the committed index. Use
				// the same verified snapshot path as the full-scan diagnostic before
				// falling back to an unknown count, so the toolbar never shows a
				// misleading "Indexed 0 files" for a populated index.
				fileCount = snapshot.FileCount
				entryCount = snapshot.EntryCount
			} else if err != nil {
				util.GetLogger().Warn(ctx, "filesearch failed to snapshot completed full index count: "+err.Error())
			}
		}
	}

	// Feature addition: the toolbar should receive one final full-index summary
	// after SQLite bulk maintenance has completed. The executor's completed
	// snapshot fires before deferred FTS rebuild/optimize work, so emitting this
	// scanner-owned snapshot keeps the user-visible "Indexed ..." message aligned
	// with the actual end of the full indexing run.
	s.setTransientRunState(StatusSnapshot{
		RootCount:           len(plan.RootPlans),
		ProgressCurrent:     plan.TotalWorkUnits,
		ProgressTotal:       plan.TotalWorkUnits,
		ActiveRunStatus:     RunStatusCompleted,
		ActiveRunKind:       RunKindFull,
		RunProgressCurrent:  plan.TotalWorkUnits,
		RunProgressTotal:    plan.TotalWorkUnits,
		ActiveRunFileCount:  fileCount,
		ActiveRunEntryCount: entryCount,
		ActiveRunElapsedMs:  elapsedMs,
		IsIndexing:          false,
	})
	s.emitStateChange(ctx)
	s.clearTransientRunState()
}

func (s *Scanner) applyRunJob(ctx context.Context, kind RunKind, root RootRecord, job Job, batch *SubtreeSnapshotBatch) error {
	switch job.Kind {
	case JobKindDirectDelta:
		return s.db.ApplyDirectDeltaJob(ctx, root, job, s.policy)
	case JobKindDirectFiles:
		if batch == nil {
			return fmt.Errorf("direct-files job %q is missing snapshot batch", job.JobID)
		}
		return s.db.ApplyDirectFilesJob(ctx, job, *batch)
	case JobKindSubtree:
		if batch == nil {
			return fmt.Errorf("subtree job %q is missing snapshot batch", job.JobID)
		}
		return s.db.ApplySubtreeJob(ctx, job, *batch)
	case JobKindFinalizeRoot:
		now := util.GetSystemTimestamp()
		root.LastReconcileAt = now
		if kind == RunKindFull {
			root.LastFullScanAt = now
		}
		root.FeedState = nextFeedStateAfterSuccessfulReconcile(root)
		root.LastError = nil
		root.ProgressCurrent = RootProgressScale
		root.ProgressTotal = RootProgressScale
		root.Status = RootStatusIdle
		root = s.captureRootFeedSnapshot(ctx, root)
		root.UpdatedAt = util.GetSystemTimestamp()
		if err := s.db.FinalizeRootRun(ctx, root); err != nil {
			return err
		}
		// Optimization: FinalizeRootRun is the durable boundary that refreshes
		// FeedState and feed snapshots, but it bypasses UpdateRootState. Update the
		// complete root cache here so the next watcher signal does not fall back to
		// SQLite after every successful incremental dirty flush.
		s.upsertRootCache(root)
		return nil
	default:
		return fmt.Errorf("unsupported run job kind %q", job.Kind)
	}
}

func (s *Scanner) handleRunFailure(ctx context.Context, run Run, roots []RootRecord) {
	snapshot, hasSnapshot := s.GetTransientRunState()
	if hasSnapshot {
		snapshot.ActiveRunStatus = run.Status
		snapshot.ActiveStage = run.Stage
		snapshot.IsIndexing = false
		snapshot.LastError = run.LastError
		s.setTransientRunState(snapshot)
		s.emitStateChange(ctx)
	}

	if strings.TrimSpace(run.ActiveJobID) == "" {
		s.clearTransientRunState()
		return
	}

	rootID := ""
	for _, root := range roots {
		if strings.Contains(run.ActiveJobID, root.ID) {
			rootID = root.ID
			break
		}
	}
	if rootID == "" {
		s.clearTransientRunState()
		return
	}

	root, ok := s.findRootByID(ctx, rootID)
	if !ok {
		s.clearTransientRunState()
		return
	}
	root.Status = RootStatusError
	root.ProgressCurrent = 0
	root.ProgressTotal = 0
	if strings.TrimSpace(run.LastError) != "" {
		errMessage := run.LastError
		root.LastError = &errMessage
	}
	root.UpdatedAt = util.GetSystemTimestamp()
	_ = s.updateRootStateAndCache(ctx, root)
	s.clearTransientRunState()
	s.emitStateChange(ctx)
}

func (s *Scanner) handleRunPlanningFailure(ctx context.Context, roots []RootRecord, cause error) {
	s.clearTransientRunState()

	var rootErr *runRootError
	if !errors.As(cause, &rootErr) || rootErr == nil || strings.TrimSpace(rootErr.RootID) == "" {
		return
	}
	root, ok := s.findRootByID(ctx, rootErr.RootID)
	if !ok {
		for _, candidate := range roots {
			if candidate.ID == rootErr.RootID {
				root = candidate
				ok = true
				break
			}
		}
	}
	if !ok {
		return
	}
	s.markRootPlanningError(ctx, root, cause)
}

func (s *Scanner) markPlannerSkippedRoots(ctx context.Context, skippedRoots []runPlannerSkippedRoot) {
	if len(skippedRoots) == 0 {
		return
	}

	for _, skippedRoot := range skippedRoots {
		root := skippedRoot.Root
		if strings.TrimSpace(root.ID) == "" {
			continue
		}
		if current, ok := s.findRootByID(ctx, root.ID); ok {
			root = current
		}
		// Bug fix: a skipped unreadable root should be visible in diagnostics, but
		// it must not keep the whole run in the previous root's preparing state.
		// Mark only the skipped root as failed and continue executing the sealed
		// plan for readable roots such as /Applications.
		s.markRootPlanningError(ctx, root, skippedRoot.Err)
	}
}

func (s *Scanner) markRootPlanningError(ctx context.Context, root RootRecord, cause error) {
	root.Status = RootStatusError
	root.ProgressCurrent = 0
	root.ProgressTotal = 0
	if cause != nil && strings.TrimSpace(cause.Error()) != "" {
		errMessage := cause.Error()
		root.LastError = &errMessage
	}
	root.UpdatedAt = util.GetSystemTimestamp()
	if err := s.updateRootStateAndCache(ctx, root); err != nil {
		util.GetLogger().Warn(ctx, "filesearch failed to persist root planning error: "+err.Error())
		return
	}
	s.emitStateChange(ctx)
}

func buildPreparationStatusSnapshot(progress RunPlannerProgress, kind RunKind) StatusSnapshot {
	current := int64(progress.RootIndex)
	total := int64(progress.RootTotal)
	if total <= 0 {
		total = 1
	}
	rootStatus := RootStatusPreparing
	return StatusSnapshot{
		ProgressCurrent:       0,
		ProgressTotal:         0,
		ActiveRootStatus:      rootStatus,
		ActiveProgressCurrent: current,
		ActiveProgressTotal:   total,
		ActiveRootIndex:       progress.RootIndex,
		ActiveRootTotal:       progress.RootTotal,
		ActiveRootPath:        filepath.Clean(progress.Root.Path),
		ActiveRunStatus:       runStatusForPreparationStage(progress.Stage),
		ActiveRunKind:         kind,
		ActiveStage:           progress.Stage,
		ActiveScopePath:       activePreparationScopePath(progress),
		RunProgressCurrent:    0,
		RunProgressTotal:      0,
		IsIndexing:            true,
	}
}

func activePreparationScopePath(progress RunPlannerProgress) string {
	scopePath := filepath.Clean(progress.ScopePath)
	if strings.TrimSpace(scopePath) == "" {
		return filepath.Clean(progress.Root.Path)
	}
	return scopePath
}

func runStatusForPreparationStage(stage RunStage) RunStatus {
	switch stage {
	case RunStagePlanning:
		return RunStatusPlanning
	default:
		return RunStatusExecuting
	}
}

func rootRecordForRunLog(roots []RootRecord, rootID string) RootRecord {
	for _, root := range roots {
		if root.ID == rootID {
			return root
		}
	}
	return RootRecord{ID: rootID}
}

func (s *Scanner) refreshChangeFeed(ctx context.Context) {
	s.refreshChangeFeedWithRoots(ctx, nil)
}

// buildMaintenanceEntryIndexesAsync keeps maintenance-only SQLite indexes off
// the user-visible full-index critical path.
func (s *Scanner) buildMaintenanceEntryIndexesAsync(ctx context.Context, refreshChangeFeedAfter bool) {
	if s == nil || s.db == nil {
		return
	}
	util.Go(ctx, "filesearch build maintenance entry indexes", func() {
		if err := s.db.BuildMaintenanceEntryIndexes(ctx); err != nil {
			util.GetLogger().Warn(ctx, "filesearch failed to build maintenance entry indexes: "+err.Error())
			return
		}
		if refreshChangeFeedAfter {
			s.refreshChangeFeed(ctx)
		}
		select {
		case s.dirtyCh <- struct{}{}:
		default:
		}
	})
}

func (s *Scanner) refreshChangeFeedWithRoots(ctx context.Context, roots []RootRecord) {
	if roots == nil {
		var err error
		roots, err = s.listPolicyAllowedRoots(ctx)
		if err != nil {
			util.GetLogger().Warn(ctx, "filesearch failed to refresh change feed roots: "+err.Error())
			return
		}
	}

	// Optimization: refresh is the one boundary where the scanner has a complete
	// policy-pruned root snapshot. Replace the hot-path cache here rather than in
	// listPolicyAllowedRoots, which is also used by planner/query paths.
	s.replaceRootCache(roots)
	if s.changeFeed == nil {
		return
	}

	if err := s.changeFeed.Refresh(ctx, roots); err != nil {
		util.GetLogger().Warn(ctx, "filesearch failed to refresh change feed: "+err.Error())
		return
	}

	util.GetLogger().Info(ctx, fmt.Sprintf("filesearch change feed refreshed: roots=%d mode=%s", len(roots), s.changeFeed.Mode()))
}

func (s *Scanner) listPolicyAllowedRoots(ctx context.Context) ([]RootRecord, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	roots, err := s.db.ListRoots(ctx)
	if err != nil {
		return nil, err
	}
	return s.prunePolicyRejectedDynamicRoots(ctx, roots)
}

func (s *Scanner) prunePolicyRejectedDynamicRoots(ctx context.Context, roots []RootRecord) ([]RootRecord, error) {
	if len(roots) == 0 {
		return roots, nil
	}

	rootsByID := make(map[string]RootRecord, len(roots))
	for _, root := range roots {
		rootsByID[root.ID] = root
	}

	kept := make([]RootRecord, 0, len(roots))
	prunedPaths := make([]string, 0)
	for _, root := range roots {
		if root.Kind != RootKindDynamic {
			kept = append(kept, root)
			continue
		}
		parentRoot, ok := rootsByID[root.DynamicParentRootID]
		if !ok {
			kept = append(kept, root)
			continue
		}
		if s.shouldProcessChange(parentRoot, ChangeSignal{
			Kind:          ChangeSignalKindDirtyPath,
			RootID:        parentRoot.ID,
			Path:          root.Path,
			PathIsDir:     true,
			PathTypeKnown: true,
		}) {
			kept = append(kept, root)
			continue
		}

		// Bug fix: ignore-rule changes must also retire hidden dynamic roots that
		// were persisted before the rule existed. Deleting the dynamic root drops
		// its indexed rows instead of moving them back to the parent, because the
		// parent policy now says this subtree should not be indexed at all.
		if err := s.db.DeleteRoot(ctx, root.ID); err != nil {
			return nil, err
		}
		prunedPaths = append(prunedPaths, root.Path)
	}

	if len(prunedPaths) > 0 {
		util.GetLogger().Info(ctx, fmt.Sprintf(
			"filesearch pruned policy-rejected dynamic roots: count=%d paths=%s",
			len(prunedPaths),
			summarizeLogPaths(prunedPaths),
		))
	}
	return kept, nil
}

func (s *Scanner) changeFeedLoop(ctx context.Context) {
	if s.changeFeed == nil {
		return
	}

	for {
		select {
		case <-s.stopCh:
			return
		case signal, ok := <-s.changeFeed.Signals():
			if !ok {
				return
			}
			s.handleChangeSignal(util.NewTraceContext(), signal)
		}
	}
}

func (s *Scanner) closeChangeFeed() {
	if s.changeFeed == nil {
		return
	}
	if err := s.changeFeed.Close(); err != nil {
		util.GetLogger().Warn(context.Background(), "filesearch failed to close change feed: "+err.Error())
	}
}

func (s *Scanner) GetTransientRootState() (TransientRootState, bool) {
	s.transientRootMu.RLock()
	defer s.transientRootMu.RUnlock()
	if s.transientRootState == nil {
		return TransientRootState{}, false
	}

	return *s.transientRootState, true
}

func (s *Scanner) GetTransientRunState() (StatusSnapshot, bool) {
	s.transientRunMu.RLock()
	defer s.transientRunMu.RUnlock()
	if s.transientRunState == nil {
		return StatusSnapshot{}, false
	}

	return *s.transientRunState, true
}

func (s *Scanner) GetTransientSyncState() (TransientSyncState, bool) {
	s.transientSyncMu.RLock()
	defer s.transientSyncMu.RUnlock()
	if s.transientSyncState == nil {
		return TransientSyncState{}, false
	}

	return *s.transientSyncState, true
}

func (s *Scanner) setTransientRunState(state StatusSnapshot) {
	stateCopy := state
	s.transientRunMu.Lock()
	s.transientRunState = &stateCopy
	s.transientRunMu.Unlock()
}

func (s *Scanner) clearTransientRunState() {
	s.transientRunMu.Lock()
	s.transientRunState = nil
	s.transientRunMu.Unlock()
}

func (s *Scanner) setTransientRootState(state TransientRootState) {
	stateCopy := state
	s.transientRootMu.Lock()
	s.transientRootState = &stateCopy
	s.transientRootMu.Unlock()
}

func (s *Scanner) clearTransientRootState(rootID string) {
	s.transientRootMu.Lock()
	defer s.transientRootMu.Unlock()
	if s.transientRootState == nil {
		return
	}
	if rootID == "" || s.transientRootState.Root.ID == rootID {
		s.transientRootState = nil
	}
}

func (s *Scanner) setTransientSyncState(state TransientSyncState) {
	stateCopy := state
	s.transientSyncMu.Lock()
	s.transientSyncState = &stateCopy
	s.transientSyncMu.Unlock()
}

func (s *Scanner) updateTransientSyncProgress(rootID string, progress ReconcileProgress) bool {
	s.transientSyncMu.Lock()
	defer s.transientSyncMu.Unlock()

	if s.transientSyncState == nil || s.transientSyncState.Root.ID != rootID {
		return false
	}

	// Reconcile used to leave the active root in "syncing" until the whole batch
	// finished, even after SQLite had started the expensive write phase. Mirror
	// the DB progress into the transient root state so toolbar/status consumers can
	// show actual write progress instead of a misleading pending-roots counter.
	switch progress.Stage {
	case ReplaceEntriesStageWriting:
		s.transientSyncState.Root.Status = RootStatusWriting
		s.transientSyncState.Root.ProgressCurrent = progress.Current
		s.transientSyncState.Root.ProgressTotal = progress.Total
	case ReplaceEntriesStageFinalizing:
		s.transientSyncState.Root.Status = RootStatusFinalizing
		s.transientSyncState.Root.ProgressCurrent = progress.Current
		s.transientSyncState.Root.ProgressTotal = progress.Total
	default:
		s.transientSyncState.Root.Status = RootStatusSyncing
		if progress.Total > 0 {
			s.transientSyncState.Root.ProgressTotal = progress.Total
		}
	}
	s.transientSyncState.Root.UpdatedAt = util.GetSystemTimestamp()
	return true
}

func (s *Scanner) clearTransientSyncState(rootID string) {
	s.transientSyncMu.Lock()
	defer s.transientSyncMu.Unlock()
	if s.transientSyncState == nil {
		return
	}
	if rootID == "" || s.transientSyncState.Root.ID == rootID {
		s.transientSyncState = nil
	}
}

func (s *Scanner) scanRoot(ctx context.Context, root RootRecord, rootIndex int, rootTotal int) {
	startTime := util.GetSystemTimestamp()
	util.GetLogger().Info(ctx, fmt.Sprintf(
		"filesearch scanning root: index=%d/%d path=%s kind=%s feed_type=%s feed_state=%s",
		rootIndex,
		rootTotal,
		root.Path,
		root.Kind,
		root.FeedType,
		root.FeedState,
	))
	s.clearTransientRootState(root.ID)
	if root.FeedType == "" {
		root.FeedType = RootFeedTypeFallback
	}
	if root.FeedState == "" {
		root.FeedState = RootFeedStateReady
	}
	root.Status = RootStatusPreparing
	root.ProgressCurrent = 0
	root.ProgressTotal = 0
	root.LastError = nil
	root.UpdatedAt = util.GetSystemTimestamp()
	_ = s.db.UpdateRootState(ctx, root)
	s.setTransientRootState(TransientRootState{
		Root:            root,
		RootIndex:       rootIndex,
		RootTotal:       rootTotal,
		DiscoveredCount: 1,
		DirectoryIndex:  0,
		DirectoryTotal:  1,
		ItemCurrent:     0,
		ItemTotal:       0,
	})
	s.emitStateChange(ctx)

	plan, err := s.buildScanPlan(ctx, root, rootIndex, rootTotal)
	if err != nil {
		root.Status = RootStatusError
		errMessage := err.Error()
		root.LastError = &errMessage
		root.UpdatedAt = util.GetSystemTimestamp()
		_ = s.db.UpdateRootState(ctx, root)
		s.emitStateChange(ctx)
		util.GetLogger().Warn(ctx, "filesearch failed to scan root "+root.Path+": "+err.Error())
		return
	}

	root.Status = RootStatusScanning
	root.ProgressCurrent = 0
	root.ProgressTotal = plan.TotalItems
	root.UpdatedAt = util.GetSystemTimestamp()
	_ = s.db.UpdateRootState(ctx, root)
	s.setTransientRootState(TransientRootState{
		Root:            root,
		RootIndex:       rootIndex,
		RootTotal:       rootTotal,
		DiscoveredCount: 1,
		DirectoryIndex:  0,
		DirectoryTotal:  plan.DirectoryTotal,
		ItemCurrent:     0,
		ItemTotal:       plan.TotalItems,
	})
	s.emitStateChange(ctx)

	entries, err := s.collectEntries(ctx, root, plan, rootIndex, rootTotal)
	if err != nil {
		root.Status = RootStatusError
		errMessage := err.Error()
		root.LastError = &errMessage
		root.UpdatedAt = util.GetSystemTimestamp()
		_ = s.db.UpdateRootState(ctx, root)
		s.clearTransientRootState(root.ID)
		s.emitStateChange(ctx)
		util.GetLogger().Warn(ctx, "filesearch failed to collect entries for root "+root.Path+": "+err.Error())
		return
	}

	s.setTransientRootState(TransientRootState{
		Root: RootRecord{
			ID:              root.ID,
			Path:            root.Path,
			Kind:            root.Kind,
			Status:          RootStatusFinalizing,
			FeedType:        root.FeedType,
			FeedCursor:      root.FeedCursor,
			FeedState:       root.FeedState,
			LastReconcileAt: root.LastReconcileAt,
			LastFullScanAt:  root.LastFullScanAt,
			ProgressCurrent: 0,
			ProgressTotal:   0,
			LastError:       nil,
			CreatedAt:       root.CreatedAt,
			UpdatedAt:       util.GetSystemTimestamp(),
		},
		RootIndex:       rootIndex,
		RootTotal:       rootTotal,
		DiscoveredCount: int64(len(entries)),
		DirectoryIndex:  plan.DirectoryTotal,
		DirectoryTotal:  plan.DirectoryTotal,
		ItemCurrent:     plan.TotalItems,
		ItemTotal:       plan.TotalItems,
	})
	s.emitStateChange(ctx)

	scanTimestamp := util.GetSystemTimestamp()
	directories := buildDirectorySnapshotRecords(root, plan, scanTimestamp)
	if err := s.db.ReplaceRootSnapshot(ctx, root, directories, entries, func(progress ReplaceEntriesProgress) {
		nextRoot := RootRecord{
			ID:              root.ID,
			Path:            root.Path,
			Kind:            root.Kind,
			Status:          RootStatusFinalizing,
			FeedType:        root.FeedType,
			FeedCursor:      root.FeedCursor,
			FeedState:       root.FeedState,
			LastReconcileAt: root.LastReconcileAt,
			LastFullScanAt:  root.LastFullScanAt,
			ProgressCurrent: 0,
			ProgressTotal:   0,
			LastError:       nil,
			CreatedAt:       root.CreatedAt,
			UpdatedAt:       util.GetSystemTimestamp(),
		}

		if progress.Stage == ReplaceEntriesStageWriting {
			nextRoot.Status = RootStatusWriting
			nextRoot.ProgressCurrent = progress.Current
			nextRoot.ProgressTotal = progress.Total
		}

		s.setTransientRootState(TransientRootState{
			Root:            nextRoot,
			RootIndex:       rootIndex,
			RootTotal:       rootTotal,
			DiscoveredCount: int64(len(entries)),
			DirectoryIndex:  plan.DirectoryTotal,
			DirectoryTotal:  plan.DirectoryTotal,
			ItemCurrent:     plan.TotalItems,
			ItemTotal:       plan.TotalItems,
		})
		s.emitStateChange(ctx)
	}); err != nil {
		root.Status = RootStatusError
		errMessage := err.Error()
		root.LastError = &errMessage
		root.UpdatedAt = util.GetSystemTimestamp()
		_ = s.db.UpdateRootState(ctx, root)
		s.clearTransientRootState(root.ID)
		s.emitStateChange(ctx)
		util.GetLogger().Warn(ctx, "filesearch failed to replace entries for root "+root.Path+": "+err.Error())
		return
	}

	root.LastReconcileAt = scanTimestamp
	root.LastFullScanAt = scanTimestamp
	root.Status = RootStatusFinalizing
	root.ProgressCurrent = RootProgressScale
	root.ProgressTotal = RootProgressScale
	root.LastError = nil
	root.FeedState = RootFeedStateReady
	root = s.captureRootFeedSnapshot(ctx, root)
	root.UpdatedAt = util.GetSystemTimestamp()
	_ = s.db.UpdateRootState(ctx, root)
	s.refreshChangeFeed(ctx)
	root.Status = RootStatusIdle
	root.UpdatedAt = util.GetSystemTimestamp()
	_ = s.db.UpdateRootState(ctx, root)
	s.clearTransientRootState(root.ID)
	s.emitStateChange(ctx)
	util.GetLogger().Info(ctx, fmt.Sprintf(
		"filesearch scanned root: index=%d/%d path=%s entries=%d cost=%dms",
		rootIndex,
		rootTotal,
		root.Path,
		len(entries),
		util.GetSystemTimestamp()-startTime,
	))
}

func (s *Scanner) buildScanPlan(ctx context.Context, root RootRecord, rootIndex int, rootTotal int) (scanPlan, error) {
	rootPath := filepath.Clean(root.Path)
	if _, err := os.Stat(rootPath); err != nil {
		return scanPlan{}, err
	}

	queue := []scanState{{
		path:   rootPath,
		policy: s.policy.newTraversalContext(root, rootPath),
	}}
	plannedDirectories := make([]plannedDirectory, 0, 64)
	discoveredDirectories := 1
	processedDirectories := 0
	totalItems := int64(1)
	lastProgressUpdateAt := time.Now()

	for len(queue) > 0 {
		select {
		case <-ctx.Done():
			return scanPlan{}, ctx.Err()
		default:
		}

		state := queue[0]
		queue = queue[1:]

		dirEntries, readErr := os.ReadDir(state.path)
		if readErr != nil {
			processedDirectories++
			s.updatePlanningProgress(ctx, root, rootIndex, rootTotal, processedDirectories, discoveredDirectories)
			if state.path == rootPath {
				return scanPlan{}, fmt.Errorf("failed to read root directory %s: %w", state.path, readErr)
			}
			util.GetLogger().Warn(ctx, "filesearch skipped unreadable directory "+state.path+": "+readErr.Error())
			continue
		}

		plannedDirectories = append(plannedDirectories, plannedDirectory{
			path:       state.path,
			childCount: len(dirEntries),
			policy:     state.policy,
		})
		totalItems += int64(len(dirEntries))
		processedDirectories++

		for _, dirEntry := range dirEntries {
			fullPath := filepath.Join(state.path, dirEntry.Name())
			isDir := dirEntry.IsDir()
			if shouldSkipSystemPathForRoot(root, fullPath, isDir) {
				continue
			}
			if !state.policy.ShouldIndexPath(fullPath, isDir) {
				continue
			}

			if isDir {
				// Optimization: the legacy scanner path now carries traversal policy
				// state like the run planner. The older per-path policy callback kept
				// this fallback path on the expensive ancestor-rebuild matcher.
				queue = append(queue, scanState{
					path:   fullPath,
					policy: state.policy.Descend(fullPath),
				})
				discoveredDirectories++
			}
		}

		if processedDirectories%progressBatchSize == 0 || time.Since(lastProgressUpdateAt) >= progressUpdateGap {
			s.updatePlanningProgress(ctx, root, rootIndex, rootTotal, processedDirectories, discoveredDirectories)
			lastProgressUpdateAt = time.Now()
		}
	}

	s.updatePlanningProgress(ctx, root, rootIndex, rootTotal, processedDirectories, discoveredDirectories)

	return scanPlan{
		directories:    plannedDirectories,
		DirectoryTotal: len(plannedDirectories),
		TotalItems:     totalItems,
	}, nil
}

func (s *Scanner) collectEntries(ctx context.Context, root RootRecord, plan scanPlan, rootIndex int, rootTotal int) ([]EntryRecord, error) {
	rootPath := filepath.Clean(root.Path)
	rootInfo, err := os.Stat(rootPath)
	if err != nil {
		return nil, err
	}

	entries := []EntryRecord{newEntryRecord(root, rootPath, rootInfo)}
	processedItems := int64(1)
	lastReportedItems := int64(0)
	lastProgressUpdateAt := time.Now()

	if len(plan.directories) == 0 {
		s.updateScanProgress(ctx, root, rootIndex, rootTotal, 0, 0, int64(len(entries)), processedItems, plan.TotalItems, &lastReportedItems, true)
		return entries, nil
	}

	for directoryIndex, plannedDirectory := range plan.directories {
		select {
		case <-ctx.Done():
			return entries, ctx.Err()
		default:
		}

		dirEntries, readErr := os.ReadDir(plannedDirectory.path)
		if readErr != nil {
			processedItems += int64(plannedDirectory.childCount)
			s.updateScanProgress(ctx, root, rootIndex, rootTotal, directoryIndex+1, plan.DirectoryTotal, int64(len(entries)), processedItems, plan.TotalItems, &lastReportedItems, true)
			if plannedDirectory.path == rootPath {
				return nil, fmt.Errorf("failed to read root directory %s: %w", plannedDirectory.path, readErr)
			}
			util.GetLogger().Warn(ctx, "filesearch skipped unreadable directory "+plannedDirectory.path+": "+readErr.Error())
			continue
		}

		count := 0
		for _, dirEntry := range dirEntries {
			fullPath := filepath.Join(plannedDirectory.path, dirEntry.Name())
			info, infoErr := dirEntry.Info()
			if infoErr != nil {
				processedItems++
				count++
				if count%progressBatchSize == 0 || time.Since(lastProgressUpdateAt) >= progressUpdateGap {
					s.updateScanProgress(ctx, root, rootIndex, rootTotal, directoryIndex+1, plan.DirectoryTotal, int64(len(entries)), processedItems, plan.TotalItems, &lastReportedItems, false)
					lastProgressUpdateAt = time.Now()
				}
				continue
			}

			isDir := info.IsDir()
			if shouldSkipSystemPathForRoot(root, fullPath, isDir) {
				processedItems++
				count++
				if count%progressBatchSize == 0 || time.Since(lastProgressUpdateAt) >= progressUpdateGap {
					s.updateScanProgress(ctx, root, rootIndex, rootTotal, directoryIndex+1, plan.DirectoryTotal, int64(len(entries)), processedItems, plan.TotalItems, &lastReportedItems, false)
					lastProgressUpdateAt = time.Now()
				}
				continue
			}
			if !plannedDirectory.policy.ShouldIndexPath(fullPath, isDir) {
				processedItems++
				count++
				if count%progressBatchSize == 0 || time.Since(lastProgressUpdateAt) >= progressUpdateGap {
					s.updateScanProgress(ctx, root, rootIndex, rootTotal, directoryIndex+1, plan.DirectoryTotal, int64(len(entries)), processedItems, plan.TotalItems, &lastReportedItems, false)
					lastProgressUpdateAt = time.Now()
				}
				continue
			}

			entries = append(entries, newEntryRecord(root, fullPath, info))
			count++
			processedItems++
			if count%progressBatchSize == 0 || time.Since(lastProgressUpdateAt) >= progressUpdateGap {
				s.updateScanProgress(ctx, root, rootIndex, rootTotal, directoryIndex+1, plan.DirectoryTotal, int64(len(entries)), processedItems, plan.TotalItems, &lastReportedItems, false)
				lastProgressUpdateAt = time.Now()
				time.Sleep(2 * time.Millisecond)
			}
		}

		s.updateScanProgress(ctx, root, rootIndex, rootTotal, directoryIndex+1, plan.DirectoryTotal, int64(len(entries)), processedItems, plan.TotalItems, &lastReportedItems, true)
	}

	return entries, nil
}

func (s *Scanner) updatePlanningProgress(
	ctx context.Context,
	root RootRecord,
	rootIndex int,
	rootTotal int,
	processedDirectories int,
	discoveredDirectories int,
) {
	s.setTransientRootState(TransientRootState{
		Root:            root,
		RootIndex:       rootIndex,
		RootTotal:       rootTotal,
		DiscoveredCount: int64(discoveredDirectories),
		DirectoryIndex:  processedDirectories,
		DirectoryTotal:  discoveredDirectories,
		ItemCurrent:     0,
		ItemTotal:       0,
	})
	s.emitStateChange(ctx)
}

func (s *Scanner) updateScanProgress(
	ctx context.Context,
	root RootRecord,
	rootIndex int,
	rootTotal int,
	directoryIndex int,
	directoryTotal int,
	discoveredCount int64,
	currentItems int64,
	totalItems int64,
	lastReportedProgress *int64,
	force bool,
) {
	if totalItems <= 0 {
		totalItems = 1
	}
	if currentItems < 0 {
		currentItems = 0
	}
	if currentItems > totalItems {
		currentItems = totalItems
	}
	if !force && currentItems <= *lastReportedProgress {
		return
	}

	*lastReportedProgress = currentItems
	root.ProgressCurrent = currentItems
	root.ProgressTotal = totalItems
	root.UpdatedAt = util.GetSystemTimestamp()
	s.setTransientRootState(TransientRootState{
		Root:            root,
		RootIndex:       rootIndex,
		RootTotal:       rootTotal,
		DiscoveredCount: discoveredCount,
		DirectoryIndex:  directoryIndex,
		DirectoryTotal:  directoryTotal,
		ItemCurrent:     currentItems,
		ItemTotal:       totalItems,
	})
	s.emitStateChange(ctx)
}

func (s *Scanner) emitStateChange(ctx context.Context) {
	if s.onStateChange != nil {
		s.onStateChange(ctx)
	}
}

func (s *Scanner) shouldProcessChange(root RootRecord, signal ChangeSignal) bool {
	if signal.Kind == ChangeSignalKindRequiresRootReconcile || signal.Kind == ChangeSignalKindFeedUnavailable {
		return true
	}
	if s.policy != nil && !s.policy.shouldProcessChange(root, signal) {
		// Bug fix: remove/rename events were previously accepted before the plugin
		// policy ran, so ignored paths such as repository internals and generated
		// build outputs still re-queued incremental scans. Run the policy first for
		// path-scoped changes, then keep the remove/rename fast path for valid files
		// that may no longer exist on disk.
		return false
	}
	if signal.SemanticKind == ChangeSemanticKindRemove || signal.SemanticKind == ChangeSemanticKindRename {
		return true
	}
	if root.FeedState != RootFeedStateReady {
		return true
	}
	return true
}

func (s *Scanner) enqueueDirty(signal DirtySignal) {
	s.enqueueDirtyWithContext(context.Background(), signal)
}

func (s *Scanner) enqueueDirtyWithContext(ctx context.Context, signal DirtySignal) {
	if ctx == nil {
		ctx = context.Background()
	}
	normalized, ok := normalizeDirtySignal(signal)
	if !ok {
		return
	}
	if normalized.TraceID == "" {
		normalized.TraceID = util.GetContextTraceId(ctx)
	}

	if s.dirtyQueue != nil {
		s.dirtyQueue.Push(normalized)
	}
	shouldEmitState := s.refreshTransientSyncPendingCountsAndReportPendingTransition()
	if shouldEmitState {
		// Optimization: watcher bursts can enqueue thousands of signals before a
		// dirty flush runs. The transient pending counters are still refreshed for
		// every signal, but status listeners only need the first empty->pending
		// notification; later signals will be reflected by direct GetStatus calls
		// and by the post-flush state change.
		s.emitStateChange(contextWithTraceID(ctx, normalized.TraceID))
	}

	select {
	case s.dirtyCh <- struct{}{}:
	default:
	}
}

func (s *Scanner) enqueueAllRootsDirty(ctx context.Context) {
	s.enqueueAllRootsDirtyWithReason(ctx, "unspecified")
}

func (s *Scanner) enqueueAllRootsDirtyWithReason(ctx context.Context, reason string) {
	roots, err := s.listPolicyAllowedRoots(ctx)
	if err != nil {
		util.GetLogger().Warn(ctx, "filesearch failed to enqueue dirty roots: "+err.Error())
		return
	}

	rootPaths := make([]string, 0, len(roots))
	for _, root := range roots {
		rootPaths = append(rootPaths, root.Path)
	}
	util.GetLogger().Info(ctx, fmt.Sprintf(
		"filesearch queued full root reconcile: reason=%s roots=%d",
		reason,
		len(roots),
	))
	if len(rootPaths) > 0 {
		util.GetLogger().Debug(ctx, fmt.Sprintf(
			"filesearch queued full root reconcile roots: reason=%s paths=%s",
			reason,
			summarizeLogPaths(rootPaths),
		))
	}

	for _, root := range roots {
		s.enqueueDirtyWithContext(ctx, DirtySignal{
			Kind:          DirtySignalKindRoot,
			RootID:        root.ID,
			Path:          root.Path,
			PathIsDir:     true,
			PathTypeKnown: true,
		})
	}
}

func (s *Scanner) enqueueDirtyForPath(ctx context.Context, path string) bool {
	root, ok := s.findRootForPath(ctx, path)
	if !ok {
		return false
	}

	cleanPath := filepath.Clean(path)
	cleanRootPath := filepath.Clean(root.Path)
	// Bug fix: manual dirty routing must not be the one path that follows a
	// symlink-to-directory into a subtree scan. Use the same Lstat-first type
	// helper as watcher feeds so symlink entries stay direct file-like deltas.
	pathIsDir, pathTypeKnown := statPathType(cleanPath)

	kind := DirtySignalKindPath
	if cleanPath == cleanRootPath {
		kind = DirtySignalKindRoot
	} else if filepath.Dir(cleanPath) == cleanRootPath && !pathTypeKnown {
		// Manual dirty-path routing has no create/remove semantic attached. If a
		// missing direct child might have been a deleted directory, root scope is
		// the smallest safe boundary that can prune unknown recursive rows.
		kind = DirtySignalKindRoot
	}

	s.enqueueDirtyWithContext(ctx, DirtySignal{
		Kind:          kind,
		SemanticKind:  ChangeSemanticKindUnknown,
		RootID:        root.ID,
		Path:          cleanPath,
		PathIsDir:     pathIsDir,
		PathTypeKnown: pathTypeKnown,
	})
	return true
}

func (s *Scanner) processDirtyQueue(ctx context.Context, now time.Time) error {
	if s.dirtyQueue == nil {
		return nil
	}
	if s.db != nil {
		ready, err := s.db.EntryMaintenanceIndexesReady(ctx)
		if err != nil {
			return err
		}
		if !ready {
			// Fresh full scans publish foreground search before maintenance indexes
			// are rebuilt. Keep watcher signals queued until dirty diffs can use
			// their scoped lookup indexes again.
			return nil
		}
	}

	rootDirectoryCounts, rootsByID, _, err := s.loadDirtyQueueContext(ctx)
	if err != nil {
		return err
	}

	queuedRootCount, queuedPathCount := 0, 0
	if fileSearchDiagnosticLoggingEnabled {
		queuedRootCount, queuedPathCount = s.pendingDirtyCounts()
	}
	batches := s.dirtyQueue.FlushReadyWithDebounce(now, rootDirectoryCounts, s.currentDirtyDebounceWindow())
	if len(batches) == 0 {
		return nil
	}
	if fileSearchDiagnosticLoggingEnabled {
		remainingRootCount, remainingPathCount := s.pendingDirtyCounts()
		// Diagnostic logging: dirty flushes can happen every few seconds during
		// watcher storms, so the default runtime must not format/write this hot
		// path log. Developers can enable the diagnostic switch when investigating
		// queue timing and backpressure behavior.
		util.GetLogger().Info(ctx, fmt.Sprintf(
			"filesearch dirty queue flushed: batches=%d queued_roots=%d queued_paths=%d remaining_roots=%d remaining_paths=%d",
			len(batches),
			queuedRootCount,
			queuedPathCount,
			remainingRootCount,
			remainingPathCount,
		))
	}

	runRoots := make([]RootRecord, 0, len(batches))
	for _, batch := range batches {
		root, ok := rootsByID[batch.RootID]
		if !ok {
			continue
		}
		runRoots = append(runRoots, root)
	}
	if len(runRoots) == 0 {
		s.refreshTransientSyncPendingCounts()
		return nil
	}

	runStartedAt := time.Now()
	if err := s.executePlannedRun(ctx, RunKindIncremental, "dirty_queue", runRoots, batches); err != nil {
		s.recordDirtyRunElapsed(time.Since(runStartedAt))
		s.handleIncrementalRunFailure(ctx, runRoots, batches, err)
		return err
	}
	s.recordDirtyRunElapsed(time.Since(runStartedAt))

	s.logRootReloadIndexSnapshot(ctx)
	if err := s.handleSuccessfulDirtyFlush(ctx, batches, now); err != nil {
		// Dynamic-root lifecycle work is opportunistic after the real reconcile
		// has succeeded. A promotion/demotion failure should not turn an already
		// applied dirty batch into a user-visible indexing failure or force a broad
		// retry; the next hot flush can try the lifecycle step again.
		util.GetLogger().Warn(ctx, "filesearch dynamic root lifecycle failed: "+err.Error())
	}

	s.clearTransientSyncState("")
	s.refreshTransientSyncPendingCounts()
	s.emitStateChange(ctx)
	return nil
}

func (s *Scanner) handleChangeSignal(ctx context.Context, signal ChangeSignal) {
	// File watchers can emit thousands of valid change signals in a short burst.
	// The raw per-event trace made focused debugging harder without adding a
	// decision point, so keep logging on policy drops and batch failures instead.

	root, rootFound := s.findRootByID(ctx, signal.RootID)
	if rootFound && !s.shouldProcessChange(root, signal) {
		// Bug fix: policy drops must happen before feed-cursor writes. Updating the
		// SQLite root row for ignored ~/.wox/filesearch events modifies the same DB
		// file and can feed the watcher loop again.
		// util.GetLogger().Debug(ctx, fmt.Sprintf(
		// 	"filesearch change signal ignored by policy: kind=%s semantic=%s root=%s path=%s",
		// 	signal.Kind,
		// 	signal.SemanticKind,
		// 	signal.RootID,
		// 	summarizeLogPath(signal.Path),
		// ))
		return
	}
	s.updateRootFeedMetadata(ctx, root, rootFound, signal.FeedType, signal.Cursor)
	if rootFound {
		s.recordDynamicRootHeat(root, signal)
	}

	switch signal.Kind {
	case ChangeSignalKindDirtyRoot:
		s.enqueueDirtyWithContext(ctx, DirtySignal{
			Kind:          DirtySignalKindRoot,
			RootID:        signal.RootID,
			Path:          cleanDirtyQueuePath(signal.Path),
			PathIsDir:     true,
			PathTypeKnown: true,
			At:            signal.At,
		})
	case ChangeSignalKindDirtyPath:
		if !rootFound || root.FeedState == RootFeedStateReady || root.FeedType == RootFeedTypeFallback || root.FeedType == "" || shouldKeepKnownFileDeltaScoped(signal) {
			// Fallback feeds cannot replay a journal, so degraded state only tells us
			// a previous reconcile failed. Keeping later concrete dirty paths scoped
			// avoids converting one transient temp-directory miss into repeated full
			// root reconciles while explicit root-reconcile signals still stay broad.
			//
			// Bug fix: known file-level watcher events remain useful even when the
			// root is degraded because they describe one exact file mutation. Keeping
			// create/modify/metadata/rename/remove scoped lets visible desktop edits
			// update immediately while the separate degraded root reconcile can still
			// repair any missed history in the background.
			s.enqueueDirtyWithContext(ctx, DirtySignal{
				Kind:          DirtySignalKindPath,
				SemanticKind:  signal.SemanticKind,
				RootID:        signal.RootID,
				Path:          signal.Path,
				PathIsDir:     signal.PathIsDir,
				PathTypeKnown: signal.PathTypeKnown,
				At:            signal.At,
			})
			return
		}
		util.GetLogger().Info(ctx, fmt.Sprintf(
			"filesearch dirty path escalated to root reconcile: root=%s path=%s feed_state=%s",
			signal.RootID,
			summarizeLogPath(signal.Path),
			root.FeedState,
		))
		s.enqueueDirtyWithContext(ctx, DirtySignal{
			Kind:          DirtySignalKindRoot,
			RootID:        signal.RootID,
			Path:          root.Path,
			PathIsDir:     true,
			PathTypeKnown: true,
			At:            signal.At,
		})
	case ChangeSignalKindRequiresRootReconcile:
		util.GetLogger().Info(ctx, fmt.Sprintf(
			"filesearch change feed requested root reconcile: root=%s path=%s feed_type=%s reason=%q",
			signal.RootID,
			summarizeLogPath(signal.Path),
			signal.FeedType,
			strings.TrimSpace(signal.Reason),
		))
		s.updateRootFeedState(ctx, signal.RootID, RootFeedStateDegraded)
		s.enqueueDirtyWithContext(ctx, DirtySignal{
			Kind:          DirtySignalKindRoot,
			RootID:        signal.RootID,
			Path:          signal.Path,
			PathIsDir:     true,
			PathTypeKnown: true,
			At:            signal.At,
		})
	case ChangeSignalKindFeedUnavailable:
		util.GetLogger().Info(ctx, fmt.Sprintf(
			"filesearch change feed unavailable: root=%s path=%s feed_type=%s reason=%q",
			signal.RootID,
			summarizeLogPath(signal.Path),
			signal.FeedType,
			strings.TrimSpace(signal.Reason),
		))
		s.updateRootFeedState(ctx, signal.RootID, RootFeedStateUnavailable)
		s.enqueueDirtyWithContext(ctx, DirtySignal{
			Kind:          DirtySignalKindRoot,
			RootID:        signal.RootID,
			Path:          signal.Path,
			PathIsDir:     true,
			PathTypeKnown: true,
			At:            signal.At,
		})
	}
}

func shouldKeepKnownFileDeltaScoped(signal ChangeSignal) bool {
	if signal.Kind != ChangeSignalKindDirtyPath || !signal.PathTypeKnown || signal.PathIsDir {
		return false
	}
	switch signal.SemanticKind {
	case ChangeSemanticKindCreate, ChangeSemanticKindModify, ChangeSemanticKindMetadata, ChangeSemanticKindRemove, ChangeSemanticKindRename:
		return true
	default:
		return false
	}
}

func (s *Scanner) handleDirtyQueueFailure(ctx context.Context, root RootRecord, batch ReconcileBatch, remaining []ReconcileBatch, cause error) {
	util.GetLogger().Warn(ctx, fmt.Sprintf(
		"filesearch reconcile batch failed: root=%s path=%s mode=%s dirty_paths=%d scopes=%d remaining_batches=%d err=%s",
		batch.RootID,
		root.Path,
		batch.Mode,
		batch.DirtyPathCount,
		len(batch.Paths),
		len(remaining),
		cause.Error(),
	))
	if len(batch.Paths) > 0 {
		util.GetLogger().Debug(ctx, fmt.Sprintf(
			"filesearch reconcile batch failed scopes: root=%s paths=%s",
			batch.RootID,
			summarizeLogPaths(batch.Paths),
		))
	}
	if isIncrementalMissingPathFailure(cause) {
		// Missing-path failures are normal churn for temp/build directories. The
		// old recovery path marked the configured root degraded, which surfaced a
		// "needs attention" banner even though there is no user action to take.
		s.clearRootTransientIncrementalFailure(ctx, root.ID, cause)
		s.requeueDirtyBatches(ctx, remaining)
		s.refreshTransientSyncPendingCounts()
		s.emitStateChange(ctx)
		return
	}
	s.updateRootFeedState(ctx, root.ID, RootFeedStateDegraded)
	if shouldRetryIncrementalRootFailure(cause) && !s.enqueueDirtyRetryForFailedBatch(ctx, root, batch, time.Now()) {
		util.GetLogger().Warn(ctx, fmt.Sprintf(
			"filesearch reconcile batch stopped retrying failed scope: root=%s mode=%s paths=%s err=%s",
			root.ID,
			batch.Mode,
			summarizeLogPaths(batch.Paths),
			cause.Error(),
		))
	}
	s.requeueDirtyBatches(ctx, remaining)
	s.refreshTransientSyncPendingCounts()
	s.emitStateChange(ctx)
}

func (s *Scanner) enqueueDirtyRetryForFailedBatch(ctx context.Context, root RootRecord, batch ReconcileBatch, at time.Time) bool {
	switch batch.Mode {
	case ReconcileModeDirectDelta:
		requeued := false
		for _, delta := range batch.DirectDeltas {
			cleanPath := cleanDirtyQueuePath(delta.Path)
			if cleanPath == "" || !pathWithinScope(root.Path, cleanPath) {
				continue
			}
			// Feature addition: direct-delta failures retry the same exact file
			// paths. Widening them to the parent directory would reintroduce the
			// slow incremental behavior this path is designed to avoid.
			s.enqueueDirtyWithContext(ctx, DirtySignal{
				Kind:          DirtySignalKindPath,
				SemanticKind:  delta.SemanticKind,
				RootID:        batch.RootID,
				Path:          cleanPath,
				PathIsDir:     delta.PathIsDir,
				PathTypeKnown: delta.PathTypeKnown,
				At:            at,
			})
			requeued = true
		}
		if requeued {
			return true
		}
	case ReconcileModeSubtree:
		requeued := false
		for _, path := range batch.Paths {
			cleanPath := cleanDirtyQueuePath(path)
			if cleanPath == "" || !pathWithinScope(root.Path, cleanPath) {
				continue
			}
			// Retry the exact subtree scope that failed. The previous recovery path
			// always enqueued the configured root, which made one bad child under a
			// large home directory trigger another full recursive count.
			s.enqueueDirtyWithContext(ctx, DirtySignal{
				Kind:          DirtySignalKindPath,
				RootID:        batch.RootID,
				Path:          cleanPath,
				PathIsDir:     true,
				PathTypeKnown: true,
				At:            at,
			})
			requeued = true
		}
		if requeued {
			return true
		}
	case ReconcileModeRoot:
	}

	if batch.Mode != ReconcileModeRoot && len(batch.Paths) > 0 {
		return false
	}
	s.enqueueDirtyWithContext(ctx, DirtySignal{
		Kind:          DirtySignalKindRoot,
		RootID:        batch.RootID,
		Path:          root.Path,
		PathIsDir:     true,
		PathTypeKnown: true,
		At:            at,
	})
	return true
}

func (s *Scanner) handleIncrementalRunFailure(ctx context.Context, roots []RootRecord, batches []ReconcileBatch, cause error) {
	util.GetLogger().Warn(ctx, fmt.Sprintf(
		"filesearch incremental run failed: roots=%d batches=%d err=%s",
		len(roots),
		len(batches),
		cause.Error(),
	))

	// Incremental job boundaries are preparation-owned execution metadata, not
	// durable state. After a failure we retry the failed batch at its original
	// scoped paths when possible, then requeue untouched roots as-is so unrelated
	// scopes keep their existing debounce granularity.
	var rootErr *runRootError
	failedRootID := ""
	if errors.As(cause, &rootErr) && rootErr != nil {
		failedRootID = rootErr.RootID
	}
	if failedRootID == "" && len(batches) > 0 {
		failedRootID = batches[0].RootID
	}

	if isIncrementalMissingPathFailure(cause) {
		for _, root := range roots {
			if failedRootID != "" && root.ID != failedRootID {
				continue
			}
			// A vanished dirty scope has already resolved itself. Clear any
			// transient error persisted by the failed run instead of asking the
			// user to fix a compiler/temp directory that no longer exists.
			s.clearRootTransientIncrementalFailure(ctx, root.ID, cause)
		}
		remaining := make([]ReconcileBatch, 0, len(batches))
		for _, batch := range batches {
			if failedRootID == "" {
				break
			}
			if batch.RootID == failedRootID {
				continue
			}
			remaining = append(remaining, batch)
		}
		s.requeueDirtyBatches(ctx, remaining)
		s.refreshTransientSyncPendingCounts()
		s.emitStateChange(ctx)
		return
	}

	requeuedAt := time.Now()
	for _, root := range roots {
		if failedRootID != "" && root.ID != failedRootID {
			continue
		}
		s.updateRootFeedState(ctx, root.ID, RootFeedStateDegraded)
		s.markRootIncrementalFailure(ctx, root.ID, cause)
		if shouldRetryIncrementalRootFailure(cause) {
			requeued := false
			for _, batch := range batches {
				if batch.RootID != root.ID {
					continue
				}
				if s.enqueueDirtyRetryForFailedBatch(ctx, root, batch, requeuedAt) {
					requeued = true
				}
			}
			if !requeued {
				util.GetLogger().Warn(ctx, fmt.Sprintf(
					"filesearch incremental run stopped retrying failed scope: root=%s path=%s err=%s",
					root.ID,
					root.Path,
					cause.Error(),
				))
			}
			continue
		}
		util.GetLogger().Warn(ctx, fmt.Sprintf(
			"filesearch incremental run stopped retrying failed root: root=%s path=%s err=%s",
			root.ID,
			root.Path,
			cause.Error(),
		))
	}
	remaining := make([]ReconcileBatch, 0, len(batches))
	for _, batch := range batches {
		if failedRootID == "" {
			break
		}
		if batch.RootID == failedRootID {
			continue
		}
		remaining = append(remaining, batch)
	}
	s.requeueDirtyBatches(ctx, remaining)
	s.refreshTransientSyncPendingCounts()
	s.emitStateChange(ctx)
}

func (s *Scanner) markRootIncrementalFailure(ctx context.Context, rootID string, cause error) {
	root, ok := s.findRootByID(ctx, rootID)
	if !ok {
		return
	}

	// Incremental failures need to surface as durable root errors because the
	// run-local status disappears once the failed run exits. Without persisting
	// the root error here, deterministic permission failures could silently clear
	// the toolbar and then immediately hot-loop into another transient run.
	root.Status = RootStatusError
	errMessage := strings.TrimSpace(cause.Error())
	if errMessage != "" {
		root.LastError = &errMessage
	}
	root.UpdatedAt = util.GetSystemTimestamp()
	if err := s.updateRootStateAndCache(ctx, root); err != nil {
		util.GetLogger().Warn(ctx, "filesearch failed to persist incremental root failure: "+err.Error())
		return
	}
}

func shouldRetryIncrementalRootFailure(cause error) bool {
	return !isIncrementalPermissionFailure(cause) && !isIncrementalMissingPathFailure(cause)
}

func isIncrementalMissingPathFailure(cause error) bool {
	if cause == nil {
		return false
	}
	return errors.Is(cause, os.ErrNotExist) || isMissingPathErrorMessage(cause.Error())
}

func isMissingPathErrorMessage(message string) bool {
	message = strings.ToLower(strings.TrimSpace(message))
	if message == "" {
		return false
	}
	return strings.Contains(message, "cannot find the file specified") ||
		strings.Contains(message, "cannot find the path specified") ||
		strings.Contains(message, "no such file or directory") ||
		strings.Contains(message, "the system cannot find")
}

func (s *Scanner) clearRootTransientIncrementalFailure(ctx context.Context, rootID string, cause error) {
	root, ok := s.findRootByID(ctx, rootID)
	if !ok {
		return
	}
	if root.Status != RootStatusError && root.FeedState != RootFeedStateDegraded && root.LastError == nil {
		return
	}
	if root.Status == RootStatusError && root.LastError != nil && !isMissingPathErrorMessage(*root.LastError) {
		return
	}

	root.Status = RootStatusIdle
	root.ProgressCurrent = RootProgressScale
	root.ProgressTotal = RootProgressScale
	root.LastError = nil
	if root.FeedState == RootFeedStateDegraded {
		root.FeedState = RootFeedStateReady
	}
	root.UpdatedAt = util.GetSystemTimestamp()
	if err := s.updateRootStateAndCache(ctx, root); err != nil {
		util.GetLogger().Warn(ctx, "filesearch failed to clear transient incremental failure: "+err.Error())
		return
	}
	util.GetLogger().Info(ctx, fmt.Sprintf(
		"filesearch ignored transient missing dirty scope: root=%s path=%s err=%s",
		root.ID,
		root.Path,
		cause.Error(),
	))
}

func isIncrementalPermissionFailure(cause error) bool {
	if cause == nil {
		return false
	}
	if errors.Is(cause, os.ErrPermission) {
		return true
	}
	message := strings.ToLower(strings.TrimSpace(cause.Error()))
	return strings.Contains(message, "access is denied") ||
		strings.Contains(message, "permission denied") ||
		strings.Contains(message, "operation not permitted")
}

func (s *Scanner) updateRootFeedMetadata(ctx context.Context, root RootRecord, rootFound bool, feedType RootFeedType, cursor string) {
	if !rootFound || root.ID == "" || feedType == "" {
		return
	}

	if root.FeedType == feedType {
		return
	}

	// Optimization: dirty-path signals can arrive in large FSEvents bursts, and
	// writing the root row for every cursor would add SQLite contention before
	// the dirty batch has actually been applied. The caller already resolved the
	// root from the cache/DB once, so reuse that record and persist only the
	// stable feed type; root/full finalization records the cursor after indexing
	// reaches a conservative acknowledgement point.
	_ = cursor
	root.FeedType = feedType
	root.UpdatedAt = util.GetSystemTimestamp()
	if err := s.updateRootStateAndCache(ctx, root); err != nil {
		util.GetLogger().Warn(ctx, "filesearch failed to update root feed metadata: "+err.Error())
		return
	}
	// util.GetLogger().Debug(ctx, fmt.Sprintf(
	// 	"filesearch root feed metadata updated: root=%s path=%s feed_type=%s cursor_updated=%t",
	// 	root.ID,
	// 	root.Path,
	// 	root.FeedType,
	// 	cursor != "",
	// ))
}

func (s *Scanner) captureRootFeedSnapshot(ctx context.Context, root RootRecord) RootRecord {
	snapshotter, ok := s.changeFeed.(RootFeedSnapshotter)
	if !ok {
		if root.FeedType == "" {
			root.FeedType = RootFeedTypeFallback
		}
		if root.FeedState == "" {
			root.FeedState = RootFeedStateReady
		}
		return root
	}

	snapshot, err := snapshotter.SnapshotRootFeed(ctx, root)
	if err != nil {
		util.GetLogger().Warn(ctx, "filesearch failed to capture root feed snapshot: "+err.Error())
		if root.FeedType == "" {
			root.FeedType = RootFeedTypeFallback
		}
		if root.FeedState == "" {
			root.FeedState = RootFeedStateReady
		}
		return root
	}

	if snapshot.FeedType != "" {
		root.FeedType = snapshot.FeedType
	}
	root.FeedCursor = snapshot.FeedCursor
	if snapshot.FeedState != "" {
		root.FeedState = snapshot.FeedState
	}

	return root
}

func (s *Scanner) refreshRootFeedSnapshot(ctx context.Context, rootID string) {
	root, ok := s.findRootByID(ctx, rootID)
	if !ok {
		return
	}

	root = s.captureRootFeedSnapshot(ctx, root)
	root.UpdatedAt = util.GetSystemTimestamp()
	if err := s.updateRootStateAndCache(ctx, root); err != nil {
		util.GetLogger().Warn(ctx, "filesearch failed to persist refreshed root feed snapshot: "+err.Error())
		return
	}
	util.GetLogger().Debug(ctx, fmt.Sprintf(
		"filesearch root feed snapshot refreshed: root=%s path=%s feed_type=%s feed_state=%s",
		root.ID,
		root.Path,
		root.FeedType,
		root.FeedState,
	))
	s.emitStateChange(ctx)
}

func (s *Scanner) resetDirtyTimer(timer *time.Timer) {
	if timer == nil {
		return
	}

	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	window := s.dirtyDebounceWindow()
	if window <= 0 {
		window = defaultDirtyDebounceWindow
	}
	timer.Reset(window)
}

func (s *Scanner) dirtyDebounceWindow() time.Duration {
	if s.dirtyQueue == nil {
		return 0
	}

	stats := s.dirtyQueue.Stats()
	if stats.RootCount == 0 && stats.PathCount == 0 {
		return s.dirtyQueue.debounceWindow()
	}

	now := time.Now()
	window := s.currentDirtyDebounceWindow()
	remaining := time.Millisecond
	if !stats.LatestSignal.IsZero() {
		// Bug fix: the timer can be scheduled before a later event arrives. Recompute
		// the remaining quiet-window delay each time instead of firing with an older
		// debounce value, otherwise bursty build output still flushes too early.
		quietRemaining := window - now.Sub(stats.LatestSignal)
		if quietRemaining > 0 {
			remaining = quietRemaining
		}
	}

	maxPendingWaitWindow := s.dirtyQueue.maxPendingWaitWindow()
	if maxPendingWaitWindow <= 0 || stats.EarliestSignal.IsZero() {
		return remaining
	}

	// Bug fix: quiet-window debounce protects burst coalescing, but it must not
	// let continuous unrelated FSEvents postpone visible file updates forever.
	// Cap the next timer by the first pending signal's hard deadline so an old
	// batch is processed even while newer events keep arriving.
	maxWaitRemaining := maxPendingWaitWindow - now.Sub(stats.EarliestSignal)
	if maxWaitRemaining <= 0 {
		return time.Millisecond
	}
	return minDuration(remaining, maxWaitRemaining)
}

func (s *Scanner) currentDirtyDebounceWindow() time.Duration {
	if s.dirtyQueue == nil {
		return 0
	}

	baseWindow := s.dirtyQueue.debounceWindow()
	if baseWindow <= 0 {
		baseWindow = defaultDirtyDebounceWindow
	}
	return s.dirtyBackpressureWindow(s.dirtyQueue.Stats(), baseWindow)
}

func (s *Scanner) dirtyBackpressureWindow(stats DirtyQueueStats, baseWindow time.Duration) time.Duration {
	window := baseWindow
	maxWindow := s.dirtyQueue.config.MaxDebounceWindow
	if maxWindow <= 0 {
		maxWindow = defaultMaxDirtyDebounceWindow
	}

	pathThreshold := s.dirtyQueue.config.BackpressurePathThreshold
	rootThreshold := s.dirtyQueue.config.BackpressureRootThreshold

	pathPressureMax := pathThreshold > 0 && stats.PathCount >= pathThreshold*8
	rootPressureMax := rootThreshold > 0 && stats.RootSignalCount >= rootThreshold*4
	pathPressureHigh := pathThreshold > 0 && stats.PathCount >= pathThreshold*2
	rootPressureHigh := rootThreshold > 0 && stats.RootSignalCount >= rootThreshold*2
	pathPressureLow := pathThreshold > 0 && stats.PathCount >= pathThreshold
	// Bug fix: RootCount includes ordinary path dirties grouped by root. A Desktop
	// rename plus a small project edit can therefore look like "two roots" and
	// incorrectly trigger minute-scale backpressure, so root pressure must only
	// consider explicit root-level signals while path pressure handles file bursts.
	rootPressureLow := rootThreshold > 0 && stats.RootSignalCount >= rootThreshold

	if pathPressureMax || rootPressureMax {
		window = maxDuration(window, maxWindow)
	} else if pathPressureHigh || rootPressureHigh {
		window = maxDuration(window, minDuration(defaultDirtyPressureHighWindow, maxWindow))
	} else if pathPressureLow || rootPressureLow {
		// Optimization: keep file-search updates quiet under generated-output
		// bursts. The base dirty flush now waits long enough for normal editor
		// saves, so pressure tiers must move to minute-scale windows instead of
		// the old sub-minute values that no longer increased the debounce.
		window = maxDuration(window, minDuration(defaultDirtyPressureLowWindow, maxWindow))
	}

	if pathPressureLow || rootPressureLow {
		// Bug fix: a slow previous dirty run used to push even tiny follow-up
		// queues into minute-scale waits. Apply elapsed-time backpressure only
		// when the current queue is already large enough to be considered pressure.
		lastElapsed := s.lastDirtyRunDuration()
		if lastElapsed >= 15*time.Second {
			window = maxDuration(window, minDuration(defaultDirtyPressureHighWindow, maxWindow))
		} else if lastElapsed >= 5*time.Second {
			window = maxDuration(window, minDuration(defaultDirtyPressureLowWindow, maxWindow))
		}
	}

	if window > maxWindow {
		return maxWindow
	}
	return window
}

func (s *Scanner) recordDirtyRunElapsed(elapsed time.Duration) {
	s.dirtyBackpressureMu.Lock()
	s.lastDirtyRunElapsed = elapsed
	s.dirtyBackpressureMu.Unlock()
}

func (s *Scanner) lastDirtyRunDuration() time.Duration {
	s.dirtyBackpressureMu.Lock()
	defer s.dirtyBackpressureMu.Unlock()
	return s.lastDirtyRunElapsed
}

func maxDuration(left time.Duration, right time.Duration) time.Duration {
	if left > right {
		return left
	}
	return right
}

func minDuration(left time.Duration, right time.Duration) time.Duration {
	if left < right {
		return left
	}
	return right
}

func (s *Scanner) resetDirtyQueue() {
	s.resetDirtyQueueWithReason(context.Background(), "unspecified")
}

func (s *Scanner) resetDirtyQueueWithReason(ctx context.Context, reason string) {
	pendingRootCount, pendingPathCount := s.pendingDirtyCounts()
	util.GetLogger().Info(ctx, fmt.Sprintf(
		"filesearch dirty queue reset: reason=%s dropped_pending_roots=%d dropped_pending_paths=%d",
		reason,
		pendingRootCount,
		pendingPathCount,
	))
	s.clearTransientSyncState("")
	s.dirtyQueue = NewDirtyQueue(s.dirtyQueueConfig)
	s.refreshTransientSyncPendingCounts()
	s.emitStateChange(ctx)
}

func (s *Scanner) loadDirtyQueueContext(ctx context.Context) (map[string]int, map[string]RootRecord, map[string]int, error) {
	roots, err := s.db.ListRoots(ctx)
	if err != nil {
		return nil, nil, nil, err
	}

	rootDirectoryCounts := make(map[string]int, len(roots))
	rootsByID := make(map[string]RootRecord, len(roots))
	rootIndexByID := make(map[string]int, len(roots))
	for index, root := range roots {
		rootsByID[root.ID] = root
		rootIndexByID[root.ID] = index + 1

		directoryCount, err := s.db.CountDirectoriesByRoot(ctx, root.ID)
		if err != nil {
			return nil, nil, nil, err
		}
		rootDirectoryCounts[root.ID] = directoryCount
	}

	return rootDirectoryCounts, rootsByID, rootIndexByID, nil
}

func (s *Scanner) logRootReloadIndexSnapshot(ctx context.Context) {
	if !shouldCollectFileSearchDiagnosticSnapshot() {
		// Optimization: root reload snapshots were the hot production path in CPU
		// profiles; they are diagnostic logs, so decide before running any SQLite
		// snapshot query.
		return
	}

	snapshot, err := s.db.SearchIndexSnapshot(ctx)
	if err != nil {
		util.GetLogger().Warn(ctx, "filesearch failed to capture sqlite snapshot after root reload: "+err.Error())
		return
	}
	logSQLiteIndexSnapshot(ctx, "root_reload_complete", snapshot, false)
}

func (s *Scanner) findRootByID(ctx context.Context, rootID string) (RootRecord, bool) {
	if rootID == "" {
		return RootRecord{}, false
	}
	if root, found, loaded := s.cachedRootByID(rootID); loaded {
		if found {
			return root, true
		}
		// Optimization: a loaded root cache is a complete policy-pruned snapshot,
		// so a miss is definitive and should not fall through to another SQLite
		// lookup for every unknown watcher signal.
		return RootRecord{}, false
	}
	if s == nil || s.db == nil {
		return RootRecord{}, false
	}

	root, err := s.db.FindRootByID(ctx, rootID)
	if err != nil {
		util.GetLogger().Warn(ctx, "filesearch failed to resolve root by id: "+err.Error())
		return RootRecord{}, false
	}
	if root == nil {
		return RootRecord{}, false
	}
	s.seedRootCacheLookup(*root)
	return *root, true
}

func (s *Scanner) updateRootFeedState(ctx context.Context, rootID string, state RootFeedState) {
	root, ok := s.findRootByID(ctx, rootID)
	if !ok {
		return
	}
	if root.FeedState == state {
		return
	}
	if root.FeedType == "" {
		root.FeedType = RootFeedTypeFallback
	}
	previousState := root.FeedState
	root.FeedState = state
	root.UpdatedAt = util.GetSystemTimestamp()
	if err := s.updateRootStateAndCache(ctx, root); err != nil {
		util.GetLogger().Warn(ctx, "filesearch failed to update root feed state: "+err.Error())
		return
	}
	util.GetLogger().Info(ctx, fmt.Sprintf(
		"filesearch root feed state updated: root=%s path=%s from=%s to=%s feed_type=%s",
		root.ID,
		root.Path,
		previousState,
		root.FeedState,
		root.FeedType,
	))
	s.emitStateChange(ctx)
}

func forceReconcileBatchForFeedState(root RootRecord, batch ReconcileBatch) ReconcileBatch {
	if batch.Mode == ReconcileModeRoot {
		return batch
	}
	if root.FeedState != RootFeedStateDegraded && root.FeedState != RootFeedStateUnavailable {
		return batch
	}

	batch.Mode = ReconcileModeRoot
	batch.Paths = nil
	batch.DirectDeltas = nil
	return batch
}

func (s *Scanner) pendingDirtyCounts() (int, int) {
	if s.dirtyQueue == nil {
		return 0, 0
	}
	return s.dirtyQueue.PendingCounts()
}

func (s *Scanner) GetDirtyQueueDiagnostics(now time.Time) DirtyQueueDiagnostics {
	if s == nil {
		return DirtyQueueDiagnostics{}
	}
	if now.IsZero() {
		now = time.Now()
	}

	diagnostics := DirtyQueueDiagnostics{
		Config:              s.dirtyQueueConfig,
		LastDirtyRunElapsed: s.lastDirtyRunDuration(),
	}
	if s.dirtyQueue == nil {
		return diagnostics
	}

	stats := s.dirtyQueue.Stats()
	diagnostics.PendingRootCount = stats.RootCount
	diagnostics.PendingRootSignalCount = stats.RootSignalCount
	diagnostics.PendingPathCount = stats.PathCount
	diagnostics.EarliestSignal = stats.EarliestSignal
	diagnostics.LatestSignal = stats.LatestSignal
	// Feature addition: expose the effective dirty queue timing knobs alongside
	// raw pending counts. The counts alone cannot explain "why hasn't this file
	// flushed yet"; the active debounce window and next timer deadline show
	// whether the queue is waiting for quiet time, hard max-wait, or backpressure.
	diagnostics.CurrentDebounceWindow = s.currentDirtyDebounceWindow()
	diagnostics.NextFlushIn = s.dirtyDebounceWindow()
	if diagnostics.NextFlushIn < 0 {
		diagnostics.NextFlushIn = 0
	}

	return diagnostics
}

func (s *Scanner) refreshTransientSyncPendingCounts() {
	pendingRootCount, pendingPathCount := s.pendingDirtyCounts()
	s.updateTransientSyncPendingCounts(pendingRootCount, pendingPathCount)
}

func (s *Scanner) refreshTransientSyncPendingCountsAndReportPendingTransition() bool {
	pendingRootCount, pendingPathCount := s.pendingDirtyCounts()
	return s.updateTransientSyncPendingCounts(pendingRootCount, pendingPathCount)
}

func (s *Scanner) updateTransientSyncPendingCounts(pendingRootCount int, pendingPathCount int) bool {
	s.transientSyncMu.Lock()
	defer s.transientSyncMu.Unlock()

	wasPending := false
	if s.transientSyncState != nil {
		wasPending = s.transientSyncState.PendingRootCount > 0 || s.transientSyncState.PendingPathCount > 0
	}
	isPending := pendingRootCount > 0 || pendingPathCount > 0

	if pendingRootCount == 0 && pendingPathCount == 0 {
		if s.transientSyncState == nil || s.transientSyncState.Root.ID == "" {
			s.transientSyncState = nil
		}
		return false
	}

	if s.transientSyncState == nil {
		s.transientSyncState = &TransientSyncState{}
	}

	s.transientSyncState.PendingRootCount = pendingRootCount
	s.transientSyncState.PendingPathCount = pendingPathCount
	return !wasPending && isPending
}

func (s *Scanner) requeueDirtyBatches(ctx context.Context, batches []ReconcileBatch) {
	if len(batches) > 0 {
		util.GetLogger().Info(ctx, fmt.Sprintf("filesearch requeueing dirty batches: batches=%d", len(batches)))
	}
	requeuedAt := time.Now()
	for _, batch := range batches {
		batchCtx := contextWithTraceID(ctx, batch.TraceID)
		util.GetLogger().Debug(batchCtx, fmt.Sprintf(
			"filesearch requeue dirty batch: root=%s mode=%s paths=%s direct_deltas=%d",
			batch.RootID,
			batch.Mode,
			summarizeLogPaths(batch.Paths),
			len(batch.DirectDeltas),
		))
		switch batch.Mode {
		case ReconcileModeRoot:
			s.enqueueDirtyWithContext(batchCtx, DirtySignal{
				Kind:          DirtySignalKindRoot,
				RootID:        batch.RootID,
				PathIsDir:     true,
				PathTypeKnown: true,
				At:            requeuedAt,
			})
		case ReconcileModeSubtree:
			for _, path := range batch.Paths {
				s.enqueueDirtyWithContext(batchCtx, DirtySignal{
					Kind:          DirtySignalKindPath,
					RootID:        batch.RootID,
					Path:          path,
					PathIsDir:     true,
					PathTypeKnown: true,
					At:            requeuedAt,
				})
			}
		case ReconcileModeDirectDelta:
			for _, delta := range batch.DirectDeltas {
				s.enqueueDirtyWithContext(batchCtx, DirtySignal{
					Kind:          DirtySignalKindPath,
					SemanticKind:  delta.SemanticKind,
					RootID:        batch.RootID,
					Path:          delta.Path,
					PathIsDir:     delta.PathIsDir,
					PathTypeKnown: delta.PathTypeKnown,
					At:            requeuedAt,
				})
			}
		}
	}
}

func (s *Scanner) findRootForPath(ctx context.Context, path string) (RootRecord, bool) {
	roots, err := s.db.ListRoots(ctx)
	if err != nil {
		util.GetLogger().Warn(ctx, "filesearch failed to resolve dirty root: "+err.Error())
		return RootRecord{}, false
	}

	return findRootForPathInRoots(roots, path)
}

func newTransientSyncState(root RootRecord, rootIndex int, rootTotal int, batch ReconcileBatch, pendingRootCount int, pendingPathCount int) TransientSyncState {
	progressTotal := int64(batch.DirtyPathCount)
	if progressTotal <= 0 {
		progressTotal = int64(len(batch.Paths))
	}
	if progressTotal <= 0 {
		progressTotal = 1
	}

	root.Status = RootStatusSyncing
	root.ProgressCurrent = 0
	root.ProgressTotal = progressTotal
	root.LastError = nil
	root.UpdatedAt = util.GetSystemTimestamp()

	return TransientSyncState{
		Root:             root,
		RootIndex:        rootIndex,
		RootTotal:        rootTotal,
		Mode:             batch.Mode,
		ScopeCount:       len(batch.Paths),
		DirtyPathCount:   batch.DirtyPathCount,
		PendingRootCount: pendingRootCount,
		PendingPathCount: pendingPathCount,
	}
}

type scanState struct {
	path   string
	policy TraversalPolicyContext
}

type plannedDirectory struct {
	path       string
	childCount int
	policy     TraversalPolicyContext
}

type scanPlan struct {
	directories    []plannedDirectory
	DirectoryTotal int
	TotalItems     int64
}

func newEntryRecord(root RootRecord, fullPath string, info os.FileInfo) EntryRecord {
	return newEntryRecordWithUpdatedAt(root, fullPath, info, util.GetSystemTimestamp())
}

func newEntryRecordWithUpdatedAt(root RootRecord, fullPath string, info os.FileInfo, updatedAt int64) EntryRecord {
	pinyinFull, pinyinInitials := buildPinyinFields(info.Name())
	return EntryRecord{
		Path:           fullPath,
		RootID:         root.ID,
		ParentPath:     filepath.Dir(fullPath),
		Name:           info.Name(),
		NormalizedName: strings.ToLower(info.Name()),
		NormalizedPath: normalizePath(fullPath),
		PinyinFull:     pinyinFull,
		PinyinInitials: pinyinInitials,
		IsDir:          info.IsDir(),
		Mtime:          info.ModTime().UnixMilli(),
		Size:           info.Size(),
		UpdatedAt:      updatedAt,
	}
}

func buildDirectorySnapshotRecords(root RootRecord, plan scanPlan, scanTimestamp int64) []DirectoryRecord {
	directories := make([]DirectoryRecord, 0, len(plan.directories))
	for _, plannedDirectory := range plan.directories {
		directories = append(directories, DirectoryRecord{
			Path:         plannedDirectory.path,
			RootID:       root.ID,
			ParentPath:   filepath.Dir(plannedDirectory.path),
			LastScanTime: scanTimestamp,
			Exists:       true,
		})
	}
	return directories
}

func shouldSkipSystemPath(fullPath string, isDir bool) bool {
	_ = isDir
	return isWoxFileSearchStoragePath(fullPath)
}

func isWoxFileSearchStoragePath(fullPath string) bool {
	cleanStoragePath := cachedWoxFileSearchStoragePath()
	if cleanStoragePath == "" {
		return false
	}

	cleanPath := filepath.Clean(strings.TrimSpace(fullPath))
	if cleanPath == "" || cleanPath == "." {
		return false
	}

	// Bug fix: Wox's own File Search SQLite files live under ~/.wox/filesearch.
	// Treat that subtree as an engine-level internal path so full scans and change
	// feeds both skip the storage before it can enqueue work against itself.
	return pathWithinScope(cleanStoragePath, cleanPath)
}
