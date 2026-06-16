# Query Session Stability Report

## Problem
Wox currently ranks and flushes query results as if every snapshot were an independent search result page. In practice, launcher search is a continuous session:

- the user extends the same query one character at a time
- plugins return results concurrently and at different speeds
- the UI receives multiple partial snapshots before the query is done
- the window height and visible ordering react to each snapshot

This creates two distinct but related instability classes:

1. **ranking instability**
   The final ranking can flip between semantically different result types during continuous input, even when the user intent has not changed.
2. **streaming instability**
   Early partial snapshots can temporarily expose a weak top result before a stronger result arrives a few milliseconds later.

The visible symptom is "list flashes, top result jumps, window resizes twice", but the root cause is broader: the current system has no explicit model for **query-session stability**.

## Scope
This report covers launcher query-result stability across:

- backend result scoring and ordering
- backend snapshot flush policy
- frontend list reconciliation and resize side effects

It does not propose:

- a plugin protocol rewrite
- a general fuzzy-matching rewrite in one step
- a launcher UI rewrite

The intent is to define a practical architecture that fits the current Wox codebase and can be rolled out incrementally.

## Executive Summary
The current system is optimized for "return matching results quickly", but not for "keep the visible result order stable while the user is still typing".

Three facts from the current implementation drive the problem:

1. **Result semantics are flattened into one `Score` field**
   Plugin activation results, plugin commands, plugin settings, and fallback-like results all compete in the same score space.
2. **Snapshots are committed based on time, not confidence**
   The first flush is driven by an estimated delay and any non-empty result batch, even if only one weak result has arrived.
3. **The frontend treats each snapshot as authoritative**
   The visible list is replaced on each incoming snapshot, so any transient reorder becomes a visible jump.

The recommended direction is:

- keep fuzzy matching as one input to ranking, not the whole ranking model
- introduce **session-aware ranking** in the backend
- introduce **confidence-based snapshot commit** for first flush and early ticks
- keep frontend logic simple, but make it more selective about when it re-renders

In short:

`visible order = committed session ranking`, not `latest raw partial score order`

## Current Implementation

### 1. Result Production
Each query fans out across matching plugins in parallel inside [`Manager.Query`](../../../wox.core/plugin/manager.go). Each plugin returns results independently through `queryParallel(...)`, and each result is polished and stored in the active session result cache through [`storeQueryResult(...)`](../../../wox.core/plugin/manager.go).

Relevant code:

- query fan-out: [`wox.core/plugin/manager.go`](../../../wox.core/plugin/manager.go)
- per-plugin query execution and latency tracking: [`wox.core/plugin/manager.go`](../../../wox.core/plugin/manager.go)
- result cache store: [`wox.core/plugin/manager.go`](../../../wox.core/plugin/manager.go)

Key property:

- the session cache is populated incrementally as plugin queries finish
- any snapshot built before all relevant plugins return is necessarily partial

### 2. Ranking
Each `QueryResult` exposes one main ranking number: [`QueryResult.Score`](../../../wox.core/plugin/query.go).

The backend adds to that score in [`PolishResult(...)`](../../../wox.core/plugin/manager.go):

- plugin-provided base score
- MRU / historical action score from [`calculateResultScore(...)`](../../../wox.core/plugin/manager.go)
- favorite bonus

Snapshot ordering currently happens in [`BuildQueryResultsSnapshot(...)`](../../../wox.core/plugin/manager.go), with a comparator that is still fundamentally score-led:

- score descending
- exact title match
- prefix match
- shorter title
- lexical fallback

This is better than raw map order, but it is still a **static snapshot comparator**. It does not know:

- whether the current query is an extension of the previous query
- whether the visible top result should be preserved unless clearly defeated
- whether two results have different semantic roles

### 3. String Matching
String matching is driven by:

- [`plugin.IsStringMatchScore(...)`](../../../wox.core/plugin/util.go)
- [`plugin.IsStringMatchScoreNoPinYin(...)`](../../../wox.core/plugin/util.go)
- [`fuzzymatch.FuzzyMatch(...)`](../../../wox.core/util/fuzzymatch/fuzzy_match.go)

Important characteristics of the current fuzzy matcher:

- exact match gets a large bonus
- prefix match gets a fixed bonus plus pattern-length contribution
- general fuzzy matches depend on boundary bonus, consecutive bonus, gap penalties, trailing penalties, and match ratio

