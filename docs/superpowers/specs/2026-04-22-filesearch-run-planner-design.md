# File Search Run Planner Design

Date: 2026-04-22
Status: Draft approved in discussion, pending written spec review
Owner: Codex + qianlifeng

## Context

The current file-search indexing path in Wox mixes three concerns inside the same root-centric loop:

1. discover what needs to be indexed
2. build a full snapshot for one root or subtree
3. write that snapshot into SQLite and its derived search artifacts

That structure was acceptable while root sizes stayed moderate, but it breaks down once one logical root contains hundreds of thousands of files:

- the scanner holds large `entries` and `directories` slices in memory before writing
- SQLite write time becomes one large opaque block rather than many bounded units
- `StatusSnapshot` and toolbar progress are derived from root-local state, so progress appears to reset when execution moves from one root to the next
- long-running work can sit on one root while the user sees either no progress or a misleading local percentage

The recent SQLite-first search change removed most steady-state query-time cache pressure, but it did not solve the indexing-time memory spike or the progress instability because the execution model is still root-centric.

## User Constraints Collected During Brainstorming

1. Incremental runs must queue behind the current run rather than mutating an active run plan.
2. Once a `RunPlan` is finalized, it must not change.
3. Every expensive phase must expose progress, including planning and pre-scan.
4. Progress must be stable, monotonic, and explainable. A displayed `99%` must mean only a small known amount of work remains.
5. Huge logical roots should be internally split into smaller execution units without changing the user-facing root model.
6. Full indexing and incremental indexing should share the same planning and execution concepts.

## Goals

1. Replace root-centric execution with a global run model that owns progress and scheduling.
2. Bound memory and transaction size for huge roots by splitting them into smaller jobs before execution.
3. Make progress global, monotonic, and based on a frozen total workload.
4. Reuse the same planner and job model for both full indexing and incremental reconcile.
5. Preserve the existing user concept of configured roots.
6. Keep the execution pipeline readable and easy to debug.

## Non-Goals

1. Changing the user-facing root settings model.
2. Running multiple indexing runs concurrently.
3. Introducing speculative adaptive scheduling inside a running plan.
4. Solving every possible future prioritization policy in version 1.
5. Replacing SQLite or redesigning the existing durable root tables in this change.

## Problem Summary

Wox currently treats a root as both:

- a durable configuration object
- a scheduling unit
- a progress-reporting unit

Those three roles do not scale together.

`RootRecord` should continue to represent the configured filesystem root, but it is the wrong abstraction for execution and progress:

- one huge root may dominate memory and write time
- two roots of very different sizes make root-local percentages misleading
- one root can contain a few tiny subtrees and a few enormous subtrees, which means the root itself is not a good unit of work

The missing layer is a global planner that converts roots into a frozen set of bounded execution jobs before indexing begins.

## Approaches Considered

### 1. Keep root-centric execution and improve the UI only

Pros:

- minimal code movement
- no new planning concepts

Cons:

- does not solve memory spikes
- does not solve long opaque write phases
- progress would still be based on unstable or guessed totals

### 2. Split huge roots into internal jobs after a global planning and pre-scan phase

Pros:

- preserves user-visible root semantics
- creates bounded work units for memory and write latency
- allows a frozen global progress denominator
- works for both full and incremental indexing

Cons:

- adds explicit planning structures
- requires a migration path from root-centric status reporting

### 3. Turn child directories into new persisted roots

Pros:

- implementation can reuse some existing root-based code

Cons:

- pollutes the meaning of configured roots
- complicates settings, delete semantics, feeds, and status
- leaks internal scheduling details into persistent state

## Decision Summary

Wox should introduce a global indexing run model with four layers:

1. `RunPlan`
   - one sealed full or incremental workload
2. `Run`
   - one execution attempt against a sealed run plan
2. `RootPlan`
   - the frozen plan for one configured root
3. `Job`
   - a bounded execution unit produced from one root plan
4. `Executor`
   - a sequential consumer of frozen jobs

The planner will process all roots up front:

1. global `planning`
2. global `pre-scan`
3. seal `RunPlan`
4. execute jobs in order
5. finalize the run

Huge roots will not become more roots. They will be recursively split into bounded scopes until each leaf scope can be executed within memory and write-time budgets.

## Architecture Overview

The indexing pipeline becomes:

1. `RunPlanner`
   - gathers the active root set and creates a new run
2. `RootPlanner`
   - builds a `RootPlan` per configured root
3. `JobBuilder`
   - turns each root plan's leaf scopes into jobs
4. `JobExecutor`
   - executes jobs in a stable order
5. `RunProgressTracker`
   - owns the global denominator and exposes user-facing progress

