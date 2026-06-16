# File Search Query Index Design

Date: 2026-04-14
Status: Draft approved in discussion, pending written spec review
Owner: Codex + qianlifeng

## Context

The current Wox-owned file search path maintains a local SQLite snapshot and an in-memory `LocalIndexProvider`, but query execution still behaves like a filtered full scan:

1. `Engine` currently enables only `local-index` for file search queries.
2. `LocalIndexProvider.Search` copies the current entry slice and evaluates every entry against the query.
3. Matching and scoring happen during the scan, then matched results are sorted at the end.
4. Native change feeds such as `USN` and `FSEvents` help keep the snapshot fresh, but they do not make query execution fast by themselves.

This architecture is acceptable for small data sets, but it does not scale well once the local snapshot grows into hundreds of thousands or millions of entries. The recent wildcard filter addition made that limitation easier to see because query latency is already close to the current timeout threshold.

The current change feed and snapshot work remains valid. The missing piece is a query-optimized in-memory structure that can narrow the candidate set before fuzzy scoring.

## User Constraints Collected During Brainstorming

1. The design should be a cross-platform query-index abstraction, not a Windows-only optimization.
2. The initial design should cover all current query families as first-class indexed queries:
   - filename search
   - wildcard and extension filtering
   - path fragment search
   - pinyin full and initials search
3. The initial version should prioritize search speed over strict memory budgeting.
4. Index freshness should still follow the Wox-owned snapshot pipeline rather than platform-specific query engines.

## Goals

1. Replace full entry scans with a `recall -> rerank` search model.
2. Make Wox-owned file search fast enough to remain viable as a cross-platform primary query path.
3. Preserve current search semantics where practical by keeping final fuzzy scoring in a dedicated rerank stage.
4. Reuse the existing snapshot and change-feed pipeline instead of introducing a second filesystem monitoring stack.
5. Support incremental query-index updates from stable entry-level diffs, with a safe fallback to root rebuilds.
6. Keep the query-index architecture independent from platform-specific change detection APIs.

## Non-Goals

1. Matching or exceeding Everything's internal implementation details.
2. Defining a strict memory ceiling for version 1.
3. Replacing the SQLite snapshot as the source of truth.
4. Solving every future query language feature in this iteration.
5. Reintroducing platform search providers as part of this design.

## Problem Summary

The current local search path uses a data representation optimized for storage and refresh, not for retrieval:

- the SQLite `entries` table is the durable source of truth
- the in-memory `entries []EntryRecord` snapshot mirrors that durable state
- query execution still scans the full in-memory list

That design conflates two different jobs:

1. keeping a correct snapshot of the indexed filesystem
2. answering search queries quickly

The query-index layer introduced by this design solves only the second problem. The existing snapshot pipeline remains the authoritative state and the basis for recovery.

## Approaches Considered

### 1. Multi-branch specialized indexes

Maintain separate narrow structures for extension, prefix, path, and pinyin, then route queries through many special-case branches.

Pros:

- easy to understand
- extension filters become very fast

Cons:

- query routing logic grows quickly
- mixed queries are awkward
- long-term maintenance cost increases as more cases are added

### 2. Layered inverted query index

Maintain a single query-index subsystem composed of field-specific inverted structures and a common query planning flow.

Pros:

- consistent architecture across query types
- good fit for cross-platform use
- supports mixed-query recall naturally
- keeps future optimizations local to one subsystem

Cons:

- more initial implementation work
- memory usage must be observed carefully

### 3. Trie-heavy design with side indexes

Use tries for filename and pinyin prefixes plus side maps for extension and path fragments.

Pros:

- strong prefix performance

Cons:

- weak fit for fuzzy and contains-style queries
- requires many auxiliary paths anyway
- harder to evolve into a unified query planner

## Decision Summary

Wox should add a cross-platform `QueryIndex` layer for the Wox-owned local snapshot and move local file search to a two-stage execution model:

1. `Recall`
   - use query-optimized indexes to retrieve a smaller candidate set
2. `Rerank`
   - run current fuzzy and pinyin-aware scoring only on recalled candidates

