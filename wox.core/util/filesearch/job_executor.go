package filesearch

import (
	"context"
	"fmt"
	"wox/util"
)

type JobExecutor struct {
	snapshot           *SnapshotBuilder
	apply              func(context.Context, RootRecord, Job, *SubtreeSnapshotBatch) error
	applySubtreeBatch  func(context.Context, RootRecord, []SubtreeSnapshotBatch) error
	streamDirectFiles  func(context.Context, RootRecord, Job, *SnapshotBuilder, func(JobApplyStats)) (JobApplyStats, error)
	streamSubtree      func(context.Context, RootRecord, Job, *SnapshotBuilder, func(JobApplyStats)) (JobApplyStats, error)
	subtreeBatchConfig subtreeApplyBatchConfig
}

type subtreeApplyBatchConfig struct {
	MaxJobCount        int
	MaxJobTotalUnits   int64
	MaxBatchTotalUnits int64
}

func defaultFullRunSubtreeApplyBatchConfig() subtreeApplyBatchConfig {
	// Real C:\dev traces showed many tiny subtree scopes paying a near-constant
	// SQLite diff/commit cost. We only batch those small full-run scopes, cap
	// the group tightly, and keep each original scope as its own snapshot so the
	// delete/update ownership boundary stays unchanged.
	return subtreeApplyBatchConfig{
		MaxJobCount:        16,
		MaxJobTotalUnits:   128,
		MaxBatchTotalUnits: 1024,
	}
}

func normalizeSubtreeApplyBatchConfig(config subtreeApplyBatchConfig) subtreeApplyBatchConfig {
	if config.MaxJobCount < 2 {
		return subtreeApplyBatchConfig{}
	}
	if config.MaxJobTotalUnits <= 0 || config.MaxBatchTotalUnits <= 0 {
		return subtreeApplyBatchConfig{}
	}
	return config
}

func (c subtreeApplyBatchConfig) enabled() bool {
	return c.MaxJobCount >= 2 && c.MaxJobTotalUnits > 0 && c.MaxBatchTotalUnits > 0
}

func (c subtreeApplyBatchConfig) allows(job Job, pendingCount int, pendingUnits int64) bool {
	if !c.enabled() {
		return false
	}
	jobUnits := job.PlannedTotalUnits
	if jobUnits <= 0 {
		jobUnits = job.PlannedWriteUnits
	}
	if jobUnits <= 0 {
		jobUnits = 1
	}
	if jobUnits > c.MaxJobTotalUnits {
		return false
	}
	if pendingCount >= c.MaxJobCount {
		return false
	}
	return pendingUnits+jobUnits <= c.MaxBatchTotalUnits
}

func NewJobExecutor(snapshot *SnapshotBuilder) *JobExecutor {
	if snapshot == nil {
		snapshot = NewSnapshotBuilder(nil)
	}
	return &JobExecutor{snapshot: snapshot}
}

func (e *JobExecutor) SetApplyFunc(apply func(context.Context, RootRecord, Job, *SubtreeSnapshotBatch) error) {
	if e == nil {
		return
	}
	e.apply = apply
}

func (e *JobExecutor) SetSubtreeBatchApplyFunc(apply func(context.Context, RootRecord, []SubtreeSnapshotBatch) error) {
	if e == nil {
		return
	}
	e.applySubtreeBatch = apply
}

func (e *JobExecutor) SetSubtreeBatchConfig(config subtreeApplyBatchConfig) {
	if e == nil {
		return
	}
	e.subtreeBatchConfig = normalizeSubtreeApplyBatchConfig(config)
}

func (e *JobExecutor) SetDirectFilesStreamFunc(stream func(context.Context, RootRecord, Job, *SnapshotBuilder, func(JobApplyStats)) (JobApplyStats, error)) {
	if e == nil {
		return
	}
	e.streamDirectFiles = stream
}

func (e *JobExecutor) SetSubtreeStreamFunc(stream func(context.Context, RootRecord, Job, *SnapshotBuilder, func(JobApplyStats)) (JobApplyStats, error)) {
	if e == nil {
		return
	}
	e.streamSubtree = stream
}

