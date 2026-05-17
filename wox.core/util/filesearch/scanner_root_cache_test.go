package filesearch

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestScannerRootCacheHandlesChangeSignalWithoutDB(t *testing.T) {
	ctx := context.Background()
	rootPath := t.TempDir()
	filePath := filepath.Join(rootPath, "changed.txt")
	scanner := NewScanner(nil)
	root := RootRecord{
		ID:        "root-cache-signal",
		Path:      rootPath,
		Kind:      RootKindUser,
		Status:    RootStatusIdle,
		FeedType:  RootFeedTypeFSEvents,
		FeedState: RootFeedStateReady,
	}
	scanner.replaceRootCache([]RootRecord{root})

	scanner.handleChangeSignal(ctx, ChangeSignal{
		Kind:          ChangeSignalKindDirtyPath,
		SemanticKind:  ChangeSemanticKindModify,
		RootID:        root.ID,
		FeedType:      RootFeedTypeFSEvents,
		Path:          filePath,
		PathIsDir:     false,
		PathTypeKnown: true,
		At:            time.Now(),
	})

	stats := scanner.dirtyQueue.Stats()
	if stats.RootCount != 1 || stats.PathCount != 1 {
		t.Fatalf("expected cached signal to enqueue one dirty path without DB, got roots=%d paths=%d", stats.RootCount, stats.PathCount)
	}
}

func TestScannerDirtyEnqueueCoalescesStateNotifications(t *testing.T) {
	ctx := context.Background()
	rootPath := t.TempDir()
	scanner := NewScanner(nil)
	notificationCount := 0
	scanner.SetStateChangeHandler(func(context.Context) {
		notificationCount++
	})

	scanner.enqueueDirtyWithContext(ctx, DirtySignal{
		Kind:          DirtySignalKindPath,
		RootID:        "root-dirty-coalesce",
		Path:          filepath.Join(rootPath, "first.txt"),
		PathTypeKnown: true,
		At:            time.Now(),
	})
	scanner.enqueueDirtyWithContext(ctx, DirtySignal{
		Kind:          DirtySignalKindPath,
		RootID:        "root-dirty-coalesce",
		Path:          filepath.Join(rootPath, "second.txt"),
		PathTypeKnown: true,
		At:            time.Now(),
	})

	if notificationCount != 1 {
		t.Fatalf("expected one empty-to-pending state notification, got %d", notificationCount)
	}
	status := TransientSyncState{}
	activeState, ok := scanner.GetTransientSyncState()
	if ok {
		status = activeState
	}
	if !ok || status.PendingRootCount != 1 || status.PendingPathCount != 2 {
		t.Fatalf("expected exact pending counts after coalesced enqueue, ok=%v state=%#v", ok, status)
	}
}

func TestEngineGetStatusUsesLoadedRootCacheWithoutDB(t *testing.T) {
	rootPath := t.TempDir()
	scanner := NewScanner(nil)
	scanner.replaceRootCache([]RootRecord{{
		ID:              "root-status-cache",
		Path:            rootPath,
		Kind:            RootKindUser,
		Status:          RootStatusIdle,
		ProgressCurrent: RootProgressScale,
		ProgressTotal:   RootProgressScale,
	}})
	engine := &Engine{scanner: scanner}

	status, err := engine.GetStatus(context.Background())
	if err != nil {
		t.Fatalf("expected cached status without DB, got error: %v", err)
	}
	if status.RootCount != 1 || status.ProgressTotal != RootProgressScale {
		t.Fatalf("expected cached root in status, got %#v", status)
	}
}

func TestScannerRootCacheLoadedMissDoesNotQueryDB(t *testing.T) {
	scanner := NewScanner(nil)
	scanner.replaceRootCache(nil)

	if root, ok := scanner.findRootByID(context.Background(), "missing-root"); ok {
		t.Fatalf("expected loaded cache miss to ignore unknown root without DB, got %#v", root)
	}
}