The selected architecture is a layered inverted query index with root-level sharding and entry-diff patching:

- source of truth stays in SQLite `entries`
- `LocalIndexProvider` owns a query-index snapshot in memory
- query-index is partitioned by `rootID`
- small changes update the query-index by entry diff
- large changes rebuild the affected root shard and atomically swap it in

## Architecture

The query path is split into five layers.

### 1. Snapshot Source

Existing `entries` data in SQLite remains the durable source of truth.

Responsibilities:

- persist indexed files and directories
- survive restarts
- provide rebuild input
- provide recovery input when query-index state is missing or invalid

### 2. Entry Delta Producer

The scanner and reconcile pipeline should produce stable entry-level changes after snapshot writes succeed.

Responsibilities:

- translate dirty filesystem work into entry-level additions, updates, and removals
- hide platform feed details such as `USN` and `FSEvents`
- signal when a change batch is too large for efficient patching

### 3. QueryIndex

An in-memory, query-optimized view derived from the snapshot.

Responsibilities:

- maintain root-sharded document tables
- maintain field-specific inverted indexes
- support fast candidate recall
- support atomic shard replacement

### 4. Query Planner

Parses a raw query into explicit clauses and chooses an efficient recall strategy.

Responsibilities:

- classify wildcard, extension, path-like, filename, and pinyin-oriented queries
- order clause evaluation by expected selectivity
- control candidate growth before reranking

### 5. Reranker

Runs final scoring on recalled candidates only.

Responsibilities:

- preserve existing fuzzy ranking behavior where practical
- combine field-level matches into a single final score
- return stable ordering

## Core Types

The following conceptual types should be introduced. Exact file layout can be adjusted during implementation.

### `QueryIndex`

Top-level read-only snapshot used by search requests.

Fields:

- `version`
- `shards map[rootID]*RootShard`
- shared configuration for tokenization and thresholds

### `RootShard`

Per-root query-index partition.

`DocID` is shard-local. Version 1 does not require globally unique document IDs across every root.

Fields:

- `rootID`
- `version`
- `docCount`
- `docTable map[DocID]DocRecord`
- `pathToDocID map[normalizedPath]DocID`
- `freedDocIDs []DocID`
- field-specific inverted indexes

### `DocRecord`

Compact search payload referenced by `DocID`.

Minimum fields:

- `DocID`
- `Path`
- `Name`
- `ParentPath`
- `IsDir`
- `PinyinFull`
- `PinyinInitials`
- `NormalizedName`
- `NormalizedPath`
- optional lightweight cached metadata for ranking

### `EntryDeltaBatch`

Stable post-reconcile update batch.

Fields:

- `RootID`
- `Added []EntryRecord`
- `Updated []EntryUpdate`
- `Removed []EntryRecord`
- `Reason`
- `EstimatedCost`
- `ForceRebuild`

### `EntryUpdate`

Fields:

- `Old EntryRecord`
- `New EntryRecord`

### `QueryPlan`

Parsed execution description for one raw search.

Fields:

- `Raw`
- `Clauses []QueryClause`
- `RecallStrategy`
- `PerClauseLimit`
- `PostIntersectionLimit`
- `PreRerankLimit`

## Index Layout

Version 1 should index all query families discussed during brainstorming.

### 1. Extension Index

Structure:

- `lowercaseExt -> []DocID`

Purpose:

- direct recall for `*.png`
- strong narrowing for mixed wildcard queries such as `foo*.md`

Version 1 should treat extension lookup as exact, case-insensitive key lookup. It is not gram-based.

### 2. Name Gram Index

Structure:

- `trigram -> []DocID` for filename-derived trigrams
- bounded `bigram -> []DocID` fallback for two-character filename queries

Purpose:

- recall for normal filename search
- recall for fuzzy-like text queries before rerank

Version 1 should use trigram postings as the default filename gram strategy. Two-character filename queries may use the bounded bigram side index. One-character queries should bypass gram recall entirely and use the short-query fallback path described below.

### 3. Path Segment Index

Structure:

- `directorySegment -> []DocID`

