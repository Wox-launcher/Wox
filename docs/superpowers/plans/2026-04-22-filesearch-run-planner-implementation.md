# File Search Run Planner Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the current root-centric filesearch indexing loop with a sealed `RunPlan -> RootPlan -> Job -> Executor` pipeline so huge roots are split into bounded jobs, indexing progress becomes global and monotonic, and incremental work queues behind the active run instead of mutating it.

**Architecture:** Keep `RootRecord` as the persisted root configuration, but move execution ownership into an in-memory run planner and executor. Planning and pre-scan freeze the denominator up front, execution consumes bounded jobs in order, and root-level feed cursor/state updates remain conservative and idempotent.

**Tech Stack:** Go, SQLite + existing filesearch schema, current `DirtyQueue` / change-feed capture, SQLite-first search provider, existing filesearch smoke/benchmark coverage

---

## File Structure

**Create**
- `/mnt/c/dev/Wox/wox.core/util/filesearch/run_plan.go`
  - Define `RunPlan`, `Run`, `RootPlan`, `ScopeNode`, `Job`, job/status enums, and progress snapshot helpers.
- `/mnt/c/dev/Wox/wox.core/util/filesearch/run_planner.go`
  - Build full and incremental run plans, own global planning/pre-scan flow, and seal immutable job slices.
- `/mnt/c/dev/Wox/wox.core/util/filesearch/job_executor.go`
  - Execute jobs sequentially, build bounded snapshots per job, and report stage-aware global progress.
- `/mnt/c/dev/Wox/wox.core/util/filesearch/run_planner_test.go`
  - Cover split policy, immutable plan behavior, direct-files chunking, and incremental queueing semantics.
- `/mnt/c/dev/Wox/wox.core/util/filesearch/job_executor_test.go`
  - Cover job execution ordering, monotonic progress, and conservative finalize/cursor rules.

**Modify**
- `/mnt/c/dev/Wox/wox.core/util/filesearch/types.go`
  - Extend or reshape `StatusSnapshot` so top-level progress comes from active run state without breaking plugin callers.
- `/mnt/c/dev/Wox/wox.core/util/filesearch/engine.go`
  - Read active run progress first, then fall back to root aggregates when no run is active.
- `/mnt/c/dev/Wox/wox.core/util/filesearch/scanner.go`
  - Replace full-scan root loop with `RunPlanner + JobExecutor`; keep change-feed capture, but stop treating one root as one execution unit.
- `/mnt/c/dev/Wox/wox.core/util/filesearch/reconciler.go`
  - Replace direct reconcile execution with incremental run planning and queue handoff.
- `/mnt/c/dev/Wox/wox.core/util/filesearch/snapshot_builder.go`
  - Add bounded scope/job snapshot builders so execution only materializes the active job instead of the whole root.
- `/mnt/c/dev/Wox/wox.core/util/filesearch/filesearch_db.go`
  - Add job-oriented write/finalize APIs and keep root cursor advancement in a dedicated finalize step.
- `/mnt/c/dev/Wox/wox.core/util/filesearch/filesearch_sqlite_storage.go`
  - Support job-scoped write progress and finalize-time WAL/checkpoint hooks without reintroducing whole-root rebuilds.
- `/mnt/c/dev/Wox/wox.core/util/filesearch/logging_helpers.go`
  - Log run/job/stage progress with stable totals and preserve SQLite snapshot diagnostics.
- `/mnt/c/dev/Wox/wox.core/util/filesearch/scanner_full_scan_test.go`
  - Convert existing full-scan expectations to run-based progress and bounded-job execution.
- `/mnt/c/dev/Wox/wox.core/util/filesearch/scanner_incremental_test.go`
  - Cover queued incremental runs, fail-fast semantics, and root cursor/finalize behavior.
- `/mnt/c/dev/Wox/wox.core/util/filesearch/filesearch_benchmark_test.go`
  - Add a benchmark that compares whole-root execution against bounded jobs for large scopes.

---

### Task 1: Introduce RunPlan and Job Types

**Files:**
- Create: `/mnt/c/dev/Wox/wox.core/util/filesearch/run_plan.go`
- Modify: `/mnt/c/dev/Wox/wox.core/util/filesearch/types.go`
- Test: `/mnt/c/dev/Wox/wox.core/util/filesearch/run_planner_test.go`

