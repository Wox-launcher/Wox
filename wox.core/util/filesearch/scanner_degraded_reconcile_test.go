package filesearch

import (
	"context"
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

// Regression test (PR review): FSEvents reports history loss per event, so
// reconcile signals must collapse inside the cooldown window and pass again
// after it. A suppressed request is retained and fires at cooldown expiry even
// without any further event.
func TestScannerRepeatedRootReconcileSignalsCollapseToFirst(t *testing.T) {
	ctx := context.Background()
	scanner := NewScanner(nil)
	rootID := "root-reconcile-gate"

	at := time.Now()
	for i := 0; i < 50; i++ {
		scanner.handleChangeSignal(ctx, ChangeSignal{
			Kind:          ChangeSignalKindRequiresRootReconcile,
			SemanticKind:  ChangeSemanticKindRequiresRootReconcile,
			RootID:        rootID,
			FeedType:      RootFeedTypeFSEvents,
			Path:          "/tmp/gated-root",
			PathIsDir:     true,
			PathTypeKnown: true,
			Reason:        "fsevents flagged history loss or root change",
			At:            at.Add(time.Duration(i) * time.Millisecond),
		})
	}

	stats := scanner.dirtyQueue.Stats()
	if stats.RootCount != 1 {
		t.Fatalf("expected 50 reconcile signals to collapse to 1 queued root, got %d", stats.RootCount)
	}

	// Drain first: same-root signals coalesce into one pending entry, so without
	// draining a suppressed signal would be indistinguishable from an enqueued one.
	flushed := scanner.dirtyQueue.FlushReadyWithDebounce(at.Add(time.Minute), nil, defaultDirtyDebounceWindow)
	if len(flushed) != 1 {
		t.Fatalf("expected the collapsed root batch to flush, got %d batches", len(flushed))
	}
	if pendingRoots, _ := scanner.pendingDirtyCounts(); pendingRoots != 0 {
		t.Fatalf("expected empty queue after flush, pending roots=%d", pendingRoots)
	}

	// A request inside the cooldown window is suppressed but retained.
	scanner.handleChangeSignal(ctx, ChangeSignal{
		Kind:          ChangeSignalKindRequiresRootReconcile,
		SemanticKind:  ChangeSemanticKindRequiresRootReconcile,
		RootID:        rootID,
		FeedType:      RootFeedTypeFSEvents,
		Path:          "/tmp/gated-root",
		PathIsDir:     true,
		PathTypeKnown: true,
		Reason:        "fsevents flagged history loss or root change",
		At:            at.Add(10 * time.Second),
	})
	if pendingRoots, _ := scanner.pendingDirtyCounts(); pendingRoots != 0 {
		t.Fatalf("expected suppressed reconcile to be retained, not enqueued, pending roots=%d", pendingRoots)
	}

	// The retained request fires at cooldown expiry with no further event and
	// is enqueued with the release timestamp so the next flush consumes it.
	releaseAt := at.Add(2 * rootReconcileCooldown)
	scanner.releaseDueRootReconciles(ctx, releaseAt)
	pendingRoots, _ := scanner.pendingDirtyCounts()
	if pendingRoots != 1 {
		t.Fatalf("expected retained reconcile to enqueue at cooldown expiry, pending roots=%d", pendingRoots)
	}
	released := scanner.dirtyQueue.FlushReadyWithDebounce(releaseAt, nil, defaultDirtyDebounceWindow)
	if len(released) != 1 || released[0].Mode != ReconcileModeRoot || released[0].RootID != rootID {
		t.Fatalf("expected released retained reconcile to flush as a root batch, got %#v", released)
	}
}

// Regression test (PR review): a suppressed request must trigger a scan at
// cooldown expiry without any additional events. Verify end-to-end through
// processDirtyQueue that the retained reconcile reaches the reconciler.
func TestScannerSuppressedReconcileFiresWithoutFurtherEvents(t *testing.T) {
	db, ctx := openTestFileSearchDB(t)
	now := time.Now().UnixMilli()

	rootPath := t.TempDir()
	root := RootRecord{
		ID:        "root-suppressed-reconcile",
		Path:      rootPath,
		Kind:      RootKindUser,
		Status:    RootStatusIdle,
		CreatedAt: now,
		UpdatedAt: now,
	}
	mustInsertRoot(t, ctx, db, root)

	scanner := NewScanner(db)
	scanner.scanAllRoots(ctx)
	for i := 0; i < 100; i++ {
		if ready, err := db.EntryMaintenanceIndexesReady(ctx); err == nil && ready {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	// First reconcile signal passes and enqueues a root batch.
	signalAt := time.Now()
	scanner.handleChangeSignal(ctx, ChangeSignal{
		Kind:          ChangeSignalKindRequiresRootReconcile,
		SemanticKind:  ChangeSemanticKindRequiresRootReconcile,
		RootID:        root.ID,
		FeedType:      RootFeedTypeFSEvents,
		Path:          rootPath,
		PathIsDir:     true,
		PathTypeKnown: true,
		At:            signalAt,
	})
	if err := scanner.processDirtyQueue(ctx, signalAt.Add(defaultDirtyDebounceWindow)); err != nil {
		t.Fatalf("process first reconcile batch: %v", err)
	}

	// A new file appears, then a second reconcile signal arrives inside the
	// cooldown window. It must be retained, not dropped: the new file only
	// becomes searchable if the retained request fires later on its own.
	newFilePath := filepath.Join(rootPath, "late.txt")
	mustWriteTestFile(t, newFilePath, "late")
	scanner.handleChangeSignal(ctx, ChangeSignal{
		Kind:          ChangeSignalKindRequiresRootReconcile,
		SemanticKind:  ChangeSemanticKindRequiresRootReconcile,
		RootID:        root.ID,
		FeedType:      RootFeedTypeFSEvents,
		Path:          rootPath,
		PathIsDir:     true,
		PathTypeKnown: true,
		At:            signalAt.Add(time.Second),
	})
	if pendingRoots, _ := scanner.pendingDirtyCounts(); pendingRoots != 0 {
		t.Fatalf("expected suppressed request to be retained, pending roots=%d", pendingRoots)
	}

	// No further events of any kind: the release fires purely from the timer
	// path at cooldown expiry.
	if err := scanner.processDirtyQueue(ctx, signalAt.Add(2*rootReconcileCooldown)); err != nil {
		t.Fatalf("process released reconcile: %v", err)
	}
	results := searchSQLiteForTest(t, db, "late", 10)
	if len(results) == 0 {
		t.Fatalf("expected retained reconcile to scan and index the new file without further events")
	}
}