func (e *JobExecutor) ExecuteRun(ctx context.Context, plan RunPlan, roots []RootRecord, onSnapshot func(StatusSnapshot, Job)) (Run, []Job, error) {
	if e == nil {
		e = NewJobExecutor(nil)
	}
	if e.snapshot == nil {
		e.snapshot = NewSnapshotBuilder(nil)
	}

	rootByID := make(map[string]RootRecord, len(roots))
	for _, root := range roots {
		rootByID[root.ID] = root
	}

	rootOrder := make(map[string]int, len(plan.RootPlans))
	for index, rootPlan := range plan.RootPlans {
		rootOrder[rootPlan.RootID] = index + 1
	}

	run := Run{
		RunID:          plan.RunID,
		PlanID:         plan.PlanID,
		Status:         RunStatusExecuting,
		Stage:          RunStageExecuting,
		TotalWorkUnits: plan.TotalWorkUnits,
	}

	jobs := make([]Job, len(plan.Jobs))
	copy(jobs, plan.Jobs)
	var lastJob Job
	type pendingSubtreeApply struct {
		root       RootRecord
		jobIndexes []int
		batches    []SubtreeSnapshotBatch
		stats      []JobApplyStats
		totalUnits int64
	}
	var pending pendingSubtreeApply

	resetPendingJobs := func() {
		for _, index := range pending.jobIndexes {
			if index < 0 || index >= len(jobs) {
				continue
			}
			jobs[index].Status = JobStatusPending
		}
	}
	clearPending := func() {
		pending = pendingSubtreeApply{}
	}
	failRunForJob := func(job *Job, rootID string, err error) (Run, []Job, error) {
		err = &runRootError{RootID: rootID, Err: err}
		job.Status = JobStatusFailed
		run.Status = RunStatusFailed
		run.LastError = err.Error()
		emitJobExecutorSnapshot(run, plan, rootOrder, *job, onSnapshot)
		return run, jobs, err
	}
	flushPending := func() (Run, []Job, error) {
		if len(pending.jobIndexes) == 0 {
			return run, jobs, nil
		}

		// Buffered subtree scopes still keep their original scope paths and are
		// only delayed until this flush point. Jobs do not become completed until
		// the combined SQLite write succeeds, which keeps retries conservative.
		if len(pending.jobIndexes) == 1 || e.applySubtreeBatch == nil {
			index := pending.jobIndexes[0]
			job := &jobs[index]
			applyStartedAt := util.GetSystemTimestamp()
			if err := e.apply(ctx, pending.root, *job, &pending.batches[0]); err != nil {
				logFilesearchJobPhase(ctx, pending.root, *job, "apply_snapshot", util.GetSystemTimestamp()-applyStartedAt)
				clearPending()
				return failRunForJob(job, job.RootID, err)
			}
			logFilesearchJobPhase(ctx, pending.root, *job, "apply_snapshot", util.GetSystemTimestamp()-applyStartedAt)
		} else {
			applyStartedAt := util.GetSystemTimestamp()
			err := e.applySubtreeBatch(ctx, pending.root, pending.batches)
			elapsed := util.GetSystemTimestamp() - applyStartedAt
			for _, index := range pending.jobIndexes {
				logFilesearchJobPhase(ctx, pending.root, jobs[index], "apply_snapshot_batched", elapsed)
			}
			if err != nil {
				resetPendingJobs()
				lastIndex := pending.jobIndexes[len(pending.jobIndexes)-1]
				job := &jobs[lastIndex]
				clearPending()
				return failRunForJob(job, job.RootID, err)
			}
		}

		for _, index := range pending.jobIndexes {
			job := &jobs[index]
			job.Status = JobStatusCompleted
			run.CompletedWorkUnits += job.PlannedTotalUnits
			if indexOffset := indexOfPendingJob(pending.jobIndexes, index); indexOffset >= 0 && indexOffset < len(pending.stats) {
				addJobApplyStatsToRun(&run, pending.stats[indexOffset])
			}
			lastJob = *job
			emitJobExecutorSnapshot(run, plan, rootOrder, *job, onSnapshot)
		}
		clearPending()
		return run, jobs, nil
	}
	shouldBufferSubtreeJob := func(root RootRecord, job Job) bool {
		if plan.Kind != RunKindFull || job.Kind != JobKindSubtree {
			return false
		}
		if e.applySubtreeBatch == nil || !e.subtreeBatchConfig.enabled() {
			return false
		}
		if len(pending.jobIndexes) > 0 && pending.root.ID != root.ID {
			return false
		}
		return e.subtreeBatchConfig.allows(job, len(pending.jobIndexes), pending.totalUnits)
	}
	shouldFlushBeforeJob := func(root RootRecord, job Job) bool {
		if len(pending.jobIndexes) == 0 {
			return false
		}
		return !shouldBufferSubtreeJob(root, job)
	}
	emitStreamingStats := func(job Job, currentStats JobApplyStats) {
		progressRun := run
		// Streaming jobs can spend most of a full index inside one large scope.
		// Emit batch-level file counts through the normal snapshot path so the
		// toolbar updates while the job is running instead of waiting for the job
		// completion boundary.
		progressRun.CompletedEntryCount += currentStats.EntryCount
		progressRun.CompletedFileCount += currentStats.FileCount
		emitJobExecutorSnapshot(progressRun, plan, rootOrder, job, onSnapshot)
	}

	for index := range jobs {
		select {
		case <-ctx.Done():
			resetPendingJobs()
			run.Status = RunStatusCanceled
			run.LastError = ctx.Err().Error()
			return run, jobs, ctx.Err()
		default:
		}

		job := &jobs[index]
		root, ok := rootByID[job.RootID]
		if !ok {
			err := &runRootError{
				RootID: job.RootID,
				Err:    fmt.Errorf("root %q not found for job %q", job.RootID, job.JobID),
			}
			resetPendingJobs()
			job.Status = JobStatusFailed
			run.Status = RunStatusFailed
			run.LastError = err.Error()
			emitJobExecutorSnapshot(run, plan, rootOrder, *job, onSnapshot)
			return run, jobs, err
		}

		if shouldFlushBeforeJob(root, *job) {
			flushedRun, flushedJobs, err := flushPending()
			run, jobs = flushedRun, flushedJobs
			if err != nil {
				return run, jobs, err
			}
		}

		lastJob = *job
		job.Status = JobStatusRunning
		run.ActiveJobID = job.JobID
		run.Status = RunStatusExecuting
		run.Stage = RunStageExecuting
		if job.Kind == JobKindFinalizeRoot {
			// Consumers need to see finalizing while the finalize job is active,
			// not only after it finishes, otherwise the last root still looks like
			// generic execution right before the run completes.
			run.Status = RunStatusFinalizing
			run.Stage = RunStageFinalizing
		}
		emitJobExecutorSnapshot(run, plan, rootOrder, *job, onSnapshot)

		if job.Kind == JobKindDirectFiles && e.streamDirectFiles != nil {
			// Direct-files jobs keep delete ownership at directory scope. Streaming
			// them directly avoids rebuilding the earlier whole-directory memory
			// spike while preserving that single authoritative prune boundary.
			// Root-level totals were not enough to explain where long runs stalled,
			// so each job now records its own apply duration to separate snapshot
			// building cost from SQLite write cost when one scope becomes hot.
			streamStartedAt := util.GetSystemTimestamp()
			stats, err := e.streamDirectFiles(ctx, root, *job, e.snapshot, func(currentStats JobApplyStats) {
				emitStreamingStats(*job, currentStats)
			})
			if err != nil {
				logFilesearchJobPhase(ctx, root, *job, "stream_apply", util.GetSystemTimestamp()-streamStartedAt)
				err = &runRootError{RootID: job.RootID, Err: err}
				job.Status = JobStatusFailed
				run.Status = RunStatusFailed
				run.LastError = err.Error()
				emitJobExecutorSnapshot(run, plan, rootOrder, *job, onSnapshot)
				return run, jobs, err
			}
			addJobApplyStatsToRun(&run, stats)
			logFilesearchJobPhase(ctx, root, *job, "stream_apply", util.GetSystemTimestamp()-streamStartedAt)
		} else if job.Kind == JobKindSubtree && e.streamSubtree != nil {
			// Full-run subtree jobs can now own a large root without an exact
			// recursive count. Streaming the recursive snapshot directly into SQLite keeps
			// the old scoped replace semantics while removing the planner's duplicate
			// filesystem traversal.
			streamStartedAt := util.GetSystemTimestamp()
			stats, err := e.streamSubtree(ctx, root, *job, e.snapshot, func(currentStats JobApplyStats) {
				emitStreamingStats(*job, currentStats)
			})
			if err != nil {
				logFilesearchJobPhase(ctx, root, *job, "stream_apply", util.GetSystemTimestamp()-streamStartedAt)
				err = &runRootError{RootID: job.RootID, Err: err}
				job.Status = JobStatusFailed
				run.Status = RunStatusFailed
				run.LastError = err.Error()
				emitJobExecutorSnapshot(run, plan, rootOrder, *job, onSnapshot)
				return run, jobs, err
			}
			addJobApplyStatsToRun(&run, stats)
			logFilesearchJobPhase(ctx, root, *job, "stream_apply", util.GetSystemTimestamp()-streamStartedAt)
		} else {
			buildStartedAt := util.GetSystemTimestamp()
			batch, err := e.buildJobSnapshot(ctx, root, *job)
			logFilesearchJobPhase(ctx, root, *job, "build_snapshot", util.GetSystemTimestamp()-buildStartedAt)
			if err != nil {
				resetPendingJobs()
				err = &runRootError{RootID: job.RootID, Err: err}
				job.Status = JobStatusFailed
				run.Status = RunStatusFailed
				run.LastError = err.Error()
				emitJobExecutorSnapshot(run, plan, rootOrder, *job, onSnapshot)
				return run, jobs, err
			}
			if shouldBufferSubtreeJob(root, *job) {
				// The executor buffers only after the snapshot has been built for
				// this exact scope, so the later batch flush can reuse the same
				// per-scope payload without widening any delete or stale-prune area.
				pending.root = root
				pending.jobIndexes = append(pending.jobIndexes, index)
				pending.batches = append(pending.batches, *batch)
				pending.stats = append(pending.stats, jobApplyStatsFromBatch(*batch))
				pending.totalUnits += maxSubtreeBatchUnits(*job)
				job.Status = JobStatusPending
				continue
			}
			if e.apply != nil {
				applyStartedAt := util.GetSystemTimestamp()
				if err := e.apply(ctx, root, *job, batch); err != nil {
					logFilesearchJobPhase(ctx, root, *job, "apply_snapshot", util.GetSystemTimestamp()-applyStartedAt)
					err = &runRootError{RootID: job.RootID, Err: err}
					job.Status = JobStatusFailed
					run.Status = RunStatusFailed
					run.LastError = err.Error()
					emitJobExecutorSnapshot(run, plan, rootOrder, *job, onSnapshot)
					return run, jobs, err
				}
				logFilesearchJobPhase(ctx, root, *job, "apply_snapshot", util.GetSystemTimestamp()-applyStartedAt)
			}
			if batch != nil {
				addJobApplyStatsToRun(&run, jobApplyStatsFromBatch(*batch))
			}
		}

		// Run-scoped progress must advance from sealed work totals instead of
		// resetting per root. Using the plan's fixed unit budget keeps progress
		// monotonic even when execution crosses from one root's last job into the
		// next root's first job.
		job.Status = JobStatusCompleted
		run.CompletedWorkUnits += job.PlannedTotalUnits
		lastJob = *job
		emitJobExecutorSnapshot(run, plan, rootOrder, *job, onSnapshot)
	}

	flushedRun, flushedJobs, err := flushPending()
	run, jobs = flushedRun, flushedJobs
	if err != nil {
		return run, jobs, err
	}

	run.Status = RunStatusCompleted
	run.ActiveJobID = ""
	lastJob.Status = ""
	emitJobExecutorSnapshot(run, plan, rootOrder, lastJob, onSnapshot)
	return run, jobs, nil
}

