# File Search Query Index Next Steps Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add root-scoped entry diff patching with rebuild fallback on top of the new local query-index path.

**Architecture:** Keep SQLite and scanner reconcile flow as the source of truth, but stop treating every root refresh as a mandatory full root rebuild. `LocalIndexProvider` computes a root-local diff from old vs new entries, then updates the affected `QueryIndex` shard in place when the batch is small enough and falls back to root rebuild when the batch is large or ambiguous.

**Tech Stack:** Go, SQLite, in-memory root-sharded inverted indexes, existing filesearch smoke and incremental tests

---

### Task 1: Root Diff Model

**Files:**
- Modify: `/mnt/c/dev/Wox/wox.core/util/filesearch/provider_local.go`
- Test: `/mnt/c/dev/Wox/wox.core/util/filesearch/provider_local_test.go`

- [ ] Add a failing test for root-local diff generation covering add, remove, and same-path update.
- [ ] Run: `go test ./util/filesearch -run TestDiffRootEntries -count=1`
- [ ] Implement root diff helpers in `provider_local.go`.
- [ ] Run: `go test ./util/filesearch -run TestDiffRootEntries -count=1`

### Task 2: Shard Patch Path

**Files:**
- Modify: `/mnt/c/dev/Wox/wox.core/util/filesearch/query_index.go`
- Test: `/mnt/c/dev/Wox/wox.core/util/filesearch/provider_local_test.go`

- [ ] Add a failing test for `ReplaceRootEntries` that exercises small-batch patch behavior without changing user-visible search results.
- [ ] Run: `go test ./util/filesearch -run TestLocalIndexProviderReplaceRootEntries -count=1`
- [ ] Implement shard patch helpers for add, update, remove, and docID reuse.
- [ ] Run: `go test ./util/filesearch -run TestLocalIndexProviderReplaceRootEntries -count=1`

### Task 3: Rebuild Fallback

**Files:**
- Modify: `/mnt/c/dev/Wox/wox.core/util/filesearch/provider_local.go`
- Test: `/mnt/c/dev/Wox/wox.core/util/filesearch/provider_local_test.go`

- [ ] Add a failing test for large root refresh batches forcing rebuild fallback.
- [ ] Run: `go test ./util/filesearch -run TestLocalIndexProviderReplaceRootEntriesFallback -count=1`
- [ ] Add threshold-based rebuild fallback in `ReplaceRootEntries`.
- [ ] Run: `go test ./util/filesearch -run TestLocalIndexProviderReplaceRootEntriesFallback -count=1`

### Task 4: Integration Verification

**Files:**
- Modify: `/mnt/c/dev/Wox/wox.core/util/filesearch/scanner_incremental_test.go`
- Test: `/mnt/c/dev/Wox/wox.core/test/plugin_test.go`

- [ ] Re-run the targeted root-refresh regression and plugin smoke coverage.
- [ ] Run: `go test ./util/filesearch -run TestScannerReloadLocalProviderFromDBRootOnlyRefreshesTargetRoot -count=1`
- [ ] Run: `go test ./test -run "TestFilePlugin_(WildcardExtensionFilter|PathFragmentSearch|PinyinInitialSearch)" -count=1`
- [ ] Run: `make build`