This makes the matcher good at relevance, but it does **not** guarantee monotonic behavior across query extension. A candidate can gain or lose rank sharply when:

- a query crosses from prefix to exact
- a longer title benefits from a better boundary-aligned fuzzy path
- different result types expose different strings to the matcher

This is expected behavior for a relevance scorer. It becomes a UX problem only because the system currently uses the scorer output as the sole source of visible ordering truth.

### 4. First Flush And Snapshot Streaming
The UI bridge in [`ui_impl.go`](../../../wox.core/ui/ui_impl.go) computes a per-query first flush delay through [`GetQueryFirstFlushDelayMs(...)`](../../../wox.core/plugin/manager.go) and then feeds incremental plugin results into a generic [`Debouncer`](../../../wox.core/util/debounce.go).

Current first-flush behavior:

- first flush delay is based on average EWMA plugin latency
- minimum delay is currently `6ms`
- once the timer opens, any non-empty batch is eligible to flush
- snapshot content is whatever has already reached the session cache

This means first flush is **time-based**, not **confidence-based**.

### 5. Frontend Reconciliation
The Flutter launcher receives snapshots through [`onReceivedQueryResults(...)`](../../../wox.ui.flutter/wox/lib/controllers/wox_launcher_controller.dart) and replaces list/grid items through [`updateItems(...)`](../../../wox.ui.flutter/wox/lib/controllers/wox_base_list_controller.dart).

The frontend already has protections for:

- non-final empty batches
- delayed shrink for window resize
- short stale-result grace and placeholder behavior

However, the result list itself is still updated from each committed snapshot. If the backend commits an unstable partial ordering, the frontend faithfully displays it.

## Observed Failure Modes

### A. Final Ranking Flips Between Semantically Different Results
Example class:

- plugin activation result
- plugin settings result
- plugin command result

These results are not the same kind of answer, but the current system lets them compete in one score space.

This causes cases like:

- query prefix A: plugin entry is top
- query prefix B: settings result becomes top
- query prefix C: plugin entry becomes top again

Even when this is numerically correct under the current scorer, it is unstable relative to user intent.

### B. Early Partial Snapshots Expose A Temporary Wrong Top Result
Example class:

- a fast plugin returns one lower-quality result first
- a more relevant plugin returns 40-80ms later
- first flush publishes the temporary result as top1
- next flush corrects it

This is not a ranking bug. It is a commit-policy bug.

### C. Frontend Amplifies Snapshot Churn
Even when the backend changes only one or two rows, the frontend currently treats each committed snapshot as a new visible list. This amplifies:

- top1 jumps
- selection resets unless explicitly preserved
- layout and resize updates tied to result count changes

## Root Cause Analysis

The core problem is not any single plugin, and it is not only the fuzzy algorithm.

The real architectural gap is:

**Wox has a query snapshot model, but it does not yet have a query session stability model.**

That gap appears in three layers.

### 1. Missing Semantic Layer
`QueryResult` currently exposes `Score`, `Group`, and presentation fields, but there is no internal ranking concept such as:

- primary plugin trigger match
- plugin command
- plugin settings
- fallback result
- low-confidence helper result

Because of that, semantically different answers fight in one undifferentiated score space.

### 2. Missing Session Layer
Ranking is recomputed per snapshot without remembering enough about the previous visible state.

The current system does not ask:

- is the new query just a prefix extension of the old query?
- is the previous top result still a strong match?
- is the observed reorder meaningful enough to expose immediately?

### 3. Missing Commit-Confidence Layer
The backend sends snapshots when time allows, not when ranking confidence is high enough.

That is the key reason first flush can show a transient wrong top result.

## Design Goals

### Goals

- Keep visible top results stable during continuous input when user intent is still coherent.
- Preserve fast response for good results.
- Reduce visible reorder count without hiding meaningful ranking changes.
- Avoid breaking plugin SDKs in the first rollout.
- Keep backend as the main authority for ordering decisions.

### Non-Goals

- Do not make ordering fully sticky regardless of relevance.
- Do not freeze the UI until all plugins complete.
- Do not require all plugins to provide new metadata in the first iteration.
- Do not redesign fuzzy matching and streaming in one risky change.

## Architecture Options