The existing `RootRecord` remains the durable root configuration and root-level health record. It is no longer the primary source of global indexing progress.

## Core Types

### `RunPlan`

Represents one immutable indexing workload after planning finishes.

Fields:

- `plan_id`
- `run_id`
- `kind` (`full`, `incremental`)
- `root_plans []RootPlan`
- `jobs []Job`
- `total_work_units`
- `planning_totals`
- `pre_scan_totals`
- timestamps

Rules:

- incremental work discovered during execution is not appended to an existing sealed plan
- once sealed, `root_plans`, `jobs`, and `total_work_units` do not change

### `Run`

Represents one execution attempt against a sealed `RunPlan`.

Fields:

- `run_id`
- `plan_id`
- `status` (`planning`, `pre_scan`, `executing`, `finalizing`, `completed`, `failed`, `canceled`)
- `completed_work_units`
- `active_job_id`
- `queued_incremental_signals`
- timestamps and last error

Rules:

- only one run executes at a time
- a run executes exactly one sealed `RunPlan`
- incremental work discovered during execution queues for the next run

### `RootPlan`

Represents how one configured root will be executed in this run.

Fields:

- `root_id`
- `root_path`
- `strategy` (`single`, `segmented`)
- `scope_tree`
- `totals`
  - `directory_count`
  - `file_count`
  - `indexable_entry_count`
  - `skipped_count`
  - `planned_scan_units`
  - `planned_write_units`
- `jobs []JobRef`
- `split_policy_version`

`RootPlan` is not a snapshot payload. It contains exact counts and frozen execution structure only.

### `ScopeNode`

Represents a planned filesystem scope during pre-scan.

Fields:

- `scope_path`
- `scope_kind` (`direct_files`, `subtree`)
- `parent_scope_path`
- `children []ScopeNode`
- `directory_count`
- `file_count`
- `indexable_entry_count`
- `skipped_count`
- `planned_scan_units`
- `planned_write_units`
- `split_required`
- `sealed`

Leaf `ScopeNode`s become jobs.

### `Job`

Represents a bounded execution unit.

Version 1 should support three job kinds:

- `DirectFilesJob`
- `SubtreeJob`
- `FinalizeRootJob`

Shared fields:

- `job_id`
- `root_id`
- `root_path`
- `scope_path`
- `kind`
- `planned_scan_units`
- `planned_write_units`
- `planned_total_units`
- `status`
- `order_index`

### `RunProgressSnapshot`

Represents global progress and current execution context.

Fields:

- `run_status`
- `progress_current`
- `progress_total`
- `active_root_id`
- `active_root_path`
- `active_job_id`
- `active_job_kind`
- `active_scope_path`
- `active_stage`
- `active_stage_current`
- `active_stage_total`
- `queued_run_count`
- optional last error

This becomes the primary source for toolbar and status reporting.

## Planning And Pre-Scan

### Global Planning

The planner first enumerates the configured roots and creates one `RootPlan` shell per root.

For each root, the initial frontier is:

- one `direct_files(root)` scope
- one `subtree(child_dir)` scope for each first-level child directory

At this stage, Wox records only structure, not full snapshot entries.

The output of global planning is a stable root list and an initial scope frontier for every root.

### Global Pre-Scan

Pre-scan walks every planned scope and computes exact counts needed for progress and scheduling:

- directory count
- file count
- indexable entry count
- skipped count
- planned scan units
- planned write units

Pre-scan does not build `EntryRecord`, does not compute pinyin/bigram payloads, and does not write SQLite rows. It exists to freeze a truthful workload.

### Pre-Scan Cost Tradeoff

Pre-scan deliberately introduces a second filesystem pass:

- pass 1: exact planning and pre-scan
- pass 2: execution and snapshot construction

This means large roots will perform roughly double metadata I/O in version 1. The design accepts that cost because stable, truthful progress and bounded job sizing are the primary user-facing goals. In practice, local page cache should absorb part of the second pass, but the spec does not rely on that as a correctness property.

Version 1 keeps pre-scan lightweight and exact rather than trying to cache every future `EntryRecord` field. A future optimization may allow pre-scan to carry selected metadata into execution, but that is explicitly out of scope for this design.

### Why Pre-Scan Must Be Exact

The progress contract requires that displayed percentages are monotonic and trustworthy. That is only possible when the global denominator is frozen before execution begins.

For that reason, version 1 explicitly chooses a double-pass approach over speculative progress:

- pass 1: planning + exact pre-scan
- pass 2: execution

## Huge Root Split Policy

Huge roots are handled inside `RootPlan`, not by introducing more durable roots.

### Split Trigger

A scope is considered too large and must be split when any of the following is true:

- `indexable_entry_count > leaf_entry_budget`
- `planned_write_units > leaf_write_budget`
- `estimated direct snapshot memory > leaf_memory_budget`

The exact threshold values should start as code constants tuned from observed logs. They are not user-facing settings in version 1.

### Recursive Split Rule

When a `subtree(dir)` scope exceeds budget, it is replaced by:

- one `direct_files(dir)` scope
- one `subtree(child_dir)` scope for each immediate child directory

Then pre-scan repeats on the new children.

This continues until every leaf scope is within budget or further splitting provides no practical reduction.

### Direct Files Streaming

A directory may be wide rather than deep. Version 1 does not split one `direct_files(dir)` scope into multiple jobs.

Instead:

- one directory owns exactly one `DirectFilesJob`
- that job streams its direct files to SQLite in bounded staging batches
- stale direct-file pruning happens once for the whole directory scope owned by that single job

This keeps delete ownership unambiguous. The earlier chunked design reduced batch size, but it also split stale-file ownership across sibling jobs and made direct-file pruning harder to reason about.

### Root Strategy

After pre-scan:

- `single` means the root ended up with one execution scope plus finalization
- `segmented` means the root has multiple leaf scopes and therefore multiple jobs

The strategy is descriptive only. Execution still happens through the same job runner.

## Job Model And Execution Order

### Job Kinds

#### `DirectFilesJob`

Processes only direct files for one directory.

Use when:

- the directory itself should not imply recursive traversal
- the planner needs a bounded ownership scope for direct-file pruning

Execution may still stream that one job in smaller SQLite staging batches, but those batches are execution-local and do not become separate jobs.

#### `SubtreeJob`

Processes one subtree scope under a root.

Use when:

- a subtree is small enough to execute as one bounded unit
- the scope needs recursive traversal

#### `FinalizeRootJob`

Per-root closing job.

Responsibilities:

- update root-level persisted state
- refresh feed snapshot metadata if required
- run root-scoped final bookkeeping

### Execution Order

Version 1 should execute jobs sequentially in stable order:

1. root order from the planner
2. within one root, lexical scope order
3. `FinalizeRootJob` last for that root

This keeps progress predictable and debugging straightforward. Parallel execution can be considered later but is intentionally out of scope for this design.

## Full Run And Incremental Run

### Full Run

Planner input:

- all configured roots

Planner output:

- one `RootPlan` per root
- one frozen global job list

Executor behavior:

- run every job in order
- finish with run finalization

### Incremental Run

Planner input:

- queued dirty root and dirty path signals accumulated since the previous run

Planner output:

- one or more `RootPlan` values built only for affected scopes
- a frozen job list for that incremental work

Executor behavior:

- identical to full run execution

This preserves one mental model. The difference is only what the planner consumes, not how execution works.

### Incremental Planner Boundary

Incremental planning does not reuse the previous full run's `ScopeNode` tree as mutable state. A sealed `RunPlan` is disposable execution metadata, not a long-lived scheduling graph.

For an incremental run:

1. queued dirty signals are coalesced by root and affected scope
2. the incremental planner builds a fresh root-local scope frontier only for affected areas
3. that frontier is pre-scanned with the same split policy used by full runs
4. the resulting incremental `RootPlan` values are sealed into a new `RunPlan`

This means the existing dirty-signal collection remains useful, but the current `DirtyQueue -> ReconcileBatch -> direct execution` flow is replaced by `DirtyQueue -> IncrementalPlanner -> RunPlan -> JobExecutor`.

If one dirty path sits under a subtree that was previously split into multiple jobs, the incremental planner does not try to recover old job boundaries. It rebuilds only the affected root-local scope frontier and applies the same budget rules again. Because job boundaries are execution-local and not persisted identities, this recomputation is safe.

## Progress Model

### Ownership

Global progress belongs to `Run` and its sealed `RunPlan`, not to `RootRecord`.

`RootRecord.Status` can continue to exist for compatibility and persistence, but toolbar percentage and active stage should come from the active run snapshot.

### Stages

The run exposes four top-level stages:

1. `planning`
2. `pre_scan`
3. `executing`
4. `finalizing`

Each stage has a stable denominator:

- `planning`: roots processed / total roots
- `pre_scan`: roots pre-scanned / total roots
- `executing`: completed job work units / total job work units
- `finalizing`: completed finalize steps / total finalize steps

Version 1 deliberately keeps `pre_scan` on a root-level denominator. Recursive split discovery can grow the scope frontier while pre-scan is still running, so a scope-count percentage would move backwards. The current root path and active scope path still surface which subtree is being measured.

### Stable Percentage Rule

Global percentage is:

`run.completed_work_units / run_plan.total_work_units`

The plan's denominator is frozen when pre-scan completes. It must never increase during execution.