func TestScannerRootCacheColdLookupFallsBackToDBAndSeedsPartialEntry(t *testing.T) {
	db, ctx := openTestFileSearchDB(t)
	now := time.Now().UnixMilli()
	root := RootRecord{
		ID:        "root-cache-db-fallback",
		Path:      t.TempDir(),
		Kind:      RootKindUser,
		Status:    RootStatusIdle,
		FeedState: RootFeedStateReady,
		CreatedAt: now,
		UpdatedAt: now,
	}
	mustInsertRoot(t, ctx, db, root)

	scanner := NewScanner(db)
	resolved, ok := scanner.findRootByID(ctx, root.ID)
	if !ok || resolved.ID != root.ID {
		t.Fatalf("expected cold cache lookup to resolve root from DB, got ok=%v root=%#v", ok, resolved)
	}
	cached, found, loaded := scanner.cachedRootByID(root.ID)
	if !found || loaded || cached.ID != root.ID {
		t.Fatalf("expected DB lookup to seed partial cache entry, got found=%v loaded=%v root=%#v", found, loaded, cached)
	}
}

func TestScannerRootCacheUpdateRootFeedStateRefreshesLoadedCache(t *testing.T) {
	db, ctx := openTestFileSearchDB(t)
	now := time.Now().UnixMilli()
	root := RootRecord{
		ID:        "root-cache-feed-state",
		Path:      t.TempDir(),
		Kind:      RootKindUser,
		Status:    RootStatusIdle,
		FeedType:  RootFeedTypeFSEvents,
		FeedState: RootFeedStateReady,
		CreatedAt: now,
		UpdatedAt: now,
	}
	mustInsertRoot(t, ctx, db, root)

	scanner := NewScanner(db)
	scanner.replaceRootCache([]RootRecord{root})
	scanner.updateRootFeedState(ctx, root.ID, RootFeedStateUnavailable)

	cached, found, loaded := scanner.cachedRootByID(root.ID)
	if !found || !loaded {
		t.Fatalf("expected loaded cache to keep updated root, found=%v loaded=%v", found, loaded)
	}
	if cached.FeedState != RootFeedStateUnavailable {
		t.Fatalf("expected cache feed state unavailable, got %q", cached.FeedState)
	}
	rootAfter, err := db.FindRootByID(ctx, root.ID)
	if err != nil {
		t.Fatalf("load root after feed-state update: %v", err)
	}
	if rootAfter == nil || rootAfter.FeedState != RootFeedStateUnavailable {
		t.Fatalf("expected DB feed state unavailable, got %#v", rootAfter)
	}
}

func TestScannerRootCacheUpsertsAfterIncrementalFinalize(t *testing.T) {
	db, ctx := openTestFileSearchDB(t)
	now := time.Now().UnixMilli()
	rootPath := t.TempDir()
	filePath := filepath.Join(rootPath, "finalized.txt")
	mustWriteTestFile(t, filePath, "finalized")
	root := RootRecord{
		ID:        "root-cache-finalize",
		Path:      rootPath,
		Kind:      RootKindUser,
		Status:    RootStatusIdle,
		FeedType:  RootFeedTypeFSEvents,
		FeedState: RootFeedStateDegraded,
		CreatedAt: now,
		UpdatedAt: now,
	}
	mustInsertRoot(t, ctx, db, root)

	scanner := NewScanner(db)
	scanner.changeFeed = newTestSnapshotChangeFeed(func(root RootRecord) (RootFeedSnapshot, error) {
		return RootFeedSnapshot{
			FeedType:  RootFeedTypeFSEvents,
			FeedState: RootFeedStateReady,
		}, nil
	})
	scanner.replaceRootCache([]RootRecord{root})
	scanner.enqueueDirty(DirtySignal{
		Kind:          DirtySignalKindRoot,
		RootID:        root.ID,
		Path:          root.Path,
		PathIsDir:     true,
		PathTypeKnown: true,
		At:            time.Now(),
	})

	if err := scanner.processDirtyQueue(ctx, time.Now().Add(2*defaultDirtyDebounceWindow)); err != nil {
		t.Fatalf("process dirty queue: %v", err)
	}

	cached, found, loaded := scanner.cachedRootByID(root.ID)
	if !found || !loaded {
		t.Fatalf("expected incremental finalize to keep complete root cache loaded, found=%v loaded=%v", found, loaded)
	}
	if cached.FeedState != RootFeedStateReady {
		t.Fatalf("expected finalized root cache to become ready, got %q", cached.FeedState)
	}

	scanner.db = nil
	resolved, ok := scanner.findRootByID(ctx, root.ID)
	if !ok {
		t.Fatalf("expected root lookup after finalize to hit cache without DB")
	}
	if resolved.FeedState != RootFeedStateReady {
		t.Fatalf("expected finalized root to reload as ready, got %q", resolved.FeedState)
	}
}