func addJobApplyStatsToRun(run *Run, stats JobApplyStats) {
	if run == nil {
		return
	}
	// Full-run toolbar counts used to come from planner estimates, which are
	// deliberately zero for streaming runs to avoid a duplicate recursive walk.
	// Accumulating the stats after a job successfully applies keeps the visible
	// file count tied to persisted work instead of planner estimates.
	run.CompletedEntryCount += stats.EntryCount
	run.CompletedFileCount += stats.FileCount
}

func indexOfPendingJob(indexes []int, target int) int {
	for index, value := range indexes {
		if value == target {
			return index
		}
	}
	return -1
}

func maxSubtreeBatchUnits(job Job) int64 {
	units := job.PlannedTotalUnits
	if units <= 0 {
		units = job.PlannedWriteUnits
	}
	if units <= 0 {
		return 1
	}
	return units
}

func (e *JobExecutor) buildJobSnapshot(ctx context.Context, root RootRecord, job Job) (*SubtreeSnapshotBatch, error) {
	switch job.Kind {
	case JobKindDirectFiles:
		batch, err := e.snapshot.BuildDirectFilesJobSnapshot(ctx, root, job)
		if err != nil {
			return nil, err
		}
		return &batch, nil
	case JobKindSubtree:
		batch, err := e.snapshot.BuildSubtreeJobSnapshot(ctx, root, job)
		if err != nil {
			return nil, err
		}
		return &batch, nil
	case JobKindFinalizeRoot:
		return nil, nil
	default:
		return nil, fmt.Errorf("unsupported job kind %q", job.Kind)
	}
}

