package filesearch

import (
	"context"
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

// Regression test: FSEvents reports history loss per event, so reconcile
// signals must collapse inside the cooldown window and pass again after it.
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

	// A request after the cooldown window must pass again.
	scanner.handleChangeSignal(ctx, ChangeSignal{
		Kind:          ChangeSignalKindRequiresRootReconcile,
		SemanticKind:  ChangeSemanticKindRequiresRootReconcile,
		RootID:        rootID,
		FeedType:      RootFeedTypeFSEvents,
		Path:          "/tmp/gated-root",
		PathIsDir:     true,
		PathTypeKnown: true,
		Reason:        "fsevents flagged history loss or root change",
		At:            at.Add(2 * rootReconcileCooldown),
	})
	pendingRoots, _ := scanner.pendingDirtyCounts()
	if pendingRoots != 1 {
		t.Fatalf("expected post-cooldown reconcile to enqueue again, pending roots=%d", pendingRoots)
	}
}
