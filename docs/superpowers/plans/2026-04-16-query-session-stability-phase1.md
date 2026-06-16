# Query Session Stability Phase 0/1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make launcher first-flush behavior stable enough that weak early partial results do not visibly take over top1 before stronger results arrive, while keeping query response latency bounded.

**Architecture:** Leave the existing fuzzy scoring and final snapshot ranking in place for now. Add deterministic backend smoke coverage around the WebSocket query pipeline, instrument first-flush corrections in `ui_impl.go`, then change first-flush commit from a pure timer gate to a confidence gate: commit immediately for `done`, otherwise wait until the snapshot is competitive enough or a bounded max-wait expires.

**Tech Stack:** Go backend, existing `wox.core/test` integration harness, WebSocket UI bridge, current launcher query snapshot pipeline

**Operator rule:** Do not change fuzzy ranking or plugin-specific scoring rules in this phase. This plan only covers instrumentation and first-flush commit policy.

---

## Planned File Layout

- Create: `/mnt/c/dev/Wox/wox.core/test/query_session_stability_test.go`
  Purpose: deterministic end-to-end smoke coverage for first-flush instability using test-only plugins and the existing WebSocket UI bridge
- Modify: `/mnt/c/dev/Wox/wox.core/ui/ui_impl.go`
  Purpose: first-flush instrumentation, confidence gating, and bounded max-wait behavior
- Modify: `/mnt/c/dev/Wox/wox.core/plugin/manager.go`
  Purpose: snapshot summary helpers and any minimal delay-policy plumbing needed by `ui_impl.go`

No Flutter file changes are planned in this phase. The backend should stop emitting visibly wrong early snapshots before any frontend reconciliation work is added.

### Task 1: Add deterministic first-flush smoke coverage through the real WebSocket query pipeline

**Files:**
- Create: `/mnt/c/dev/Wox/wox.core/test/query_session_stability_test.go`
- Use: `/mnt/c/dev/Wox/wox.core/test/test_base.go`
- Use: `/mnt/c/dev/Wox/wox.core/test/plugin_test.go`

- [ ] **Step 1: Add test-only system plugins in `query_session_stability_test.go` before service initialization**

Define two minimal test-only plugins in the `test` package and register them in `init()` by appending to `plugin.AllSystemPlugin`:

- `stabilityEarlyWeakPlugin`
  - supports `ignoreAutoScore`
  - returns exactly one weak helper result such as `"Open Stability Portfolio Settings"`
  - sleeps for about `5ms` before returning
  - only returns for a unique query string such as `"stability-portfolio"`
- `stabilityLateStrongPlugin`
  - supports `ignoreAutoScore`
  - returns exactly one strong primary result such as `"stability-portfolio"`
  - sleeps for about `70ms`
  - only returns for the same unique query string

Keep titles unique so existing real plugins do not interfere with assertions.

- [ ] **Step 2: Add a WebSocket query timeline helper that exercises `ui_impl.go`, not `Manager.Query(...)` directly**

In the same file, add a helper that:

- calls `ensureTestUIWebsocket(t)`
- dials the test UI WebSocket
- sends a `Query` request matching the launcher protocol
- collects every `Query` response message for the active `queryId`
- records, for each response:
  - receive timestamp
  - `isFinal`
  - result count
  - top1 title
  - ordered titles

Do not use `runQuery(...)` for these tests, because it bypasses `handleWebsocketQuery(...)`, first flush delay, and the debouncer path.

- [ ] **Step 3: Add the failing smoke test for the known bad pattern**

Add:

```go
func TestQueryFirstFlushWaitsForCompetitiveSnapshot(t *testing.T)
```

Assertions:

- issue a query for `"stability-portfolio"`
- collect the non-final `Query` responses
- the first visible non-final batch must already contain both test results
- the first visible top1 must be `"stability-portfolio"`, not the weak settings-like result

Current expected failure before implementation:

- the first non-final response contains only the weak early result
- the second response corrects top1 after the strong result arrives

- [ ] **Step 4: Add the bounded-latency smoke test for single-result queries**

Add:

```go
func TestQueryFirstFlushMaxWaitStillShowsSingleResult(t *testing.T)
```

Use one additional test-only plugin or a branch in the existing weak plugin so that:

- only one result ever exists for query `"stability-single"`
- the query does not finish immediately

Assertions:

- a non-final response still arrives before the final `done`
- the first visible response arrives within the configured max-wait window plus a small tolerance

This test protects the fix from becoming "wait for everything" instead of "wait until confidence or max-wait".

- [ ] **Step 5: Run the targeted backend smoke tests and verify the main one fails for the right reason**

Run:

```bash
cmd.exe /c "cd /d C:\dev\Wox\wox.core && go test ./test -run TestQueryFirstFlush -count=1 -v"
```

Expected before implementation:

- `TestQueryFirstFlushWaitsForCompetitiveSnapshot` fails because the first visible non-final response contains only the weak early result
- `TestQueryFirstFlushMaxWaitStillShowsSingleResult` can fail or remain pending depending on how the helper is wired, but the first test must clearly prove the current instability

### Task 2: Instrument first-flush corrections and snapshot confidence in the backend

**Files:**
- Modify: `/mnt/c/dev/Wox/wox.core/ui/ui_impl.go`
- Modify: `/mnt/c/dev/Wox/wox.core/plugin/manager.go`

