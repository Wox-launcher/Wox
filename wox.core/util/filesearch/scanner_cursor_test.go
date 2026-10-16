package filesearch

import (
	"context"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

// TestScannerCommitsSealedCursor ensures finalization cannot acknowledge events
// arriving after the batch was sealed, and that queue coalescing keeps its cursor.
func TestScannerCommitsSealedCursor(t *testing.T) {
	db, ctx := openTestFileSearchDB(t)
	root := RootRecord{ID: "cursor-root", Path: t.TempDir(), Kind: RootKindUser, Status: RootStatusIdle, FeedType: RootFeedTypeFSEvents, FeedState: RootFeedStateReady}
	cursor := func(id uint64) string {
		return mustEncodeFeedCursorForTest(t, FeedCursor{FeedType: RootFeedTypeFSEvents, UpdatedAt: time.Now().UnixMilli(), FSEventID: id})
	}
	root.FeedCursor = cursor(10)
	mustInsertRoot(t, ctx, db, root)
	scanner := NewScanner(db)
	scanner.replaceRootCache([]RootRecord{root})
	feed := &cursorAcknowledgementFeed{ChangeFeed: newTestSnapshotChangeFeed(func(RootRecord) (RootFeedSnapshot, error) {
		t.Fatal("incremental finalize sampled the live event head")
		return RootFeedSnapshot{}, nil
	})}
	scanner.changeFeed = feed
	path := filepath.Join(root.Path, "changed.txt")
	mustWriteTestFile(t, path, "changed")
	now := time.Now()
	for _, id := range []uint64{20, 15} {
		scanner.handleChangeSignal(ctx, ChangeSignal{Kind: ChangeSignalKindDirtyPath, SemanticKind: ChangeSemanticKindModify, RootID: root.ID, FeedType: RootFeedTypeFSEvents, Path: path, PathTypeKnown: true, Cursor: cursor(id), At: now})
	}
	if err := scanner.processDirtyQueue(ctx, now.Add(2*defaultDirtyDebounceWindow)); err != nil {
		t.Fatal(err)
	}
	cached, found, _ := scanner.cachedRootByID(root.ID)
	got, ok := decodeFeedCursor(cached.FeedCursor, RootFeedTypeFSEvents)
	if !found || !ok || got.FSEventID != 20 {
		t.Fatalf("wrong acknowledgement: %+v", cached)
	}
	if len(feed.roots) != 1 || feed.roots[0].FeedCursor != cached.FeedCursor {
		t.Fatalf("feed not acknowledged after commit: %v", feed.roots)
	}
	// Finalization errors must not update the feed's durable boundary.
	failed := root
	failed.ID = "missing-root"
	if err := scanner.applyRunJobInternal(context.Background(), RunKindIncremental, failed, Job{Kind: JobKindFinalizeRoot}, nil); err == nil {
		t.Fatal("expected failed finalize")
	}
	if len(feed.roots) != 1 {
		t.Fatal("failed finalize acknowledged the feed")
	}
}

// cursorAcknowledgementFeed records the durable boundary exposed to the source.
type cursorAcknowledgementFeed struct {
	ChangeFeed
	roots []RootRecord
}

func (f *cursorAcknowledgementFeed) AcknowledgeRoot(root RootRecord) {
	f.roots = append(f.roots, root)
}

// TestScannerFullRunCapturesCursorBeforeTraversal guards against acknowledging
// events that arrive during a full scan but have not been processed yet.
func TestScannerFullRunCapturesCursorBeforeTraversal(t *testing.T) {
	db, ctx := openTestFileSearchDB(t)
	root := RootRecord{ID: "full-cursor", Path: t.TempDir(), Kind: RootKindUser, Status: RootStatusIdle, FeedType: RootFeedTypeFSEvents, FeedState: RootFeedStateReady}
	mustInsertRoot(t, ctx, db, root)
	mustWriteTestFile(t, filepath.Join(root.Path, "file.txt"), "content")
	scanner := NewScanner(db)
	var traversalStarted atomic.Bool
	scanner.SetStateChangeHandler(func(context.Context) { traversalStarted.Store(true) })
	calls := 0
	cursor := mustEncodeFeedCursorForTest(t, FeedCursor{FeedType: RootFeedTypeFSEvents, UpdatedAt: time.Now().UnixMilli(), FSEventID: 100})
	scanner.changeFeed = newTestSnapshotChangeFeed(func(RootRecord) (RootFeedSnapshot, error) {
		if traversalStarted.Load() {
			t.Fatal("sampled event head after traversal started")
		}
		calls++
		return RootFeedSnapshot{FeedType: RootFeedTypeFSEvents, FeedState: RootFeedStateReady, FeedCursor: cursor}, nil
	})
	if err := scanner.executePlannedRun(ctx, RunKindFull, "test", []RootRecord{root}, nil); err != nil {
		t.Fatal(err)
	}
	cached, found, _ := scanner.cachedRootByID(root.ID)
	if calls != 1 || !found || cached.FeedCursor != cursor {
		t.Fatalf("wrong full boundary: calls=%d root=%v", calls, cached)
	}
}
