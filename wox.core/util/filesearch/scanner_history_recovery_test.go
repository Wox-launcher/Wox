package filesearch

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Regression test: unknown-semantics file events on a degraded FSEvents root
// used to queue a full root reconcile per event; known file paths stay scoped.
func TestScannerDegradedRootKeepsUnknownFileDeltaScoped(t *testing.T) {
	ctx := context.Background()
	scanner := NewScanner(nil)
	scanner.replaceRootCache([]RootRecord{{
		ID:        "root-degraded-delta",
		Path:      "/tmp/degraded-root",
		Kind:      RootKindUser,
		Status:    RootStatusIdle,
		FeedType:  RootFeedTypeFSEvents,
		FeedState: RootFeedStateDegraded,
	}})

	scanner.handleChangeSignal(ctx, ChangeSignal{
		Kind:          ChangeSignalKindDirtyPath,
		SemanticKind:  ChangeSemanticKindUnknown,
		RootID:        "root-degraded-delta",
		FeedType:      RootFeedTypeFSEvents,
		Path:          "/tmp/degraded-root/some/file",
		PathIsDir:     false,
		PathTypeKnown: true,
		At:            time.Now(),
	})

	stats := scanner.dirtyQueue.Stats()
	if stats.RootSignalCount != 0 || stats.PathCount != 1 {
		t.Fatalf("expected unknown file delta to stay scoped on degraded root, got root_signals=%d paths=%d", stats.RootSignalCount, stats.PathCount)
	}
}

// Feeds can report history loss once per dropped event. All of them must
// collapse into one scheduled relist, and requests after a run wait for the
// cooldown instead of being dropped.
func TestScannerHistoryLossSignalsCollapseToOneRecovery(t *testing.T) {
	ctx := context.Background()
	scanner := NewScanner(nil)
	rootID := "root-history-loss"
	scanner.replaceRootCache([]RootRecord{{
		ID:        rootID,
		Path:      "/tmp/history-loss-root",
		Kind:      RootKindUser,
		Status:    RootStatusIdle,
		FeedType:  RootFeedTypeFSEvents,
		FeedState: RootFeedStateReady,
	}})

	at := time.Now()
	for i := range 50 {
		scanner.handleChangeSignal(ctx, ChangeSignal{
			Kind:         ChangeSignalKindHistoryLost,
			SemanticKind: ChangeSemanticKindHistoryLost,
			RootID:       rootID,
			Path:         "/tmp/history-loss-root",
			At:           at.Add(time.Duration(i) * time.Millisecond),
		})
	}

	if stats := scanner.dirtyQueue.Stats(); stats.RootCount != 0 {
		t.Fatalf("history loss must not queue a root reconcile, got %d roots", stats.RootCount)
	}
	if due := scanner.nextHistoryRecoveryAt(); !due.Equal(at) {
		t.Fatalf("expected one recovery due at the first loss, got %s", due)
	}

	// The root is unknown to this scanner, so the run finds nothing to relist.
	scanner.runDueHistoryRecoveries(ctx, at)
	if due := scanner.nextHistoryRecoveryAt(); !due.IsZero() {
		t.Fatalf("expected no pending recovery after the run, got %s", due)
	}

	scanner.requestHistoryRecovery(ctx, rootID, "second loss", time.Now())
	due := scanner.nextHistoryRecoveryAt()
	if due.Sub(time.Now()) < historyRecoveryCooldown-5*time.Second {
		t.Fatalf("expected a request inside the cooldown to wait for it, due in %s", time.Until(due))
	}
}