### Option 1: Tune Fuzzy Matching Only
Adjust `FuzzyMatch(...)` bonuses and penalties to prefer shorter aliases, exact triggers, and monotonic extension behavior.

Pros:

- attacks instability near the relevance source
- benefits all search paths consistently

Cons:

- does not solve early partial flush problems
- difficult to prove globally correct across all plugins and languages
- risks broad search-quality regressions

Assessment:

- necessary eventually, but not the first lever

### Option 2: Tune Flush Policy Only
Keep current scoring, but make first flush and early ticks more conservative.

Pros:

- directly addresses "wrong result flashes first"
- low-risk and contained

Cons:

- does not solve semantically wrong final flips
- still leaves ranking fully score-driven

Assessment:

- useful and should be done, but not sufficient alone

### Option 3: Session-Aware Ranking Plus Confidence-Based Commit
Introduce a ranking pipeline that distinguishes:

- semantic role
- raw match relevance
- personalization
- session stability bias

Then expose snapshots only when the candidate ordering is confident enough.

Pros:

- addresses both ranking instability and first-flush instability
- aligns with user intent rather than isolated score values
- can be rolled out incrementally behind internal heuristics

Cons:

- more design work
- requires new internal metadata and logging
- needs careful instrumentation to avoid "sticky but wrong" behavior

Assessment:

- recommended

## Recommended Direction

### Overview
The recommended target model is:

`CommittedRank = SemanticPriority + MatchScore + Personalization + StabilityBias`

and

`VisibleSnapshot = Commit(CommittedRank, Confidence)`

This splits the problem into four concerns:

1. **semantic classification**
2. **static relevance**
3. **session stability**
4. **snapshot commit confidence**

### 1. Add Internal Result Semantics
Do not start by changing the public plugin API.

Instead, derive internal result semantics during polishing or snapshot construction. For example:

- `PrimaryTrigger`
- `PluginCommand`
- `PluginSettings`
- `GeneralResult`
- `Fallback`

Possible placement:

- internal metadata attached in `QueryResultCache`
- or derived in snapshot build from plugin instance, trigger keyword, title, subtitle, and system action context

This gives the backend a way to express "these results are not equivalent kinds of answers".

### 2. Keep Fuzzy Score As Relevance Input, Not Final Order
The existing fuzzy score should remain the main relevance signal, but it should stop being the only source of visible order.

Immediate implication:

- exact and prefix behavior still matter
- fuzzy boundary bonuses still matter
- but they operate inside a broader ranking model

This preserves existing plugin behavior while making room for stability policies.

### 3. Introduce Stability Bias For Prefix-Extension Queries
When a new query is a direct extension or minor refinement of the previous query:

- inspect the previous committed top result
- if it still matches strongly under the new query, add a temporary stability bias
- only allow a new result to overtake immediately if it beats the old result by a meaningful margin or represents a stronger semantic role under explicit intent

This should be query-session scoped, not global MRU.

Important:

- this is not "always keep old top1"
- it is "do not expose reorder unless the new ranking evidence is strong enough"

### 4. Add Commit Confidence To First Flush
First flush should no longer mean:

- timer elapsed
- at least one result exists

It should mean:

- enough evidence exists to expose a plausible top ordering

Practical first version:

- require at least a small candidate set for first flush, unless query completes
- or allow early flush if top1 dominates top2 by a strong enough margin
- or flush after a bounded max wait if not enough evidence arrives

This policy belongs in [`ui_impl.go`](../../../wox.core/ui/ui_impl.go), not in the generic debouncer utility.

### 5. Keep Frontend As A Conservative Presenter
The frontend should remain mostly simple:

- accept committed snapshots from backend
- skip no-op equivalent snapshots when possible
- preserve selected item by stable result identity where appropriate
- keep resize hysteresis independent from ranking logic

The frontend should not own ranking stabilization. It should only avoid amplifying backend churn.

## Concrete Changes By Layer

### Backend: Ranking
Primary touchpoints:

- [`wox.core/plugin/manager.go`](../../../wox.core/plugin/manager.go)
- [`wox.core/plugin/query.go`](../../../wox.core/plugin/query.go)

Recommended changes:

- add internal rank metadata near `QueryResultCache` or snapshot build
- replace pure score-led comparator with a composite ranking comparator
- keep the current lexical tie-breaker as a final fallback only