Purpose:

- fast recall for directory and path fragment queries such as `src/plugin`
- lower memory cost than indexing full path substrings alone

The segment index should include directory names only. It should not index the filename itself. Filename recall belongs to the name index so the responsibilities stay distinct.

### 4. Path Gram Index

Structure:

- `trigram -> []DocID` for normalized directory-path trigrams

Purpose:

- support path contains-style recall when segment-only matching is insufficient

Version 1 should not maintain a path bigram index. Two-character path queries should rely on the path segment index or degrade to filter-only behavior when recall would be too broad.

### 5. Pinyin Full Gram Index

Structure:

- `trigram -> []DocID`
- bounded `bigram -> []DocID` fallback for two-character pinyin-full queries

Purpose:

- direct recall for full pinyin queries

### 6. Pinyin Initial Index

Structure:

- `initialsTrie`
- each terminal node stores the posting list for the complete initials string

Purpose:

- direct recall for initial-style queries such as `zsbg`

Version 1 should not materialize every initials prefix as its own posting list. The initials index should store complete initials strings and support prefix traversal through a dedicated trie-like structure.

### Posting Representation

Version 1 should use sorted `[]DocID` posting lists for simplicity and predictable iteration behavior. This keeps implementation cost lower and is sufficient while search speed is the primary goal.

This design intentionally does not require a bitset-first implementation in version 1.

## Query Parsing

Each raw query should be normalized once, then classified into explicit clauses.

Supported clause families in version 1:

- `ExtClause`
- `WildcardClause`
- `PathClause`
- `NameClause`
- `PinyinInitialClause`
- `PinyinFullClause`

Examples:

- `*.png`
  - `ExtClause(png)`
- `foo*.md`
  - `NameClause(foo)`
  - `WildcardClause(foo*.md)`
  - `ExtClause(md)`
- `src/plugin`
  - `PathClause([src, plugin])`
- `search`
  - `NameClause(search)`
- `zsbg`
  - `PinyinInitialClause(zsbg)`
- `zongjie`
  - `PinyinFullClause(zongjie)`

The parser does not need to be perfect in version 1. It only needs to classify queries well enough to recall candidates without changing user-visible matching semantics unexpectedly.

### Short Query Policy

Very short queries should not blindly enter the gram-based recall path.

Version 1 should apply the following policy:

1. one-character queries:
   - skip gram indexes
   - use bounded filename prefix recall only
   - if even prefix recall is too broad, fall back to a tightly capped sequential scan path for that root
2. two-character queries:
   - use the bounded filename and pinyin-full bigram side indexes where available
   - keep stricter candidate caps than longer queries
   - path recall should prefer directory segments over path grams
3. three or more characters:
   - use the normal trigram-based recall path

This keeps short queries from exploding recall cost while preserving acceptable behavior for common launcher usage.

## Query Execution

### Phase 1: Recall

Recall should narrow the candidate set before scoring.

Execution rules:

1. Evaluate the most selective clauses first.
2. Multiple clauses use `AND` semantics by default.
3. Prefer intersection when multiple clauses are available.
4. When one clause produces a very small posting list, use it as the seed candidate set and validate the remaining clauses as sequential filters instead of intersecting large lists.
5. Wildcard clauses do not own a standalone inverted index in version 1. They may contribute narrowing through their literal components and extension clause, but full wildcard validation happens later.
6. Apply clause-level candidate caps before reranking.
7. Keep clause-level recall results observable for debugging and tuning.

Suggested selectivity order:

1. extension
2. pinyin initials trie
3. path segment
4. wildcard-derived narrowing
5. name grams
6. path grams
7. pinyin full grams

### Mixed-Query Strategy

Examples such as `foo*.md` should execute as a mixed-query plan:

1. `ExtClause(md)` recalls exact extension candidates.
2. `NameClause(foo)` recalls filename candidates.
3. The planner chooses the smaller recall set as the seed where practical.
4. Full wildcard validation runs after the seed candidate set is formed.

This avoids expensive intersections between very large posting lists when a smaller clause can act as the primary filter.

### Candidate Caps

