# File Search Native ChangeFeed Design

Date: 2026-04-12
Status: Draft approved in discussion, pending written spec review
Owner: Codex + qianlifeng

## Context

The current file search indexer is built around `fsnotify` plus periodic full rescans. That model has three structural problems:

1. On macOS, `fsnotify` uses `kqueue`, which does not recursively cover deep directory trees and becomes resource-heavy when many directories are watched.
2. The current system mixes "discovering changes" with "rebuilding the index", so the product often falls back to expensive rescans and confusing progress states.
3. The launcher experience is not predictable. Indexing can appear stuck, and the system keeps scanning more often than necessary.

The existing provider model is also important:

- macOS already has `SpotlightProvider`, backed by `MDQuery`.
- Windows already has `EverythingProvider`, backed by the Everything SDK.
- `LocalIndexProvider` runs alongside those providers and is the only provider backed by Wox's own SQLite snapshot.

These system providers are part of the current transition state, not necessarily the final architecture. The product direction is to build a Wox-owned index service that can eventually become the primary file search backend if its quality and freshness are good enough.

User constraints collected during brainstorming:

- The design must not rely on watching large numbers of directories on macOS.
- A convergence window of up to 30 seconds is acceptable for daily incremental updates.
- Slightly stale results during that window are acceptable.
- The long-term direction should maximize platform-native filesystem capabilities instead of staying tied to `fsnotify`.

## Goals

1. Make file indexing efficient for large directory trees, especially on macOS.
2. Separate change detection from index reconciliation so daily updates do not look like full rescans.
3. Keep indexing behavior predictable:
   - first build and manual rebuild can show detailed progress,
   - daily incremental sync should converge within 30 seconds in the common case.
4. Use platform-native change feeds where they provide a clear advantage:
   - macOS: `FSEvents`
   - Windows: `USN Journal`
5. Preserve a working fallback path for platforms or volumes where native feeds are unavailable.
6. Build Wox's own index service as the long-term primary path, with current system providers treated as transitional supplements until parity is proven.

## Non-Goals

1. Exact real-time consistency after every filesystem mutation.
2. A single event model that preserves every platform-specific detail.
3. Rewriting the entire search ranking pipeline.
4. Solving Linux filesystem monitoring beyond a reasonable fallback for now.

## Decision Summary

Wox should stop treating `fsnotify` as the primary indexing signal source. Instead, file search should move to a `ChangeFeed` abstraction for maintaining a Wox-owned index service backed by the local snapshot:

- `darwin`: `FSEvents`
- `windows`: `USN Journal`
- `fallback`: root-level `fsnotify` or polling-based dirty marking where native feeds are not available

This change feed layer will only answer one question: "what part of the local indexed tree is now dirty?" It will not update the index directly.

The indexer will reconcile dirty roots or dirty subtrees against an on-disk snapshot model. Full two-phase scans remain only for first build and explicit rebuilds.

`SpotlightProvider` and `EverythingProvider` remain transitional search providers during rollout. The intended end state is that Wox's own index service is good enough to become the default primary file search path, after which those platform providers can be reduced to optional fallback or removed.

## Architecture

The system is split into four layers:

### 1. ChangeFeed

Platform-specific source of change notifications.

Responsibilities:

- start and stop monitoring configured roots
- persist and resume feed cursors where supported
- emit normalized dirty signals
- emit recovery signals when the feed cannot guarantee incremental correctness

Normalized output:

- `DirtyRoot(rootID)`
- `DirtyPath(rootID, path)`
- `RequiresRootReconcile(rootID, reason)`
- `FeedUnavailable(rootID, reason)`

The abstraction is intentionally lossy. Upper layers only need to know whether a subtree can be reconciled incrementally or whether a wider rebuild is required.

### 2. Dirty Queue

Buffered, coalescing queue of pending reconcile work.

Responsibilities:

- deduplicate repeated events
- collapse nested dirty paths only when doing so does not expand work disproportionately
- debounce bursts
- schedule reconcile work under a fixed latency budget

Behavior:

- debounce window: small, on the order of 1 to 3 seconds
- convergence target: 30 seconds or less for normal daily activity
- coalescing is subtree-local, not global:
  - `/a/b/c/file.txt` and `/a/b/d/file.txt` may collapse to `/a/b`
  - `/a/b/c/file.txt` and `/a/d/e/file.txt` must remain separate dirty subtrees
- escalation to a full root reconcile happens only when batch size or estimated work crosses a root-level threshold

The queue must build batches per root, then merge within each subtree using explicit thresholds rather than always lifting to the nearest common ancestor.

Initial threshold guidance:

- sibling merge threshold:
  - if a parent accumulates 8 or more dirty direct children in the same batch, it may collapse to that parent directory
- root escalation threshold:
  - if dirty subtrees for one root exceed 10% of tracked directories for that root, or exceed 512 distinct dirty paths in one batch, escalate to full root reconcile

These numbers are initial tuning values and must be validated by benchmark data before broad rollout.

### 3. Reconciler

Compares dirty roots/subtrees against the stored snapshot and updates index rows.

Responsibilities:

- scan only the affected subtree when the dirty signal is narrow enough
- fall back to root-level reconcile when the signal is broad or reliability is degraded
- update index entries and snapshot metadata transactionally
- expose progress only for explicit full builds

### 4. Search Snapshot Store

Persistent representation of the indexed tree used by reconcile logic.

This extends the existing file search database so incremental updates do not require full root replacement.

## Data Model

The snapshot store should evolve from "entries only" to "entries plus directory state".

Required persisted data:

### `roots`

Existing root metadata, extended with:

- `feed_type`
- `feed_cursor`
- `feed_state`
- `last_reconcile_at`
- `last_full_scan_at`

### `entries`

Existing indexed files and directories, kept as the query source.

The reconciler will stop replacing the entire root on every update. It will update affected path ranges instead.

### `directories`

New directory snapshot table.

Minimum fields:

- `root_id`
- `path`
- `parent_path`
- `last_scan_time`
- `exists`

Optional optimization fields that can be added in later phases:

- `child_file_count`
- `child_dir_count`
- lightweight content fingerprint

This table is the boundary that makes subtree reconcile practical. It lets the reconciler know which directory snapshots already exist and which subtree must be refreshed or deleted.

This table is part of Wox's own index service. It does not directly affect `SpotlightProvider` or `EverythingProvider`, except that those providers may become less important as the Wox index matures.

Lifecycle and cleanup rules:

1. when a root is removed, its `directories` rows and `entries` rows must be deleted together
2. when a directory disappears, the reconciler may mark its snapshot row as `exists=false` immediately
3. tombstoned directory rows should be cleaned during later maintenance, with full build as the minimum required cleanup point
4. the design must avoid an unbounded "only grows" `directories` table

## Provider Integration

The current engine queries `LocalIndexProvider` together with platform system providers. That is the starting point, not the final target.

Provider responsibilities after this design:

- `Wox index service` (`LocalIndexProvider` evolved): Wox-owned snapshot and the intended long-term primary file search path for configured roots.
- `SpotlightProvider`: transitional macOS supplemental search source during rollout.
- `EverythingProvider`: transitional Windows supplemental search source during rollout.

This means:

1. `ChangeFeed`, dirty queue, reconcile, and snapshot storage exist to build Wox's own index service into the main path.
2. `SpotlightProvider` and `EverythingProvider` stay during rollout to protect search quality and latency while the Wox index service matures.
3. Once Wox's index quality, coverage, and freshness meet product requirements, system providers may be demoted or removed in a later follow-up.

### Target End State

The desired steady state is:

1. Wox owns indexing, freshness, and recovery logic.
2. Platform-native OS APIs are used for change detection where available.
3. Search results primarily come from Wox's own index service.
4. `SpotlightProvider` and `EverythingProvider` are optional compatibility layers, not architectural dependencies.

### Parity Criteria

System providers should not be demoted from their default query role until the Wox-owned index service meets explicit rollout criteria on representative workloads.

Initial parity criteria:

1. incremental convergence latency:
   - p95 dirty-root convergence time under 30 seconds
2. result coverage:
   - on benchmark query sets, top result set overlap with the current system-backed path is at least 95%
3. query responsiveness:
   - p95 query latency is no worse than 1.5x the current mixed-provider baseline