// End-to-end: offline or dropped changes are repaired by relisting changed
// directories, without re-crawling the whole root.
func TestScannerHistoryRecoveryRelistsChangedDirectories(t *testing.T) {
	db, ctx := openTestFileSearchDB(t)
	now := time.Now().UnixMilli()

	rootPath := t.TempDir()
	mustMkdirAll(t, filepath.Join(rootPath, "alpha", "beta"))
	mustMkdirAll(t, filepath.Join(rootPath, "gamma"))
	mustMkdirAll(t, filepath.Join(rootPath, "untouched"))
	mustWriteTestFile(t, filepath.Join(rootPath, "alpha", "keepfile.txt"), "keep")
	mustWriteTestFile(t, filepath.Join(rootPath, "alpha", "beta", "goneleaf.txt"), "gone")
	mustWriteTestFile(t, filepath.Join(rootPath, "gamma", "gammaleaf.txt"), "gamma")
	mustWriteTestFile(t, filepath.Join(rootPath, "untouched", "stillhere.txt"), "still")

	root := RootRecord{
		ID:        "root-history-recovery",
		Path:      rootPath,
		Kind:      RootKindUser,
		Status:    RootStatusIdle,
		CreatedAt: now,
		UpdatedAt: now,
	}
	mustInsertRoot(t, ctx, db, root)

	scanner := NewScanner(db)
	scanner.scanAllRootsWithReason(ctx, "test")
	if len(searchColumnsForTest(t, db, "goneleaf", 10)) == 0 {
		t.Fatal("expected initial scan to index goneleaf.txt")
	}

	// Ensure directory mtimes move past the indexed millisecond values.
	time.Sleep(20 * time.Millisecond)
	mustWriteTestFile(t, filepath.Join(rootPath, "alpha", "freshfile.txt"), "fresh")
	if err := os.Remove(filepath.Join(rootPath, "alpha", "beta", "goneleaf.txt")); err != nil {
		t.Fatalf("remove leaf: %v", err)
	}
	if err := os.RemoveAll(filepath.Join(rootPath, "gamma")); err != nil {
		t.Fatalf("remove directory: %v", err)
	}
	mustMkdirAll(t, filepath.Join(rootPath, "delta", "epsilon"))
	mustWriteTestFile(t, filepath.Join(rootPath, "delta", "epsilon", "deepnew.txt"), "deep")

	scanner.handleChangeSignal(ctx, ChangeSignal{
		Kind:         ChangeSignalKindHistoryLost,
		SemanticKind: ChangeSemanticKindHistoryLost,
		RootID:       root.ID,
		Path:         rootPath,
		Reason:       "test history loss",
		At:           time.Now(),
	})
	if degraded, ok := scanner.findRootByID(ctx, root.ID); !ok || degraded.FeedState != RootFeedStateDegraded {
		t.Fatalf("expected history loss to mark the root degraded, got %#v", degraded)
	}

	scanner.runDueHistoryRecoveries(ctx, time.Now())
	stats := scanner.dirtyQueue.Stats()
	if stats.RootSignalCount != 0 || stats.PathCount == 0 {
		t.Fatalf("expected scoped repair signals instead of a root reconcile, got root_signals=%d paths=%d", stats.RootSignalCount, stats.PathCount)
	}

	if err := scanner.processDirtyQueue(ctx, time.Now().Add(2*defaultDirtyDebounceWindow)); err != nil {
		t.Fatalf("process recovery signals: %v", err)
	}

	for _, name := range []string{"freshfile", "deepnew", "keepfile", "stillhere"} {
		if len(searchColumnsForTest(t, db, name, 10)) == 0 {
			t.Fatalf("expected %s to be indexed after recovery", name)
		}
	}
	for _, name := range []string{"goneleaf", "gammaleaf"} {
		if results := searchColumnsForTest(t, db, name, 10); len(results) != 0 {
			t.Fatalf("expected %s to be pruned after recovery, got %#v", name, results)
		}
	}
	if recovered, ok := scanner.findRootByID(ctx, root.ID); !ok || recovered.FeedState != RootFeedStateReady {
		t.Fatalf("expected recovery to return the root to ready, got %#v", recovered)
	}
}

// Removal events often arrive after the path is gone, so their type is unknown.
// They used to queue a whole-root reconcile; deleting by path must prune both a
// file and a directory with all of its descendants.
func TestScannerUnknownTypeRemovalPrunesDirectoryWithoutRootReconcile(t *testing.T) {
	db, ctx := openTestFileSearchDB(t)
	now := time.Now().UnixMilli()

	rootPath := t.TempDir()
	doomedPath := filepath.Join(rootPath, "doomed")
	mustMkdirAll(t, filepath.Join(doomedPath, "inner"))
	mustWriteTestFile(t, filepath.Join(doomedPath, "inner", "doomedleaf.txt"), "leaf")
	mustWriteTestFile(t, filepath.Join(rootPath, "survivor.txt"), "survivor")

	root := RootRecord{ID: "root-unknown-removal", Path: rootPath, Kind: RootKindUser, Status: RootStatusIdle, CreatedAt: now, UpdatedAt: now}
	mustInsertRoot(t, ctx, db, root)
	scanner := NewScanner(db)
	scanner.scanAllRootsWithReason(ctx, "test")
	if len(searchColumnsForTest(t, db, "doomedleaf", 10)) == 0 {
		t.Fatal("expected initial scan to index doomedleaf.txt")
	}

	if err := os.RemoveAll(doomedPath); err != nil {
		t.Fatalf("remove directory: %v", err)
	}
	scanner.handleChangeSignal(ctx, ChangeSignal{
		Kind:          ChangeSignalKindDirtyPath,
		SemanticKind:  ChangeSemanticKindRemove,
		RootID:        root.ID,
		FeedType:      RootFeedTypeFallback,
		Path:          doomedPath,
		PathTypeKnown: false,
		At:            time.Now(),
	})
	if stats := scanner.dirtyQueue.Stats(); stats.RootSignalCount != 0 || stats.PathCount != 1 {
		t.Fatalf("expected one path-level removal, got root_signals=%d paths=%d", stats.RootSignalCount, stats.PathCount)
	}
	if err := scanner.processDirtyQueue(ctx, time.Now().Add(2*defaultDirtyDebounceWindow)); err != nil {
		t.Fatalf("process removal: %v", err)
	}

	for _, name := range []string{"doomed", "doomedleaf"} {
		if results := searchColumnsForTest(t, db, name, 10); len(results) != 0 {
			t.Fatalf("expected %s to be removed, got %#v", name, results)
		}
	}
	if len(searchColumnsForTest(t, db, "survivor", 10)) == 0 {
		t.Fatal("expected unrelated entries to survive")
	}
}