- [ ] Add a failing test that seals a `RunPlan` and verifies its jobs, totals, and root plans do not change when the builder mutates the original planning buffers afterward.
- [ ] Run: `cd /mnt/c/dev/Wox/wox.core && go test -tags sqlite_fts5 ./util/filesearch -run 'TestRunPlanSealFreezesWorkload' -count=1`
- [ ] Implement `RunPlan`, `Run`, `RootPlan`, `ScopeNode`, `Job`, and run/job status enums in `run_plan.go`.
- [ ] Extend `StatusSnapshot` with run-scoped fields (`ActiveRunStatus`, `ActiveJobKind`, `ActiveScopePath`, `ActiveStage`, `RunProgressCurrent`, `RunProgressTotal`) while preserving existing fields for compatibility.
- [ ] Add English intent comments next to the new structs and status fields explaining that root-local progress was not sufficient because one logical root can now fan out into many execution jobs.
- [ ] Run: `cd /mnt/c/dev/Wox/wox.core && go test -tags sqlite_fts5 ./util/filesearch -run 'TestRunPlanSealFreezesWorkload' -count=1`

### Task 2: Build Full-Run Planning and Recursive Split Policy

**Files:**
- Create: `/mnt/c/dev/Wox/wox.core/util/filesearch/run_planner.go`
- Test: `/mnt/c/dev/Wox/wox.core/util/filesearch/run_planner_test.go`
- Modify: `/mnt/c/dev/Wox/wox.core/util/filesearch/policy.go`

- [ ] Add failing tests for:
  - a small root staying `single`
  - a huge root splitting into multiple jobs
  - a wide directory chunking direct files even when it has no deeper subdirectories
- [ ] Run: `cd /mnt/c/dev/Wox/wox.core && go test -tags sqlite_fts5 ./util/filesearch -run 'TestRunPlanner(BuildsSingleRootPlan|SplitsLargeRootIntoLeafJobs|ChunksWideDirectFiles)' -count=1`
- [ ] Implement `RunPlanner.PlanFullRun(...)` with explicit `planning -> pre-scan -> seal` phases.
- [ ] Add a split-budget struct in `policy.go` for v1 constants only; do not expose user settings.
- [ ] During pre-scan, count directories/files/indexable entries exactly but do not build `EntryRecord` slices. Add comments explaining that v1 accepts double metadata I/O to guarantee monotonic progress.
- [ ] Release scope-tree buffers after `RunPlan` sealing so the planner does not hold the same giant root in memory through execution.
- [ ] Run: `cd /mnt/c/dev/Wox/wox.core && go test -tags sqlite_fts5 ./util/filesearch -run 'TestRunPlanner(BuildsSingleRootPlan|SplitsLargeRootIntoLeafJobs|ChunksWideDirectFiles)' -count=1`

### Task 3: Add Sequential Job Executor and Global Progress Tracker

**Files:**
- Create: `/mnt/c/dev/Wox/wox.core/util/filesearch/job_executor.go`
- Modify: `/mnt/c/dev/Wox/wox.core/util/filesearch/snapshot_builder.go`
- Test: `/mnt/c/dev/Wox/wox.core/util/filesearch/job_executor_test.go`

- [ ] Add failing tests for:
  - job execution preserving `order_index`
  - global progress never decreasing when execution crosses root boundaries
  - `99%` only appearing after the remaining planned work is small
- [ ] Run: `cd /mnt/c/dev/Wox/wox.core && go test -tags sqlite_fts5 ./util/filesearch -run 'TestJobExecutor(OrderIsStable|ProgressNeverDecreasesAcrossRoots|NinetyNinePercentMeansSmallKnownRemainder)' -count=1`
- [ ] Implement `JobExecutor.ExecuteRun(...)` so it consumes jobs in slice order and emits run-scoped progress snapshots.
- [ ] Refactor `SnapshotBuilder` to support job-bounded builders (`BuildDirectFilesJobSnapshot`, `BuildSubtreeJobSnapshot`) instead of whole-root-only materialization.
- [ ] Add comments near the bounded snapshot builders explaining why whole-root snapshot accumulation caused the previous indexing-time memory spike.
- [ ] Run: `cd /mnt/c/dev/Wox/wox.core && go test -tags sqlite_fts5 ./util/filesearch -run 'TestJobExecutor(OrderIsStable|ProgressNeverDecreasesAcrossRoots|NinetyNinePercentMeansSmallKnownRemainder)' -count=1`