Candidate control should happen in layers, not only once before rerank.

Version 1 should support:

1. `PerClauseLimit`
   - limits the result size returned by one clause
2. `PostIntersectionLimit`
   - limits the merged or intersected recall set
3. `PreRerankLimit`
   - limits the set that enters reranking

Low-selectivity clauses should be allowed to switch into `filter-only` mode when their expected posting list is too large to justify direct recall work.

### Phase 2: Rerank

Only recalled candidates should enter final scoring.

Rerank should preserve current search quality while making scoring rules more explicit:

- `name` match has the highest base weight
- `pinyin initials` and `pinyin full` have strong secondary weight
- `path` match has lower weight than filename match
- extension exactness and wildcard exactness apply as boosts

Version 1 should keep the existing fuzzy matcher as the scoring primitive where practical rather than introducing a second ranking engine. However, rerank should receive recall match hints so it can skip unnecessary work:

- candidates recalled through pinyin-specific clauses should go through the pinyin scoring path directly
- candidates recalled through filename clauses should not pay for redundant pinyin work unless needed for tie-breaking or fallback
- wildcard validation should run as a final filter on the recalled candidate set rather than against the full shard

This avoids repeating expensive pinyin matching work that recall has already partially resolved.

### Result Ordering

The final ordering should remain stable and predictable:

1. final score descending
2. directory preference only when scores are exactly equal
3. name ascending
4. path ascending

## Root Sharding

The query-index should be partitioned by `rootID`.

Reasons:

1. updates from the current scanner are already root-scoped
2. rebuild work stays local to one root
3. root add/remove operations remain simple
4. memory and token statistics become easier to observe per root

Queries run across all active root shards, but shard-internal recall happens independently before global result merge.

When a root is removed from the file search configuration, its shard should be removed entirely rather than left as an inactive cache entry.

## Build and Rebuild Flow

### Startup Build

On startup:

1. load entries from SQLite
2. group entries by `rootID`
3. build one `RootShard` per root
4. assemble a `QueryIndex` snapshot
5. atomically publish the completed snapshot

This avoids exposing partially built query-index state to search callers.

### Root Rebuild

Root rebuild should be used when:

- a root is first indexed
- a large reconcile occurs
- a patch batch exceeds thresholds
- query-index consistency is uncertain

Flow:

1. load current entries for the affected root from SQLite
2. build a new shard in the background
3. atomically swap the shard into the current query-index
4. retire the old shard

## Incremental Sync Strategy

Version 1 should use the "entry diff patch + root rebuild fallback" model.

### Why Entry Diffs

The query-index should not consume raw platform events directly.

Raw events are the wrong abstraction for this layer because:

- rename and move handling become error-prone
- directory-level changes can expand unpredictably
- platform-specific semantics leak upward

Instead, the query-index should update from stable entry-level state after reconcile has already determined the final snapshot result.

### Patch Operations

#### Add

1. assign or allocate a `DocID`
2. write `DocRecord`
3. derive tokens
4. insert `DocID` into relevant posting lists

Version 1 should reuse IDs from `freedDocIDs` before allocating new shard-local IDs so the shard stays compact over time.

#### Update

Version 1 should treat update as:

1. locate `DocID` from the old path
2. remove old tokens
3. update `DocRecord`
4. insert new tokens

The existing `DocID` should be preserved across update so rename and move operations do not create unnecessary ID churn.

No field-specific partial optimization is required in version 1.

#### Remove

1. locate `DocID`
2. remove `DocID` from all postings referenced by the old record
3. remove the record from `docTable`
4. remove old path mappings
5. push the released `DocID` into `freedDocIDs`

### Fallback to Root Rebuild

A patch batch should rebuild the full root shard instead of applying incremental changes when:

1. the batch size crosses a configured absolute threshold
2. the batch size crosses a configured percentage of the root's current document count
3. the change reason indicates directory-heavy restructuring
4. patch bookkeeping cannot reliably map old and new records
5. token list fragmentation becomes too high

Version 1 may start with conservative thresholds because correctness is more important than squeezing every possible incremental patch.