This guarantees:

- no backward movement when switching roots
- no root-local `100%` followed by another root's `5%`
- `99%` always means a small, already known amount of work remains

### Current Activity Text

The UI should show:

- global percentage from the run
- current root path
- current job or planner scope
- current stage

Example:

`Overall 41% · C:\Windows · subtree(System32\drivers) · write`

This keeps the stable percentage global while still telling the user what is happening right now.

## Integration With Existing Types

The existing code already stores root-level status in `roots` and aggregates it in [engine.go](/mnt/c/dev/Wox/wox.core/util/filesearch/engine.go). Version 1 should integrate with that runtime model without preserving root-centric progress semantics.

Implementation path:

1. add run-level in-memory state and status events
2. keep `RootRecord` persistence for configuration and recovery
3. teach `Engine.GetStatus` to prefer active run progress when a run exists
4. keep root-level counts as secondary detail, not the source of the main percentage

The existing `StatusSnapshot` should be extended rather than replaced in version 1 so plugin integration can migrate without a second compatibility layer. Its semantics must change:

- the top-level percentage is global
- active root/job/stage are descriptive
- root counts remain aggregate diagnostics

## Scanner And Database Responsibilities

### Planner Responsibilities

- enumerate roots
- build initial scope frontier
- pre-scan scopes
- apply recursive split policy
- seal `RunPlan`

Once `JobBuilder` has emitted the sealed job list, the planner must release the mutable traversal buffers that are no longer needed. Version 1 may still keep a sealed `ScopeNode` tree inside `RootPlan` for diagnostics and status inspection, but it must not retain the planner's mutable working buffers or duplicate scope trees through execution.

### Executor Responsibilities

- execute one job at a time
- build only the snapshot data needed for the active job
- write that job into SQLite
- update run progress using actual completed work units

### Database Layer Responsibilities

The database layer remains job-oriented, not run-oriented.

It should accept bounded job payloads and report write progress for each job. The global run tracker is responsible for combining job progress into one monotonic percentage.

### Changefeed Cursor And Finalization

Version 1 uses a conservative cursor rule:

- job transactions persist entries and derived artifacts for that job
- durable root-level feed cursor advancement happens only in `FinalizeRootJob`
- `FinalizeRootJob` advances the cursor to the run's captured fence only after every prior job for that root has committed successfully

This means a crash before `FinalizeRootJob` may leave entries newer than the persisted root cursor. That is acceptable because the next run may replay some already-applied signals, but it will not skip unseen filesystem changes. The write path therefore relies on idempotent job application, not on partial cursor advancement.

## Error Handling And Recovery

1. If planning fails before sealing the run, the run fails with no partial denominator exposed to the UI.
2. If one job fails during execution:
   - the active run fails
   - the error is attached to the run and the owning root
   - queued dirty work remains available for the next run
3. Incremental signals arriving during execution are queued for the next run rather than mutating the current run plan.
4. Cancellation leaves the current run incomplete and the queue intact.

Version 1 intentionally chooses fail-fast run semantics over root-isolated continuation. This is stricter than the current root-by-root behavior, but it keeps the first run-based implementation readable and makes progress semantics easier to reason about. The user-visible consequence is that one hard job failure stops the active run and requires a later retry. Root-isolated continuation and job-level retry are valid future enhancements once the run model is proven.

## Testing And Verification Expectations

Implementation should verify the following behaviors:

1. giant roots are split into multiple jobs without changing persisted root identity
2. `RunPlan` is immutable after sealing
3. incremental dirty signals discovered during execution queue for the next run
4. global progress never decreases
5. root switches do not reset global percentage
6. `99%` only occurs when the remaining frozen work is small
7. wide directories keep one direct-files job while streaming bounded SQLite staging batches
8. bounded job execution reduces peak in-memory snapshot size compared with whole-root execution
9. a crash before `FinalizeRootJob` may replay changefeed input but must not skip it

Smoke coverage should focus on run planning, large-root split behavior, and progress stability because those are the core user-visible guarantees of this design.

## Implementation Notes

Version 1 should prefer the simplest readable control flow:

- one active run
- one planner
- one sequential executor
- one global progress tracker

`JobBuilder` is responsible for producing jobs in final execution order and assigning `order_index`. `JobExecutor` must consume jobs in slice order and must not apply a second sorting layer.

Direct-file staging batches are execution-local only. They are not persisted identities and are not used as diff keys across runs. If file additions or removals change the number of streamed staging batches between two runs, the next incremental or full planner still computes the same single `DirectFilesJob` ownership scope for that directory.

This design intentionally avoids parallel job execution, dynamic replanning, and user-configurable split thresholds because those would make progress semantics and debugging harder before the basic run model is proven.