### Backend: Commit Policy
Primary touchpoints:

- [`wox.core/ui/ui_impl.go`](../../../wox.core/ui/ui_impl.go)
- [`wox.core/plugin/manager.go`](../../../wox.core/plugin/manager.go)
- [`wox.core/util/debounce.go`](../../../wox.core/util/debounce.go)

Recommended changes:

- keep `Debouncer` generic
- move first-flush decision policy into the query-handling closure in `ui_impl.go`
- compute first-flush confidence from:
  - result count
  - top1/top2 margin
  - whether previous committed top1 still survives
  - elapsed time since query start

### Frontend: Reconciliation
Primary touchpoints:

- [`wox.ui.flutter/wox/lib/controllers/wox_launcher_controller.dart`](../../../wox.ui.flutter/wox/lib/controllers/wox_launcher_controller.dart)
- [`wox.ui.flutter/wox/lib/controllers/wox_base_list_controller.dart`](../../../wox.ui.flutter/wox/lib/controllers/wox_base_list_controller.dart)

Recommended changes:

- detect visually equivalent snapshots and skip `updateItems(...)`
- preserve active result by result id when still visible
- keep current resize settle logic separate from ranking logic

## Rollout Plan

### Phase 0: Instrumentation
Add measurements before changing ranking behavior.

Track:

- first flush candidate count
- first flush top1 id
- whether top1 changes within 120ms after first flush
- number of visible reorders per query session
- query extension sequences where old top1 survives but loses visibility

Success metric:

- establish a baseline "visible correction rate" for first flush

### Phase 1: Commit Stability
Implement confidence-based first flush and early tick policy.

Target effect:

- eliminate most "temporary wrong top1" flashes

Why first:

- highest user-visible impact
- lowest API risk
- does not require changing plugin behavior

### Phase 2: Session-Aware Ranking
Implement internal result semantics and stability bias.

Target effect:

- reduce semantically wrong final flips across continuous input

Why second:

- more design-sensitive
- easier to evaluate once early flush noise is already reduced

### Phase 3: Frontend Reconciliation Hardening
Implement no-op snapshot detection and active-result preservation improvements.

Target effect:

- reduce the remaining visible churn from harmless snapshot differences

### Phase 4: Fuzzy Formula Refinement
Only after the previous phases are measured, refine fuzzy scoring itself.

Candidate areas:

- prefix-extension monotonicity
- shorter-alias protection
- boundary scoring for mixed-language titles

Why last:

- broadest blast radius
- easiest to mis-tune without stable upstream infrastructure

## Risks

### Sticky But Wrong Ranking
If stability bias is too strong, old top results may linger after user intent has actually changed.

Mitigation:

- limit bias to clear prefix-extension or refinement queries
- disable bias when explicit intent terms appear
- use bounded margins and time windows

### Over-Conservative First Flush
If first flush gating is too strict, Wox can feel sluggish.

Mitigation:

- use a bounded max wait
- log first-paint latency separately from first-flush latency
- measure correction-rate reduction against latency increase

### Semantic Heuristics Becoming Plugin-Specific
If internal semantic classification is implemented as one-off patches per plugin, the system will regress into case-by-case behavior.

Mitigation:

- define a small shared internal role model
- derive it through common rules first
- add plugin-specific exceptions only if a shared model cannot represent a class of results

## Open Questions

1. Should semantic role remain entirely internal in v1, or should Wox eventually expose an optional result-role hint in the plugin SDK?
2. How should query-session stability behave for non-prefix edits such as paste, replacement, or mid-string cursor edits?
3. Should first flush confidence be computed globally or only from the top N visible candidates?
4. Should active selection preservation be by index, by result id, or by "semantic equivalence" of the previously active result?

## Recommendation
Treat this as a **launcher query session stability** project, not a fuzzy-score tweak and not a single-plugin bug fix.

Recommended first implementation order:

1. instrument visible instability
2. fix first-flush confidence
3. implement internal semantic ranking plus stability bias
4. harden frontend reconciliation
5. tune fuzzy scoring only after the system can measure whether ranking changes are actually improvements

This direction matches the current codebase:

- backend already owns ranking and snapshot generation
- frontend already expects authoritative snapshots
- debouncer and flush logic are already centralized

The missing piece is not more local patching. The missing piece is a session-aware ranking and commit model.