## Concurrency Model

Search readers should not block on long rebuilds.

Recommended model:

- search reads a stable `QueryIndex` snapshot
- patch and rebuild work happens on background copies
- publication happens through short atomic shard or snapshot replacement

Update scheduling should be serialized per root:

- each root owns a dedicated update worker or equivalent sequential queue
- batches for different roots may run in parallel
- patch and rebuild for the same root are mutually exclusive
- if a rebuild is in progress, new delta batches for that root queue behind it and are re-evaluated after rebuild publication
- if rebuild output already reflects the queued changes, queued deltas may be dropped safely

This can be implemented with pointer replacement guarded by a short lock or another equivalent snapshot publication mechanism.

The important constraint is behavioral, not mechanical:

- long query-index mutation must never hold the read path hostage

## Failure and Recovery

The query-index is a derived cache, not the authority.

Recovery rules:

1. if query-index patching fails, keep the current snapshot and schedule a root rebuild
2. if startup query-index build fails, search may temporarily fall back to scan mode for that root only if absolutely necessary, but the preferred behavior is to retry build and surface telemetry
3. if a shard becomes invalid, rebuild it from SQLite rather than trying to repair in place

This keeps correctness anchored in the persisted snapshot.

## Observability

Version 1 must add enough visibility to tune search behavior and memory later.

Required metrics or logs:

- query-index shard count
- documents per shard
- posting count per index family
- top hot tokens by posting length
- recall candidate count per query
- rerank candidate count per query
- recall time
- rerank time
- patch count
- rebuild count
- patch-to-rebuild fallback count
- approximate memory usage per shard and index family
- slow-query traces with:
  - raw query
  - query plan
  - clause-level recall counts
  - post-intersection count
  - pre-rerank count
  - recall time
  - rerank time

Without these signals it will be difficult to know whether slow queries come from weak recall, oversized candidate sets, or expensive reranking.

## Testing Strategy

Version 1 should add focused tests at the query-index layer and preserve plugin-level smoke coverage.

### Unit and Component Tests

1. build from entries:
   - all expected fields are indexed
2. recall:
   - extension
   - wildcard narrowing
   - filename text
   - path segment
   - path gram
   - pinyin full
   - pinyin initials
3. rerank:
   - filename beats weaker path-only matches
4. incremental patch:
   - add
   - update
   - remove
5. fallback rebuild:
   - large diff rebuild produces the same result set as a fresh build

### Integration and Smoke Tests

1. real plugin query for `*.png`
2. real plugin query for normal filename search
3. real plugin query for path fragment search
4. real plugin query for pinyin initials
5. incremental update cases:
   - create
   - rename
   - move
   - delete

## Rollout Plan

The implementation should land incrementally.

### Phase 1

Introduce the query-index component and root shard model with startup build support.

### Phase 2

Move local search to `parse -> recall -> rerank` for:

- extension
- filename
- path segment

### Phase 3

Add:

- path gram recall
- pinyin full recall
- pinyin initials recall

### Phase 4

Integrate entry-diff patching and root rebuild fallback.

### Phase 5

Tune recall thresholds, candidate caps, and memory behavior using observed metrics.

## Open Design Choices Left Explicit for Implementation Planning

The following items are intentionally deferred to the implementation plan because they affect tuning more than architecture:

1. exact token normalization rules and gram generation boundaries within the chosen trigram-first strategy
2. exact thresholds for patch vs rebuild
3. exact numeric values for layered candidate caps
4. exact score weighting formula for combined ranking
5. whether to keep the current scan path behind a debug or emergency fallback switch

These should be resolved during implementation planning with benchmark-driven defaults.

## Final Recommendation

Wox should add a cross-platform root-sharded `QueryIndex` for local file search and move query execution to a `recall -> rerank` model driven by stable entry diffs.

This gives the current Wox-owned snapshot architecture the missing query-time acceleration layer without coupling search performance to `USN`, `FSEvents`, Spotlight, or Everything. It also keeps the system evolvable: storage remains authoritative, the query-index remains replaceable, and platform-specific change detection remains below the query layer.