4. resource cost:
   - steady-state CPU and memory cost is no worse than 1.5x the current mixed-provider baseline
5. recovery behavior:
   - feed loss or cursor invalidation must recover to a correct full rebuild without permanent staleness

These are rollout gates, not architecture guarantees. They may be tightened after real benchmark data exists.

Measurement baseline rules:

1. Phase 1a must capture a baseline snapshot of the current mixed-provider path before any rollout comparison is considered valid.
2. That baseline snapshot must include:
   - p95 query latency
   - steady-state CPU usage
   - steady-state memory usage
   - representative result sets for benchmark queries
3. Benchmark query sets should come from:
   - a curated fixed query suite that covers common path and filename patterns
   - real usage samples when available and privacy-safe to collect
4. Later parity comparisons must be measured against that frozen baseline snapshot, not against an implicitly changing runtime.

## Platform Design

### macOS: `FSEvents`

Use a single `FSEventStream` per root set, not per subdirectory, as the primary macOS change feed for the Wox-owned index service.

Why:

- Apple positions `FSEvents` as the efficient API for monitoring directory hierarchies.
- It avoids the kqueue scaling problem caused by large watch counts.
- It is the correct platform-native foundation if Wox is going to own indexing on macOS rather than depend on Spotlight indefinitely.

Behavior:

- monitor configured roots
- persist the last processed event ID
- on restart, resume from the stored event ID when valid
- when an event arrives, mark the event path dirty
- when flags indicate coalescing or history loss, escalate to `RequiresRootReconcile`

Important constraint:

`FSEvents` is still advisory. It tells us where changes occurred, but correctness comes from the reconcile pass, not from trusting raw event payloads.

Rollout note:

`SpotlightProvider` still helps during transition, but it is no longer the architectural target. `FSEvents` is the intended long-term macOS change feed for Wox's own index service.

### Windows: `USN Journal`

Use the volume change journal as the intended long-term Windows incremental feed for Wox's own index service on supported volumes.

Why:

- it is designed for efficient, persistent filesystem change tracking
- it survives process restarts better than transient directory notification streams
- it is a better fit for index maintenance than recursive watcher trees
- it is the most direct native foundation if Wox wants to own indexing on Windows instead of depending on Everything long term

Behavior:

- map configured roots to their backing volume
- persist `journal_id` and last processed `USN`
- read journal deltas and convert them into dirty paths under indexed roots
- if the journal is reset, truncated past the saved cursor, or unavailable for the volume, escalate to root reconcile or feed fallback

Important constraint:

USN Journal support is volume-dependent. The architecture must allow per-root fallback when a root is on an unsupported or inaccessible volume.

Rollout note:

`EverythingProvider` still protects Windows search quality during transition, but it is not the desired final dependency if Wox can provide comparable quality with its own index service.

### Fallback: `fsnotify` or Polling

Fallback is not the primary strategy. It exists so non-macOS, non-Windows-supported cases still work.

Rules:

- never recursively watch large directory trees
- at most watch roots, or use periodic root dirty marking
- fallback signals should bias toward `DirtyRoot` rather than pretending to support precise deep incremental sync

Fallback is acceptable during early rollout, but it is not the intended high-quality end state on macOS or Windows.

## Reconcile Strategy

There are two distinct indexing modes.

### Mode A: Full Build

Used for:

- first index build
- manual rebuild
- recovery from invalid snapshot state

Flow:

1. Pre-scan directories to estimate total work.
2. Run the full scan with detailed progress.
3. Replace or rebuild snapshot state for the root.
4. Update feed cursors after the full build completes.

This is the only mode that should show detailed scan percentages.

### Mode B: Incremental Reconcile

Used for daily updates triggered by the change feed.

Flow:

1. Consume dirty items from the queue.
2. Coalesce overlapping paths.
3. For each dirty item:
   - if it is a file path, reconcile the parent directory subtree
   - if it is a directory path, reconcile that subtree
   - if reliability is degraded, reconcile the full root