### Task 4: Make the Database Layer Job-Oriented and Finalize Cursors Conservatively

**Files:**
- Modify: `/mnt/c/dev/Wox/wox.core/util/filesearch/filesearch_db.go`
- Modify: `/mnt/c/dev/Wox/wox.core/util/filesearch/filesearch_sqlite_storage.go`
- Test: `/mnt/c/dev/Wox/wox.core/util/filesearch/job_executor_test.go`
- Test: `/mnt/c/dev/Wox/wox.core/util/filesearch/filesearch_db_test.go`

- [ ] Add failing tests for:
  - applying one bounded job without deleting unaffected sibling scopes
  - advancing the feed cursor only in `FinalizeRootJob`
  - crash-safe replay semantics where entries may be written before cursor advancement, but unseen changes are never skipped
- [ ] Run: `cd /mnt/c/dev/Wox/wox.core && go test -tags sqlite_fts5 ./util/filesearch -run 'Test(JobExecutorFinalizeRootAdvancesCursorAfterPriorJobs|FileSearchDBApplyJobPreservesSiblingScopes|FileSearchDBFinalizeRootCursorIsConservative)' -count=1`
- [ ] Introduce job-oriented DB entry points such as `ApplyDirectFilesJob`, `ApplySubtreeJob`, and `FinalizeRootRun` while reusing current subtree/root write helpers internally where that keeps the diff small.
- [ ] Keep root cursor advancement in `FinalizeRootRun` only. Add English comments near the cursor write path explaining that replay is acceptable but skipping unseen signals is not.
- [ ] Add a finalize-time WAL maintenance hook (`checkpoint`/`optimize` only where already justified by current SQLite-first flow); do not reintroduce per-row trigger logic or whole-db rebuilds.
- [ ] Run: `cd /mnt/c/dev/Wox/wox.core && go test -tags sqlite_fts5 ./util/filesearch -run 'Test(JobExecutorFinalizeRootAdvancesCursorAfterPriorJobs|FileSearchDBApplyJobPreservesSiblingScopes|FileSearchDBFinalizeRootCursorIsConservative)' -count=1`

### Task 5: Replace Full-Scan Root Loop with RunPlanner + JobExecutor

**Files:**
- Modify: `/mnt/c/dev/Wox/wox.core/util/filesearch/scanner.go`
- Modify: `/mnt/c/dev/Wox/wox.core/util/filesearch/engine.go`
- Modify: `/mnt/c/dev/Wox/wox.core/util/filesearch/logging_helpers.go`
- Test: `/mnt/c/dev/Wox/wox.core/util/filesearch/scanner_full_scan_test.go`

- [ ] Add failing smoke coverage for:
  - full scan using run progress instead of root-local progress
  - toolbar-visible stage transitions across `planning`, `pre_scan`, `executing`, `finalizing`
  - huge root execution producing multiple jobs while keeping one persisted root identity
- [ ] Run: `cd /mnt/c/dev/Wox/wox.core && go test -tags sqlite_fts5 ./util/filesearch -run 'TestScanner(FullScanUsesGlobalRunProgress|FullScanReportsAllRunStages|FullScanSplitsLargeRootWithoutChangingRootIdentity)' -count=1`
- [ ] Replace `scanAllRootsWithReason`'s direct root loop with:
  - planner creation
  - sealed full `RunPlan`
  - sequential `JobExecutor`
  - run-finalize cleanup
- [ ] Update `Engine.GetStatus` to prefer active run progress and activity labels while keeping root aggregate counts as diagnostics.
- [ ] Update logging helpers to emit `run`, `root`, `job`, and `stage` fields instead of only root-local counters.
- [ ] Run: `cd /mnt/c/dev/Wox/wox.core && go test -tags sqlite_fts5 ./util/filesearch -run 'TestScanner(FullScanUsesGlobalRunProgress|FullScanReportsAllRunStages|FullScanSplitsLargeRootWithoutChangingRootIdentity)' -count=1`

### Task 6: Route DirtyQueue Through Incremental Run Planning

**Files:**
- Modify: `/mnt/c/dev/Wox/wox.core/util/filesearch/reconciler.go`
- Modify: `/mnt/c/dev/Wox/wox.core/util/filesearch/scanner.go`
- Test: `/mnt/c/dev/Wox/wox.core/util/filesearch/scanner_incremental_test.go`
- Test: `/mnt/c/dev/Wox/wox.core/util/filesearch/run_planner_test.go`