func TestScannerRefreshChangeFeedWithRootsReplacesCache(t *testing.T) {
	ctx := context.Background()
	feed := newTestSnapshotChangeFeed(nil)
	scanner := NewScanner(nil)
	scanner.changeFeed = feed
	root := RootRecord{
		ID:        "root-cache-refresh-direct",
		Path:      t.TempDir(),
		Kind:      RootKindUser,
		Status:    RootStatusIdle,
		FeedState: RootFeedStateReady,
	}

	scanner.refreshChangeFeedWithRoots(ctx, []RootRecord{root})

	cached, found, loaded := scanner.cachedRootByID(root.ID)
	if !found || !loaded || cached.ID != root.ID {
		t.Fatalf("expected non-nil refresh roots to replace cache, found=%v loaded=%v root=%#v", found, loaded, cached)
	}
	refreshedRoots := feed.refreshedRoots()
	if len(refreshedRoots) != 1 || refreshedRoots[0].ID != root.ID {
		t.Fatalf("expected change feed to receive direct refresh root, got %#v", refreshedRoots)
	}
}

func TestScannerRefreshChangeFeedWithNilRootsLoadsDBAndReplacesCache(t *testing.T) {
	db, ctx := openTestFileSearchDB(t)
	now := time.Now().UnixMilli()
	root := RootRecord{
		ID:        "root-cache-refresh-db",
		Path:      t.TempDir(),
		Kind:      RootKindUser,
		Status:    RootStatusIdle,
		FeedState: RootFeedStateReady,
		CreatedAt: now,
		UpdatedAt: now,
	}
	mustInsertRoot(t, ctx, db, root)
	feed := newTestSnapshotChangeFeed(nil)
	scanner := NewScanner(db)
	scanner.changeFeed = feed

	scanner.refreshChangeFeedWithRoots(ctx, nil)

	cached, found, loaded := scanner.cachedRootByID(root.ID)
	if !found || !loaded || cached.ID != root.ID {
		t.Fatalf("expected nil refresh roots to load DB and replace cache, found=%v loaded=%v root=%#v", found, loaded, cached)
	}
	refreshedRoots := feed.refreshedRoots()
	if len(refreshedRoots) != 1 || refreshedRoots[0].ID != root.ID {
		t.Fatalf("expected change feed to receive DB-loaded refresh root, got %#v", refreshedRoots)
	}
}

func TestEngineRootMutationsInvalidateRootCacheBeforeRescan(t *testing.T) {
	db, ctx := openTestFileSearchDB(t)
	now := time.Now().UnixMilli()
	existingRootPath := t.TempDir()
	existingRoot := RootRecord{
		ID:        "root-cache-engine-existing",
		Path:      existingRootPath,
		Kind:      RootKindUser,
		Status:    RootStatusIdle,
		CreatedAt: now,
		UpdatedAt: now,
	}
	mustInsertRoot(t, ctx, db, existingRoot)

	scanner := NewScanner(db)
	engine := &Engine{db: db, scanner: scanner}
	scanner.replaceRootCache([]RootRecord{existingRoot})
	addedRootPath := t.TempDir()
	if err := engine.AddRoot(ctx, addedRootPath); err != nil {
		t.Fatalf("add root: %v", err)
	}
	if _, _, loaded := scanner.cachedRootByID(existingRoot.ID); loaded {
		t.Fatalf("expected AddRoot to invalidate root cache")
	}

	scanner.replaceRootCache([]RootRecord{existingRoot})
	if err := engine.RemoveRoot(ctx, existingRootPath); err != nil {
		t.Fatalf("remove root: %v", err)
	}
	if _, _, loaded := scanner.cachedRootByID(existingRoot.ID); loaded {
		t.Fatalf("expected RemoveRoot to invalidate root cache")
	}

	scanner.replaceRootCache([]RootRecord{{
		ID:     "root-cache-engine-added",
		Path:   addedRootPath,
		Kind:   RootKindUser,
		Status: RootStatusIdle,
	}})
	changed, err := syncUserRootsToDB(ctx, db, scanner, nil, false)
	if err != nil {
		t.Fatalf("sync user roots: %v", err)
	}
	if !changed {
		t.Fatalf("expected sync user roots to remove existing user roots")
	}
	if _, _, loaded := scanner.cachedRootByID("root-cache-engine-added"); loaded {
		t.Fatalf("expected syncUserRootsToDB to invalidate root cache")
	}
}