4. Apply DB updates only to affected paths.
5. Process the whole batch inside a bounded transaction per root batch, not one transaction per path.
6. In early phases, allow a full in-memory reload after the committed batch if that keeps scope manageable.
7. Move to true in-memory incremental mutation only after the provider data structure is redesigned to support it efficiently.

This mode should present itself as "syncing file changes", not "scanning files".

Transaction rule:

- one dirty batch per root should usually map to one SQLite transaction
- very large batches may be chunked deliberately, but not down to one transaction per dirty path
- root-level reconcile should chunk writes by a bounded unit, for example around 1000 entry mutations per commit, when needed to cap WAL growth and reduce read interference
- the reconciler should optimize for amortized WAL throughput, not minimal transaction size

## Progress and UX Model

### Full Build UX

Phases:

1. `Preparing index`
2. `Pre-scanning folders`
3. `Scanning files x/y`
4. `Writing index z%`
5. `Finalizing index`

This progress can be detailed because work is explicit and user-initiated or first-run.

### Incremental Sync UX

Phases:

1. `Syncing file changes`
2. optional detail: `N roots pending` or `N paths pending`

Rules:

- no fake 99% state
- no pretending that incremental sync is a full scan
- query results may remain available from the last committed snapshot during sync

## Failure and Recovery

The design must assume feeds are imperfect.

Recovery triggers:

- feed cursor invalid
- history lost or overflow signal
- root moved or deleted
- database snapshot inconsistency
- unsupported volume or permissions failure

Recovery behavior:

- mark the root degraded
- stop trusting incremental updates for that root
- schedule a full root rebuild
- once rebuild succeeds, resume incremental tracking from a fresh cursor

This keeps the system correct without requiring every event stream to be perfect.

Conservative resume policy:

- persisted cursors should have an age check
- if the saved cursor is older than the configured safe window, the root should go straight to full reconcile instead of attempting a long incremental catch-up
- the default safe window can start conservative, for example 24 hours, and be tuned with real-world data

## Migration Plan

The target architecture is larger than a safe single patch. Implementation should be phased.

### Phase 1a

- define provider boundaries explicitly: system providers are transitional supplements while Wox's own index service is being strengthened
- change the DB model so local snapshot updates can happen per subtree batch instead of `ReplaceRootEntries`
- add the `directories` snapshot table and migration path
- benchmark directory snapshot scale and subtree-update cost before changing runtime behavior
- capture the current mixed-provider baseline metrics and benchmark query/result snapshot for later parity comparison
- add unit and integration coverage for the new snapshot schema and subtree-update paths
- keep search behavior functionally unchanged apart from any DB preparation needed for benchmarking

### Phase 1b

- introduce dirty queue and reconcile flow independent of direct full rescan requests
- keep current fallback path working
- change UX so incremental sync is distinct from full build
- keep in-memory provider refresh simple at first, even if that means full reload after a committed batch
- add unit tests for dirty queue merging and escalation rules, plus integration tests for batched reconcile behavior

### Phase 2

- add macOS `FSEvents` adapter for the Wox index service
- persist event IDs and root feed state
- switch macOS indexed roots from root-only `fsnotify` to `FSEvents`

### Phase 3

- add Windows `USN Journal` adapter
- persist journal cursors
- enable per-volume fallback handling
- keep `EverythingProvider` available during rollout until the Wox-owned path demonstrates parity on target scenarios

### Phase 4

- optimize subtree DB updates and in-memory refreshes
- add optional directory metadata optimizations if needed
- re-evaluate whether `SpotlightProvider` and `EverythingProvider` are still needed as default query participants

## Scope Check

The full target architecture is intentionally phased. A single implementation plan should not try to deliver every platform adapter at once.

Recommended first implementation plan scope:

- Phase 1a only:
  - DB changes needed to support batched subtree updates
  - `directories` table introduction
  - benchmark and measurement harness for realistic root sizes

Optional first-plan extension only if scope remains acceptable:

- none

Recommended second implementation plan scope:

- Phase 1b:
  - dirty queue and reconcile flow
  - fallback behavior for unsupported platforms and roots
  - UX split between full build and incremental sync
  - conservative in-memory reload after committed batches

Windows `USN Journal` support remains a later phase because of complexity, but it is part of the target end state rather than an optional side quest.