func emitJobExecutorSnapshot(run Run, plan RunPlan, rootOrder map[string]int, job Job, onSnapshot func(StatusSnapshot, Job)) {
	if onSnapshot == nil {
		return
	}
	onSnapshot(buildJobExecutorStatusSnapshot(run, plan, rootOrder, job), job)
}

func buildJobExecutorStatusSnapshot(run Run, plan RunPlan, rootOrder map[string]int, job Job) StatusSnapshot {
	rootStatus := RootStatusScanning
	if job.Kind == JobKindFinalizeRoot {
		rootStatus = RootStatusFinalizing
	}
	activeProgressCurrent, activeProgressTotal := activeJobProgress(job)

	// The previous root-local progress view could show regressions when the next
	// root started at zero. Mirroring the run's sealed unit counters into the
	// exported run-progress fields makes the global progress bar monotonic across
	// job and root boundaries while preserving the legacy active-progress fields
	// as the scoped progress of the current job/root.
	return StatusSnapshot{
		RootCount:             len(plan.RootPlans),
		ProgressCurrent:       run.CompletedWorkUnits,
		ProgressTotal:         run.TotalWorkUnits,
		ActiveRootStatus:      rootStatus,
		ActiveProgressCurrent: activeProgressCurrent,
		ActiveProgressTotal:   activeProgressTotal,
		ActiveRootIndex:       rootOrder[job.RootID],
		ActiveRootTotal:       len(plan.RootPlans),
		ActiveRootPath:        job.RootPath,
		ActiveRunStatus:       run.Status,
		ActiveRunKind:         plan.Kind,
		ActiveJobKind:         job.Kind,
		ActiveScopePath:       job.ScopePath,
		ActiveStage:           run.Stage,
		RunProgressCurrent:    run.CompletedWorkUnits,
		RunProgressTotal:      run.TotalWorkUnits,
		ActiveRunFileCount:    run.CompletedFileCount,
		ActiveRunEntryCount:   run.CompletedEntryCount,
		IsIndexing:            run.Status == RunStatusExecuting || run.Status == RunStatusFinalizing,
		LastError:             run.LastError,
	}
}

func activeJobProgress(job Job) (int64, int64) {
	total := job.PlannedTotalUnits
	if total < 0 {
		total = 0
	}
	current := int64(0)
	if job.Status == JobStatusCompleted {
		current = total
	}
	return current, total
}
