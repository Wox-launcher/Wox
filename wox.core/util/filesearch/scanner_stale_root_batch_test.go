package filesearch

import (
	"path/filepath"
	"testing"
	"time"
)

// Regression test: a deleted root's stale batches failed the flush with
// "root not found" and were requeued forever; the fix drops them at flush
// time while live roots still reconcile.
func TestScannerProcessDirtyQueueDropsStaleBatchesForDeletedRoot(t *testing.T) {
	db, ctx := openTestFileSearchDB(t)
	now := time.Now().UnixMilli()

	deletedRoot := RootRecord{
		ID:        "root-deleted-stale-batch",
		Path:      t.TempDir(),
		Kind:      RootKindUser,
		Status:    RootStatusIdle,
		CreatedAt: now,
		UpdatedAt: now,
	}
	mustInsertRoot(t, ctx, db, deletedRoot)

	liveRootPath := filepath.Join(t.TempDir(), "live")
	liveFilePath := filepath.Join(liveRootPath, "live.txt")
	mustMkdirAll(t, liveRootPath)
	mustWriteTestFile(t, liveFilePath, "live")
	liveRoot := RootRecord{
		ID:        "root-live-stale-batch",
		Path:      liveRootPath,
		Kind:      RootKindUser,
		Status:    RootStatusIdle,
		CreatedAt: now,
		UpdatedAt: now,
	}
	mustInsertRoot(t, ctx, db, liveRoot)

	scanner := NewScanner(db)
	scanner.scanAllRoots(ctx)

	// scanAllRoots kicks off background maintenance index building; wait for it
	// to release SQLite before deleting the root row.
	for i := 0; i < 100; i++ {
		if ready, err := db.EntryMaintenanceIndexesReady(ctx); err == nil && ready {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if err := db.DeleteRoot(ctx, deletedRoot.ID); err != nil {
		t.Fatalf("delete root: %v", err)
	}

	// Stale dirty signals for the deleted root are still queued after the
	// root row is gone, mirroring the production sequence.
	scanner.enqueueDirtyWithContext(ctx, DirtySignal{
		Kind:          DirtySignalKindRoot,
		RootID:        deletedRoot.ID,
		PathIsDir:     true,
		PathTypeKnown: true,
		At:            time.Now(),
	})
	scanner.enqueueDirtyWithContext(ctx, DirtySignal{
		Kind:          DirtySignalKindPath,
		SemanticKind:  ChangeSemanticKindCreate,
		RootID:        liveRoot.ID,
		Path:          liveFilePath,
		PathIsDir:     false,
		PathTypeKnown: true,
		At:            time.Now(),
	})

	processAt := time.Now().Add(2 * defaultDirtyDebounceWindow)
	if err := scanner.processDirtyQueue(ctx, processAt); err != nil {
		t.Fatalf("expected dirty flush to drop the stale deleted-root batch instead of failing: %v", err)
	}

	results := searchSQLiteForTest(t, db, "live", 10)
	if len(results) != 2 || results[0].Path != liveRootPath {
		t.Fatalf("expected live root to still reconcile alongside the dropped stale batch, got %#v", results)
	}

	// The stale batch must not be requeued for another retry loop.
	pendingRootCount, pendingPathCount := scanner.pendingDirtyCounts()
	if pendingRootCount != 0 || pendingPathCount != 0 {
		t.Fatalf("expected stale deleted-root batch to be dropped, pending roots=%d paths=%d", pendingRootCount, pendingPathCount)
	}
}