True in-memory incremental mutation and provider data-structure redesign are explicitly out of scope for the first and second implementation plans.

## Risks

1. `FSEvents` and `USN Journal` have different semantics, so the abstraction must stay minimal.
2. Incremental subtree DB updates will add complexity compared with current root-wide replace logic.
3. Feed cursor bugs can cause silent staleness if recovery rules are not strict.
4. Windows support will require careful handling of volume capabilities and path-to-volume mapping.
5. The `directories` table may become large on some roots, so rollout should be gated by benchmark data rather than assumption.
6. The macOS adapter will need CGo plus run loop coordination, which is more complex than the current `MDQuery` bridge.
7. The Windows adapter will need careful handling of journal record formats, cursor invalidation, and path reconstruction, making it a high-complexity phase.

## Explicit Decisions

To avoid ambiguity, the following decisions are fixed by this spec:

1. This architecture is intended to grow Wox's own index service into the primary file search backend over time.
2. Wox will not rely on recursive directory watching as the primary strategy on macOS.
3. `fsnotify` will become a fallback transport, not the main long-term indexing architecture.
4. First build and manual rebuild use full two-phase indexing.
5. Daily updates use dirty queue plus incremental reconcile, with a 30-second convergence target.
6. Dirty path merging must remain subtree-local unless an explicit root-level escalation threshold is crossed.
7. Incremental DB work must be batch-transactional, not one transaction per dirty path.
8. Platform-native feeds are allowed to be advisory; correctness is enforced by reconcile and rebuild rules.
9. `SpotlightProvider` and `EverythingProvider` are rollout aids, not required permanent dependencies.

## Exclude Handling

Exclude rules should be applied in two places:

1. Dirty queue filtering for cheap early rejection when the changed path is obviously excluded by simple prefix-based rules.
2. Reconciler filtering as the correctness boundary before any snapshot rows are written.

Boundary rules:

- dirty queue only performs cheap path-prefix or exact-directory rejection
- complex glob or `.gitignore`-style matching remains inside the reconciler

This keeps the feed layer simple while still preventing obviously excluded paths from generating unnecessary subtree work in the common case.

## Testing Strategy

Regression prevention is part of the architecture, not optional cleanup after implementation. Each implementation phase must ship with tests appropriate to the layer being changed.

### Unit Tests

Unit tests should cover deterministic logic that does not need the real OS feed or launcher runtime:

1. dirty queue subtree merge behavior
2. dirty queue root-escalation thresholds
3. cursor age decision logic
4. exclude early-rejection logic
5. snapshot diff and reconcile planning helpers

### Integration Tests

Integration tests should cover stateful behavior around SQLite and reconcile execution:

1. subtree update writes `entries` and `directories` correctly
2. root removal cascades snapshot cleanup correctly
3. tombstoned directories are cleaned on full rebuild
4. batched reconcile keeps queryable state valid during and after commit
5. large root-level reconcile chunking preserves correctness

### Adapter Contract Tests

Native feed adapters should be tested through a common contract using fake event inputs where possible:

1. dirty path normalization
2. recovery escalation on feed loss or invalid cursor
3. cursor persistence and resume decisions

Direct end-to-end tests against real `FSEvents` or `USN Journal` should be minimal and targeted, because they are platform-specific and harder to keep stable in CI.

### Smoke Tests

Because this is a major indexing behavior change, launcher-level smoke coverage is required:

1. first build shows full-build progress phases
2. incremental sync does not regress query responsiveness
3. fallback path still returns results when native feed support is unavailable

### Phase Gates

No phase should be considered complete without its corresponding tests:

1. Phase 1a requires schema and subtree-update coverage
2. Phase 1b requires dirty queue and batched reconcile coverage
3. later native-feed phases require adapter contract tests and at least one targeted platform smoke path

## Rollout Gates

Before enabling the new local incremental architecture broadly, the implementation must demonstrate:

1. acceptable reconcile latency for typical dirty batches
2. acceptable SQLite behavior with the `directories` table at realistic scale, including six-figure row counts
3. no search query blocking while reconcile writes are running
4. clear UX distinction between full build and incremental sync
5. parity criteria are met before system providers are demoted from their default role