- [ ] **Step 1: Add a compact snapshot summary helper in `manager.go`**

Add a small internal helper that can summarize a built snapshot for logging:

- top1 title
- top1 score
- top2 title
- top1/top2 score gap when available
- visible candidate count

Keep the helper internal to the backend; do not change public plugin APIs in this phase.

- [ ] **Step 2: Add first-flush lifecycle logging in `ui_impl.go`**

Enhance `handleWebsocketQuery(...)` so the query bridge logs:

- first-flush timer opening
- snapshot held because confidence is insufficient
- snapshot committed because:
  - `done`
  - `competitive_snapshot`
  - `max_wait`
- whether top1 changed between first commit and the next commit within an early correction window such as `120ms`

Use one log line per event, keeping it concise and comparable to the existing:

- `first flush delay`
- `result flushed (reason: first|tick|done)`

logs already emitted today.

- [ ] **Step 3: Re-run the failing smoke test and verify the logs expose the correction**

Run:

```bash
cmd.exe /c "cd /d C:\dev\Wox\wox.core && go test ./test -run TestQueryFirstFlushWaitsForCompetitiveSnapshot -count=1 -v"
```

Expected before the real fix:

- the test still fails
- verbose output and backend logs now show:
  - first flush attempted with one candidate
  - top1 later corrected when the strong result arrives

This task is complete only when the instrumentation makes the failure measurable, not anecdotal.

### Task 3: Replace pure time-based first flush with confidence-gated commit

**Files:**
- Modify: `/mnt/c/dev/Wox/wox.core/ui/ui_impl.go`
- Modify: `/mnt/c/dev/Wox/wox.core/plugin/manager.go`

- [ ] **Step 1: Add an explicit first-flush policy in `ui_impl.go`**

Implement a small policy in the query bridge closure, not in the generic debouncer:

- `done` always commits immediately
- before the first visible commit, non-final snapshots must satisfy one of:
  - candidate count is at least `2`
  - elapsed time since query start reached `firstFlushMaxWait`
  - future optional path: top1 dominates strongly enough

For this phase, do **not** add semantic-role heuristics yet.

- [ ] **Step 2: Keep the debouncer generic and gate only the query bridge**

Do not change the contract of `/mnt/c/dev/Wox/wox.core/util/debounce.go`.

Instead:

- let the debouncer continue batching plugin results
- build the snapshot in `ui_impl.go`
- decide whether this snapshot is allowed to become the first committed visible snapshot

This preserves existing generic debounce behavior for other call sites.

- [ ] **Step 3: Add a bounded max-wait constant and keep it local to the query bridge**

Add a small constant such as:

```go
const firstFlushMaxWaitMs = 96
```

Policy rules:

- if only one weak result arrives, hold it until either:
  - another result arrives and the snapshot becomes competitive enough
  - `firstFlushMaxWaitMs` expires
  - the query finishes

Do not increase the existing tick interval in this phase.

- [ ] **Step 4: Keep post-first-commit behavior unchanged**

After the first visible snapshot has been committed:

- keep existing tick and done behavior
- keep existing snapshot build ordering
- do not introduce session-stability bias yet

This phase is intentionally narrow: first visible snapshot quality only.

- [ ] **Step 5: Run the targeted smoke tests and verify green**

Run:

```bash
cmd.exe /c "cd /d C:\dev\Wox\wox.core && go test ./test -run TestQueryFirstFlush -count=1 -v"
```

Expected:

- `TestQueryFirstFlushWaitsForCompetitiveSnapshot` passes because the first visible non-final response already contains the strong top1
- `TestQueryFirstFlushMaxWaitStillShowsSingleResult` passes because single-result queries still paint before final completion

### Task 4: Verify build and document the remaining Phase 2 boundary

**Files:**
- Modify as needed: `/mnt/c/dev/Wox/wox.core/ui/ui_impl.go`
- Modify as needed: `/mnt/c/dev/Wox/wox.core/plugin/manager.go`
- Modify as needed: `/mnt/c/dev/Wox/wox.core/test/query_session_stability_test.go`

- [ ] **Step 1: Run the full backend build verification required by repo rules**

Run:

```bash
cmd.exe /c "cd /d C:\dev\Wox\wox.core && go build ./..."
cd /mnt/c/dev/Wox/wox.core && make build
```

Expected:

- `go build ./...` succeeds
- `make build` succeeds in a correctly provisioned backend environment

- [ ] **Step 2: Re-run one existing smoke slice to ensure no unrelated query regression**

Run:

```bash
cmd.exe /c "cd /d C:\dev\Wox\wox.core && go test ./test -run TestSystemPlugin -count=1"
```

Expected:

- existing query smoke coverage still passes

- [ ] **Step 3: Record Phase 2 boundary in code comments where needed**

If a short comment is necessary, clarify that this phase only gates first visible commit quality and intentionally does not yet implement:

- semantic role ranking
- prefix-extension stability bias
- frontend no-op snapshot skipping

Keep comments concise and in English.

- [ ] **Step 4: Prepare the next plan boundary**

After verification, stop. Do not continue into session-aware ranking in the same branch without a separate plan. The next phase should target:

- semantic result roles
- session stability bias for prefix-extension queries
- optional frontend no-op snapshot reconciliation