- [ ] Add failing tests for:
  - dirty signals discovered during execution being queued for the next run
  - incremental planner rebuilding affected scope frontiers instead of mutating the active plan
  - one failing incremental job failing the active run and preserving the queue
- [ ] Run: `cd /mnt/c/dev/Wox/wox.core && go test -tags sqlite_fts5 ./util/filesearch -run 'Test(ScannerQueuesDirtySignalsForNextRunDuringExecution|RunPlannerIncrementalScopesAreRebuiltFresh|ScannerIncrementalRunFailsFastAndKeepsQueue)' -count=1`
- [ ] Replace `DirtyQueue -> ReconcileBatch -> direct execution` with `DirtyQueue -> IncrementalPlanner -> sealed RunPlan -> JobExecutor`.
- [ ] Keep the current dirty-signal collection, batching, and debounce inputs. Only replace the execution target and queue semantics.
- [ ] Add comments in `scanner.go` and `reconciler.go` explaining that incremental plans are disposable execution metadata, so re-planning affected scopes is safer than trying to preserve old job boundaries.
- [ ] Run: `cd /mnt/c/dev/Wox/wox.core && go test -tags sqlite_fts5 ./util/filesearch -run 'Test(ScannerQueuesDirtySignalsForNextRunDuringExecution|RunPlannerIncrementalScopesAreRebuiltFresh|ScannerIncrementalRunFailsFastAndKeepsQueue)' -count=1`

### Task 7: Benchmarks, Smoke Validation, and Build Verification

**Files:**
- Modify: `/mnt/c/dev/Wox/wox.core/util/filesearch/filesearch_benchmark_test.go`
- Modify: `/mnt/c/dev/Wox/wox.core/util/filesearch/scanner_full_scan_test.go`
- Modify: `/mnt/c/dev/Wox/wox.core/util/filesearch/scanner_incremental_test.go`

- [ ] Add or update the large-root benchmark so it measures bounded job execution rather than only `ReplaceSubtreeSnapshot`.
- [ ] Run: `cd /mnt/c/dev/Wox/wox.core && go test -tags sqlite_fts5 ./util/filesearch -run 'TestScanner(FullScanUsesGlobalRunProgress|FullScanReportsAllRunStages|FullScanSplitsLargeRootWithoutChangingRootIdentity|QueuesDirtySignalsForNextRunDuringExecution|IncrementalRunFailsFastAndKeepsQueue)' -count=1`
- [ ] Run: `cd /mnt/c/dev/Wox/wox.core && go test -tags sqlite_fts5 ./util/filesearch -bench 'Benchmark(FileSearchRunPlanner|FileSearchJobExecutor)' -run '^$' -count=1`
- [ ] Run: `cd /mnt/c/dev/Wox/wox.core && make build`
- [ ] If benchmark or smoke output shows whole-root memory retention or regressing progress monotonicity, stop and fix before moving on.

---

## Self-Review

### Spec Coverage

- Global `RunPlan -> RootPlan -> Job` model: Tasks 1, 2, 3
- Huge-root recursive split policy: Task 2
- Global stable progress and stage ownership: Tasks 1, 3, 5
- Incremental runs queue behind active run: Task 6
- Conservative finalize/cursor behavior: Task 4
- Existing root identity preserved: Tasks 5 and 7
- Smoke and benchmark validation for progress/memory behavior: Task 7

No known spec gaps remain in this plan.

### Placeholder Scan

- No `TODO`, `TBD`, or “handle appropriately” placeholders remain.
- Every task names concrete files, behaviors, and verification commands.

### Type Consistency

- `RunPlan` is the sealed workload.
- `Run` is the execution attempt.
- `JobExecutor` consumes `jobs []Job` in `order_index` / slice order.
- `FinalizeRootRun` is the conservative root-level finalize hook referenced by Tasks 4–6.

Plan complete and saved to `/mnt/c/dev/Wox/docs/superpowers/plans/2026-04-22-filesearch-run-planner-implementation.md`.

Two execution options:

1. Subagent-Driven (recommended) - I dispatch a fresh subagent per task, review between tasks, fast iteration
2. Inline Execution - Execute tasks in this session using `superpowers:executing-plans`, batch execution with checkpoints

Which approach?
