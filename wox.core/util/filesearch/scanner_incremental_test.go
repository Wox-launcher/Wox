package filesearch

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func makeTestEntryRecord(root RootRecord, fullPath string, isDir bool, size int64, mtime time.Time) EntryRecord {
	name := filepath.Base(fullPath)
	pinyinFull, pinyinInitials := buildPinyinFields(name)

	return EntryRecord{
		Path:           fullPath,
		RootID:         root.ID,
		ParentPath:     filepath.Dir(fullPath),
		Name:           name,
		NormalizedName: normalizeIndexText(name),
		NormalizedPath: normalizePath(fullPath),
		PinyinFull:     pinyinFull,
		PinyinInitials: pinyinInitials,
		IsDir:          isDir,
		Mtime:          mtime.UnixMilli(),
		Size:           size,
		UpdatedAt:      time.Now().UnixMilli(),
	}
}

func TestScannerProcessDirtyQueueUpdatesSQLiteAfterReconcile(t *testing.T) {
	db, ctx := openTestFileSearchDB(t)
	now := time.Now().UnixMilli()
	rootPath := filepath.Join(t.TempDir(), "root-incremental-reload")
	nestedDirPath := filepath.Join(rootPath, "nested")
	initialFilePath := filepath.Join(nestedDirPath, "initial.txt")
	newFilePath := filepath.Join(nestedDirPath, "new.txt")

	mustMkdirAll(t, nestedDirPath)
	mustWriteTestFile(t, initialFilePath, "initial")

	root := RootRecord{
		ID:        "root-incremental-reload",
		Path:      rootPath,
		Kind:      RootKindUser,
		Status:    RootStatusIdle,
		CreatedAt: now,
		UpdatedAt: now,
	}
	mustInsertRoot(t, ctx, db, root)

	scanner := NewScanner(db)
	scanner.dirtyQueueConfig = DirtyQueueConfig{
		DebounceWindow:               defaultDirtyDebounceWindow,
		SiblingMergeThreshold:        8,
		RootEscalationPathThreshold:  512,
		RootEscalationDirectoryRatio: 0,
	}
	scanner.dirtyQueue = NewDirtyQueue(scanner.dirtyQueueConfig)
	engine := &Engine{
		db:      db,
		scanner: scanner,
	}
	scanner.scanAllRoots(ctx)

	results := searchSQLiteForTest(t, db, "initial", 10)
	if len(results) != 1 || results[0].Path != initialFilePath {
		t.Fatalf("expected sqlite provider to include initial file %q after full build, got %#v", initialFilePath, results)
	}

	if err := os.Remove(initialFilePath); err != nil {
		t.Fatalf("remove initial file %q: %v", initialFilePath, err)
	}
	mustWriteTestFile(t, newFilePath, "new")
	// Direct file deltas no longer widen a create event to the parent directory,
	// so the test must model the watcher remove signal that evicts the old row.
	scanner.enqueueDirtyWithContext(ctx, DirtySignal{
		Kind:          DirtySignalKindPath,
		SemanticKind:  ChangeSemanticKindRemove,
		RootID:        root.ID,
		Path:          initialFilePath,
		PathIsDir:     false,
		PathTypeKnown: true,
		At:            time.Now(),
	})
	if ok := scanner.enqueueDirtyForPath(ctx, newFilePath); !ok {
		t.Fatalf("expected scanner to route dirty path %q to root %q", newFilePath, root.ID)
	}
	status, err := engine.GetStatus(ctx)
	if err != nil {
		t.Fatalf("get status after enqueueing dirty path: %v", err)
	}
	if status.PendingDirtyRootCount != 1 || status.PendingDirtyPathCount != 2 {
		t.Fatalf("expected pending dirty counts root=1 path=2 after enqueue, got root=%d path=%d", status.PendingDirtyRootCount, status.PendingDirtyPathCount)
	}
	processAt := time.Now().Add(2 * defaultDirtyDebounceWindow)

	if err := scanner.processDirtyQueue(ctx, processAt); err != nil {
		t.Fatalf("process dirty queue: %v", err)
	}

	results = searchSQLiteForTest(t, db, "new", 10)
	if len(results) != 1 || results[0].Path != newFilePath {
		t.Fatalf("expected sqlite provider to index new file %q, got %#v", newFilePath, results)
	}

	results = searchSQLiteForTest(t, db, "initial", 10)
	if len(results) != 0 {
		t.Fatalf("expected removed file %q to be evicted from sqlite provider, got %#v", initialFilePath, results)
	}
}

func TestScannerProcessDirtyQueueHandlesFileRenameSignals(t *testing.T) {
	db, ctx := openTestFileSearchDB(t)
	now := time.Now().UnixMilli()
	rootPath := filepath.Join(t.TempDir(), "root-rename-delta")
	oldFilePath := filepath.Join(rootPath, "rename-old-report.txt")
	newFilePath := filepath.Join(rootPath, "rename-new-report.txt")

	mustWriteTestFile(t, oldFilePath, "renamed")

	root := RootRecord{
		ID:        "root-rename-delta",
		Path:      rootPath,
		Kind:      RootKindUser,
		Status:    RootStatusIdle,
		CreatedAt: now,
		UpdatedAt: now,
	}
	mustInsertRoot(t, ctx, db, root)

	scanner := NewScanner(db)
	scanner.dirtyQueueConfig = DirtyQueueConfig{
		DebounceWindow:               defaultDirtyDebounceWindow,
		SiblingMergeThreshold:        8,
		RootEscalationPathThreshold:  512,
		RootEscalationDirectoryRatio: 0,
	}
	scanner.dirtyQueue = NewDirtyQueue(scanner.dirtyQueueConfig)
	scanner.scanAllRoots(ctx)

	results := searchSQLiteForTest(t, db, "rename-old-report", 10)
	if len(results) != 1 || results[0].Path != oldFilePath {
		t.Fatalf("expected old file %q after full build, got %#v", oldFilePath, results)
	}

	if err := os.Rename(oldFilePath, newFilePath); err != nil {
		t.Fatalf("rename file %q to %q: %v", oldFilePath, newFilePath, err)
	}
	// FSEvents reports ItemRenamed for both the disappeared old path and the
	// existing new path. Direct-delta must stat each rename path so the old row is
	// deleted while the new row is upserted, instead of treating rename as
	// delete-only.
	for _, path := range []string{oldFilePath, newFilePath} {
		scanner.enqueueDirtyWithContext(ctx, DirtySignal{
			Kind:          DirtySignalKindPath,
			SemanticKind:  ChangeSemanticKindRename,
			RootID:        root.ID,
			Path:          path,
			PathIsDir:     false,
			PathTypeKnown: true,
			At:            time.Now(),
		})
	}

	if err := scanner.processDirtyQueue(ctx, time.Now().Add(2*defaultDirtyDebounceWindow)); err != nil {
		t.Fatalf("process rename dirty queue: %v", err)
	}

	results = searchSQLiteForTest(t, db, "rename-old-report", 10)
	if len(results) != 0 {
		t.Fatalf("expected renamed old file %q to disappear, got %#v", oldFilePath, results)
	}

	results = searchSQLiteForTest(t, db, "rename-new-report", 10)
	if len(results) != 1 || results[0].Path != newFilePath {
		t.Fatalf("expected renamed new file %q to be searchable, got %#v", newFilePath, results)
	}
}

func TestScannerQueuesKnownFileRenameDeltaWhenRootIsDegraded(t *testing.T) {
	db, ctx := openTestFileSearchDB(t)
	now := time.Now().UnixMilli()
	rootPath := filepath.Join(t.TempDir(), "root-degraded-rename-delta")
	renamedFilePath := filepath.Join(rootPath, "renamed-report.txt")

	mustWriteTestFile(t, renamedFilePath, "renamed")

	root := RootRecord{
		ID:        "root-degraded-rename-delta",
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
	scanner.dirtyQueueConfig = DirtyQueueConfig{
		DebounceWindow:               defaultDirtyDebounceWindow,
		SiblingMergeThreshold:        8,
		RootEscalationPathThreshold:  512,
		RootEscalationDirectoryRatio: 0,
	}
	scanner.dirtyQueue = NewDirtyQueue(scanner.dirtyQueueConfig)

	signalAt := time.Now().Add(-3 * defaultDirtyDebounceWindow)
	scanner.handleChangeSignal(ctx, ChangeSignal{
		Kind:          ChangeSignalKindDirtyPath,
		SemanticKind:  ChangeSemanticKindRename,
		RootID:        root.ID,
		FeedType:      RootFeedTypeFSEvents,
		Path:          renamedFilePath,
		PathIsDir:     false,
		PathTypeKnown: true,
		At:            signalAt,
	})

	rootDirectoryCounts, _, _, err := scanner.loadDirtyQueueContext(ctx)
	if err != nil {
		t.Fatalf("load dirty queue context: %v", err)
	}
	batches := scanner.dirtyQueue.FlushReadyWithDebounce(time.Now(), rootDirectoryCounts, scanner.currentDirtyDebounceWindow())
	if len(batches) != 1 {
		t.Fatalf("expected one dirty batch, got %#v", batches)
	}
	if batches[0].Mode != ReconcileModeDirectDelta {
		t.Fatalf("expected degraded known file rename to stay direct-delta, got %s with paths=%#v", batches[0].Mode, batches[0].Paths)
	}
	if len(batches[0].DirectDeltas) != 1 || batches[0].DirectDeltas[0].Path != renamedFilePath {
		t.Fatalf("expected exact renamed file delta, got %#v", batches[0].DirectDeltas)
	}
}

func TestScannerDirtyDebounceWindowIsCappedByMaxPendingWait(t *testing.T) {
	scanner := NewScanner(nil)
	scanner.dirtyQueueConfig = DirtyQueueConfig{
		DebounceWindow:        2 * time.Minute,
		MaxPendingWaitWindow:  5 * time.Second,
		SiblingMergeThreshold: 8,
	}
	scanner.dirtyQueue = NewDirtyQueue(scanner.dirtyQueueConfig)

	now := time.Now()
	scanner.dirtyQueue.Push(DirtySignal{
		Kind:          DirtySignalKindPath,
		RootID:        "root-a",
		Path:          filepath.Join(string(filepath.Separator), "root", "first.txt"),
		PathTypeKnown: true,
		At:            now.Add(-4 * time.Second),
	})
	scanner.dirtyQueue.Push(DirtySignal{
		Kind:          DirtySignalKindPath,
		RootID:        "root-a",
		Path:          filepath.Join(string(filepath.Separator), "root", "latest.txt"),
		PathTypeKnown: true,
		At:            now.Add(-100 * time.Millisecond),
	})

	window := scanner.dirtyDebounceWindow()
	if window > 1500*time.Millisecond {
		t.Fatalf("expected max pending wait to cap dirty timer near 1s, got %s", window)
	}
	if window <= 0 {
		t.Fatalf("expected positive dirty timer window, got %s", window)
	}
}

func TestScannerHandleChangeSignalDoesNotPersistCursorBeforeReconcile(t *testing.T) {
	db, ctx := openTestFileSearchDB(t)
	now := time.Now().UnixMilli()
	rootPath := filepath.Join(t.TempDir(), "root-cursor-write-throttle")
	filePath := filepath.Join(rootPath, "changed.txt")
	initialCursor := mustEncodeFeedCursorForTest(t, FeedCursor{
		FeedType:  RootFeedTypeFSEvents,
		UpdatedAt: time.Now().Add(-time.Hour).UnixMilli(),
		FSEventID: 100,
	})
	nextCursor := mustEncodeFeedCursorForTest(t, FeedCursor{
		FeedType:  RootFeedTypeFSEvents,
		UpdatedAt: time.Now().UnixMilli(),
		FSEventID: 200,
	})

	mustWriteTestFile(t, filePath, "changed")
	mustInsertRoot(t, ctx, db, RootRecord{
		ID:         "root-cursor-write-throttle",
		Path:       rootPath,
		Kind:       RootKindUser,
		Status:     RootStatusIdle,
		FeedType:   RootFeedTypeFSEvents,
		FeedCursor: initialCursor,
		FeedState:  RootFeedStateReady,
		CreatedAt:  now,
		UpdatedAt:  now,
	})

	scanner := NewScanner(db)
	scanner.handleChangeSignal(ctx, ChangeSignal{
		Kind:          ChangeSignalKindDirtyPath,
		SemanticKind:  ChangeSemanticKindModify,
		RootID:        "root-cursor-write-throttle",
		FeedType:      RootFeedTypeFSEvents,
		Cursor:        nextCursor,
		Path:          filePath,
		PathIsDir:     false,
		PathTypeKnown: true,
		At:            time.Now(),
	})

	rootAfter, err := db.FindRootByID(ctx, "root-cursor-write-throttle")
	if err != nil {
		t.Fatalf("find root after change signal: %v", err)
	}
	if rootAfter.FeedCursor != initialCursor {
		t.Fatalf("expected dirty signal not to persist feed cursor before reconcile, got %q want %q", rootAfter.FeedCursor, initialCursor)
	}
}

func TestScannerProcessDirtyQueueReloadsDirectChildUnderRoot(t *testing.T) {
	db, ctx := openTestFileSearchDB(t)
	now := time.Now().UnixMilli()
	rootPath := filepath.Join(t.TempDir(), "root-direct-child")
	initialFilePath := filepath.Join(rootPath, "initial.txt")
	newFilePath := filepath.Join(rootPath, "sync-target.txt")

	mustMkdirAll(t, rootPath)
	mustWriteTestFile(t, initialFilePath, "initial")

	root := RootRecord{
		ID:        "root-direct-child",
		Path:      rootPath,
		Kind:      RootKindUser,
		Status:    RootStatusIdle,
		CreatedAt: now,
		UpdatedAt: now,
	}
	mustInsertRoot(t, ctx, db, root)

	scanner := NewScanner(db)
	scanner.dirtyQueueConfig = DirtyQueueConfig{
		DebounceWindow:               defaultDirtyDebounceWindow,
		SiblingMergeThreshold:        8,
		RootEscalationPathThreshold:  512,
		RootEscalationDirectoryRatio: 0,
	}
	scanner.dirtyQueue = NewDirtyQueue(scanner.dirtyQueueConfig)
	engine := &Engine{
		db:      db,
		scanner: scanner,
	}
	scanner.scanAllRoots(ctx)

	if err := os.Remove(initialFilePath); err != nil {
		t.Fatalf("remove initial file %q: %v", initialFilePath, err)
	}
	mustWriteTestFile(t, newFilePath, "new")
	if ok := scanner.enqueueDirtyForPath(ctx, newFilePath); !ok {
		t.Fatalf("expected scanner to route direct child dirty path %q to root %q", newFilePath, root.ID)
	}

	status, err := engine.GetStatus(ctx)
	if err != nil {
		t.Fatalf("get status after enqueueing direct child dirty path: %v", err)
	}
	if status.PendingDirtyRootCount != 1 || status.PendingDirtyPathCount != 1 {
		t.Fatalf("expected pending dirty counts root=1 path=1 after direct child enqueue, got root=%d path=%d", status.PendingDirtyRootCount, status.PendingDirtyPathCount)
	}

	if err := scanner.processDirtyQueue(ctx, time.Now().Add(2*defaultDirtyDebounceWindow)); err != nil {
		t.Fatalf("process dirty queue: %v", err)
	}

	results := searchSQLiteForTest(t, db, "sync-target", 10)
	if len(results) != 1 || results[0].Path != newFilePath {
		t.Fatalf("expected direct child file %q to be searchable after dirty processing, got %#v", newFilePath, results)
	}
}

func TestScannerProcessDirtyQueueRequeuesRemainingBatchesAfterFailure(t *testing.T) {
	db, ctx := openTestFileSearchDB(t)
	now := time.Now().UnixMilli()
	rootAPath := filepath.Join(t.TempDir(), "root-a")
	rootBPath := filepath.Join(t.TempDir(), "root-b")
	rootAScopePath := filepath.Join(rootAPath, "scope")
	rootBScopePath := filepath.Join(rootBPath, "scope")
	rootAChildPath := filepath.Join(rootAScopePath, "child")
	rootBChildPath := filepath.Join(rootBScopePath, "child")
	rootAInitialFilePath := filepath.Join(rootAScopePath, "initial-a.txt")
	rootBInitialFilePath := filepath.Join(rootBScopePath, "initial-b.txt")
	rootANewFilePath := filepath.Join(rootAChildPath, "new-a.txt")
	rootBNewFilePath := filepath.Join(rootBChildPath, "new-b.txt")

	mustMkdirAll(t, rootAChildPath)
	mustMkdirAll(t, rootBChildPath)
	mustWriteTestFile(t, rootAInitialFilePath, "initial-a")
	mustWriteTestFile(t, rootBInitialFilePath, "initial-b")

	mustInsertRoot(t, ctx, db, RootRecord{
		ID:        "root-a",
		Path:      rootAPath,
		Kind:      RootKindUser,
		Status:    RootStatusIdle,
		CreatedAt: now,
		UpdatedAt: now,
	})
	mustInsertRoot(t, ctx, db, RootRecord{
		ID:        "root-b",
		Path:      rootBPath,
		Kind:      RootKindUser,
		Status:    RootStatusIdle,
		CreatedAt: now,
		UpdatedAt: now,
	})

	scanner := NewScanner(db)
	scanner.dirtyQueueConfig = DirtyQueueConfig{
		DebounceWindow:               defaultDirtyDebounceWindow,
		SiblingMergeThreshold:        8,
		RootEscalationPathThreshold:  512,
		RootEscalationDirectoryRatio: 0,
	}
	scanner.dirtyQueue = NewDirtyQueue(scanner.dirtyQueueConfig)
	engine := &Engine{
		db:      db,
		scanner: scanner,
	}
	scanner.scanAllRoots(ctx)

	mustWriteTestFile(t, rootANewFilePath, "new-a")
	mustWriteTestFile(t, rootBNewFilePath, "new-b")

	outOfScopePath := filepath.Join(t.TempDir(), "outside-root-a", "broken.txt")
	scanner.enqueueDirty(DirtySignal{
		Kind:   DirtySignalKindPath,
		RootID: "root-a",
		Path:   outOfScopePath,
		At:     time.Now(),
	})
	if ok := scanner.enqueueDirtyForPath(ctx, rootBNewFilePath); !ok {
		t.Fatalf("expected scanner to route dirty path %q", rootBNewFilePath)
	}

	if err := scanner.processDirtyQueue(ctx, time.Now().Add(2*defaultDirtyDebounceWindow)); err == nil {
		t.Fatalf("expected dirty queue processing to fail for out-of-scope root-a batch")
	}

	failedRoot, err := db.FindRootByID(ctx, "root-a")
	if err != nil {
		t.Fatalf("load failed root after dirty queue error: %v", err)
	}
	if failedRoot.FeedState != RootFeedStateDegraded {
		t.Fatalf("expected failed root feed state degraded, got %q", failedRoot.FeedState)
	}

	status, err := engine.GetStatus(ctx)
	if err != nil {
		t.Fatalf("get status after failed dirty queue processing: %v", err)
	}
	if status.PendingDirtyRootCount != 1 || status.PendingDirtyPathCount != 1 {
		t.Fatalf("expected only unaffected dirty scope to stay queued after failure, got root=%d path=%d", status.PendingDirtyRootCount, status.PendingDirtyPathCount)
	}

	if err := scanner.processDirtyQueue(ctx, time.Now().Add(2*defaultDirtyDebounceWindow)); err != nil {
		t.Fatalf("process dirty queue after degraded root requeue: %v", err)
	}

	recoveredRoot, err := db.FindRootByID(ctx, "root-a")
	if err != nil {
		t.Fatalf("load failed root after scoped retry: %v", err)
	}
	if recoveredRoot.FeedState != RootFeedStateDegraded {
		t.Fatalf("expected failed root to stay degraded without a root-wide retry, got %q", recoveredRoot.FeedState)
	}

	results := searchSQLiteForTest(t, db, "new-a", 10)
	if len(results) != 0 {
		t.Fatalf("expected invalid failed scope not to trigger root-a full retry for %q, got %#v", rootANewFilePath, results)
	}

	results = searchSQLiteForTest(t, db, "new-b", 10)
	if len(results) != 1 || results[0].Path != rootBNewFilePath {
		t.Fatalf("expected root-b new file %q after retry, got %#v", rootBNewFilePath, results)
	}
}

func TestScannerProcessDirtyQueueCapturesFreshCursorAfterRootReconcile(t *testing.T) {
	db, ctx := openTestFileSearchDB(t)
	now := time.Now().UnixMilli()
	rootPath := filepath.Join(t.TempDir(), "root-reconcile-snapshot")
	initialFilePath := filepath.Join(rootPath, "initial.txt")
	updatedFilePath := filepath.Join(rootPath, "updated.txt")

	mustWriteTestFile(t, initialFilePath, "initial")

	expectedCursor := mustEncodeFeedCursorForTest(t, FeedCursor{
		FeedType:  RootFeedTypeFSEvents,
		UpdatedAt: time.Now().UnixMilli(),
		FSEventID: 222,
	})

	mustInsertRoot(t, ctx, db, RootRecord{
		ID:       "root-reconcile-snapshot",
		Path:     rootPath,
		Kind:     RootKindUser,
		Status:   RootStatusIdle,
		FeedType: RootFeedTypeFSEvents,
		FeedCursor: mustEncodeFeedCursorForTest(t, FeedCursor{
			FeedType:  RootFeedTypeFSEvents,
			UpdatedAt: time.Now().Add(-time.Hour).UnixMilli(),
			FSEventID: 100,
		}),
		FeedState: RootFeedStateUnavailable,
		CreatedAt: now,
		UpdatedAt: now,
	})

	scanner := NewScanner(db)
	scanner.changeFeed = newTestSnapshotChangeFeed(func(root RootRecord) (RootFeedSnapshot, error) {
		return RootFeedSnapshot{
			FeedType:   RootFeedTypeFSEvents,
			FeedCursor: expectedCursor,
			FeedState:  RootFeedStateReady,
		}, nil
	})
	scanner.scanAllRoots(ctx)

	if err := os.Remove(initialFilePath); err != nil {
		t.Fatalf("remove initial file: %v", err)
	}
	mustWriteTestFile(t, updatedFilePath, "updated")

	scanner.updateRootFeedState(ctx, "root-reconcile-snapshot", RootFeedStateUnavailable)
	scanner.enqueueDirty(DirtySignal{
		Kind:          DirtySignalKindRoot,
		RootID:        "root-reconcile-snapshot",
		Path:          rootPath,
		PathIsDir:     true,
		PathTypeKnown: true,
		At:            time.Now(),
	})

	if err := scanner.processDirtyQueue(ctx, time.Now().Add(2*defaultDirtyDebounceWindow)); err != nil {
		t.Fatalf("process dirty queue root reconcile: %v", err)
	}

	rootAfter, err := db.FindRootByID(ctx, "root-reconcile-snapshot")
	if err != nil {
		t.Fatalf("find root after root reconcile: %v", err)
	}
	if rootAfter == nil {
		t.Fatalf("expected root after root reconcile")
	}
	if rootAfter.FeedCursor != expectedCursor {
		t.Fatalf("expected root reconcile to persist fresh feed cursor %q, got %q", expectedCursor, rootAfter.FeedCursor)
	}
	if rootAfter.FeedState != RootFeedStateReady {
		t.Fatalf("expected root reconcile to recover feed state to ready, got %q", rootAfter.FeedState)
	}
}

func TestScopePathForDirtySignalPreservesFilesystemRootDirectory(t *testing.T) {
	scopePath, ok := scopePathForDirtySignal(string(filepath.Separator), true, true)
	if !ok {
		t.Fatalf("expected filesystem root to produce a scope")
	}
	if scopePath != string(filepath.Separator) {
		t.Fatalf("expected filesystem root to resolve back to %q, got %q", string(filepath.Separator), scopePath)
	}
}

func TestScannerQueuesDirtySignalsForNextRunDuringExecution(t *testing.T) {
	db, ctx := openTestFileSearchDB(t)
	now := time.Now().UnixMilli()
	rootPath := filepath.Join(t.TempDir(), "root-queue-next-run")
	nestedPath := filepath.Join(rootPath, "nested")
	initialFilePath := filepath.Join(nestedPath, "initial.txt")
	firstFilePath := filepath.Join(nestedPath, "first.txt")
	secondFilePath := filepath.Join(nestedPath, "second.txt")

	mustMkdirAll(t, nestedPath)
	mustWriteTestFile(t, initialFilePath, "initial")

	root := RootRecord{
		ID:        "root-queue-next-run",
		Path:      rootPath,
		Kind:      RootKindUser,
		Status:    RootStatusIdle,
		CreatedAt: now,
		UpdatedAt: now,
	}
	mustInsertRoot(t, ctx, db, root)

	scanner := NewScanner(db)
	scanner.plannerBudgetOverride = &splitBudget{
		LeafEntryBudget:     3,
		LeafWriteBudget:     3,
		LeafMemoryBudget:    1 << 20,
		DirectFileBatchSize: 1,
	}
	engine := &Engine{db: db, scanner: scanner}
	scanner.scanAllRoots(ctx)
	if err := db.BuildMaintenanceEntryIndexes(ctx); err != nil {
		t.Fatalf("build maintenance indexes after full scan: %v", err)
	}

	mustWriteTestFile(t, firstFilePath, "first")
	if ok := scanner.enqueueDirtyForPath(ctx, firstFilePath); !ok {
		t.Fatalf("expected scanner to route first dirty path %q", firstFilePath)
	}

	var (
		queuedSecondMu sync.Mutex
		queuedSecond   bool
	)
	scanner.SetStateChangeHandler(func(changeCtx context.Context) {
		status, err := engine.GetStatus(changeCtx)
		if err != nil {
			t.Fatalf("get status during incremental run: %v", err)
		}
		if status.ActiveStage != RunStageExecuting || status.ActiveRunStatus != RunStatusExecuting {
			return
		}
		queuedSecondMu.Lock()
		if queuedSecond {
			queuedSecondMu.Unlock()
			return
		}
		queuedSecond = true
		queuedSecondMu.Unlock()

		mustWriteTestFile(t, secondFilePath, "second")
		if ok := scanner.enqueueDirtyForPath(changeCtx, secondFilePath); !ok {
			t.Fatalf("expected scanner to route second dirty path %q", secondFilePath)
		}
	})

	if err := scanner.processDirtyQueue(ctx, time.Now().Add(2*defaultDirtyDebounceWindow)); err != nil {
		t.Fatalf("process first incremental run: %v", err)
	}

	status, err := engine.GetStatus(ctx)
	if err != nil {
		t.Fatalf("get status after first incremental run: %v", err)
	}
	if status.PendingDirtyRootCount != 1 || status.PendingDirtyPathCount != 1 {
		t.Fatalf("expected queued second signal for next run, got roots=%d paths=%d", status.PendingDirtyRootCount, status.PendingDirtyPathCount)
	}

	results := searchSQLiteForTest(t, db, "first", 10)
	if len(results) != 1 || results[0].Path != firstFilePath {
		t.Fatalf("expected first file %q after first incremental run, got %#v", firstFilePath, results)
	}

	if err := scanner.processDirtyQueue(ctx, time.Now().Add(4*defaultDirtyDebounceWindow)); err != nil {
		t.Fatalf("process second incremental run: %v", err)
	}

	status, err = engine.GetStatus(ctx)
	if err != nil {
		t.Fatalf("get status after second incremental run: %v", err)
	}
	if status.PendingDirtyRootCount != 0 || status.PendingDirtyPathCount != 0 {
		t.Fatalf("expected dirty queue to drain after second run, got roots=%d paths=%d", status.PendingDirtyRootCount, status.PendingDirtyPathCount)
	}

	results = searchSQLiteForTest(t, db, "second", 10)
	if len(results) != 1 || results[0].Path != secondFilePath {
		t.Fatalf("expected second file %q after second incremental run, got %#v", secondFilePath, results)
	}
}

func TestScannerIncrementalRunFailsFastAndKeepsQueue(t *testing.T) {
	db, ctx := openTestFileSearchDB(t)
	now := time.Now().UnixMilli()
	rootAPath := filepath.Join(t.TempDir(), "root-a-fast-fail")
	rootBPath := filepath.Join(t.TempDir(), "root-b-fast-fail")
	rootAChildPath := filepath.Join(rootAPath, "child")
	rootBChildPath := filepath.Join(rootBPath, "child")
	rootAFilePath := filepath.Join(rootAChildPath, "initial-a.txt")
	rootBFilePath := filepath.Join(rootBChildPath, "initial-b.txt")
	rootBNewFilePath := filepath.Join(rootBChildPath, "new-b.txt")

	mustMkdirAll(t, rootAChildPath)
	mustMkdirAll(t, rootBChildPath)
	mustWriteTestFile(t, rootAFilePath, "initial-a")
	mustWriteTestFile(t, rootBFilePath, "initial-b")

	mustInsertRoot(t, ctx, db, RootRecord{ID: "root-a-fast-fail", Path: rootAPath, Kind: RootKindUser, Status: RootStatusIdle, CreatedAt: now, UpdatedAt: now})
	mustInsertRoot(t, ctx, db, RootRecord{ID: "root-b-fast-fail", Path: rootBPath, Kind: RootKindUser, Status: RootStatusIdle, CreatedAt: now, UpdatedAt: now})

	scanner := NewScanner(db)
	scanner.dirtyQueueConfig = DirtyQueueConfig{
		DebounceWindow:               defaultDirtyDebounceWindow,
		SiblingMergeThreshold:        8,
		RootEscalationPathThreshold:  512,
		RootEscalationDirectoryRatio: 0,
	}
	scanner.dirtyQueue = NewDirtyQueue(scanner.dirtyQueueConfig)
	engine := &Engine{db: db, scanner: scanner}
	scanner.scanAllRoots(ctx)

	mustWriteTestFile(t, rootBNewFilePath, "new-b")
	outOfScopePath := filepath.Join(t.TempDir(), "outside-root-a-fast-fail", "broken.txt")
	scanner.enqueueDirty(DirtySignal{
		Kind:   DirtySignalKindPath,
		RootID: "root-a-fast-fail",
		Path:   outOfScopePath,
		At:     time.Now(),
	})
	if ok := scanner.enqueueDirtyForPath(ctx, rootBNewFilePath); !ok {
		t.Fatalf("expected scanner to route dirty path %q", rootBNewFilePath)
	}

	if err := scanner.processDirtyQueue(ctx, time.Now().Add(2*defaultDirtyDebounceWindow)); err == nil {
		t.Fatal("expected incremental run to fail fast")
	}

	status, err := engine.GetStatus(ctx)
	if err != nil {
		t.Fatalf("get status after failed incremental run: %v", err)
	}
	if status.PendingDirtyRootCount != 1 || status.PendingDirtyPathCount != 1 {
		t.Fatalf("expected failed incremental run to keep only unaffected scopes queued, got roots=%d paths=%d", status.PendingDirtyRootCount, status.PendingDirtyPathCount)
	}

	failedRoot, err := db.FindRootByID(ctx, "root-a-fast-fail")
	if err != nil {
		t.Fatalf("load failed root after incremental failure: %v", err)
	}
	if failedRoot == nil || failedRoot.FeedState != RootFeedStateDegraded {
		t.Fatalf("expected failed root feed state degraded, got %#v", failedRoot)
	}
}

func TestScannerIncrementalPermissionFailureStopsHotLoopingFailedRoot(t *testing.T) {
	db, ctx := openTestFileSearchDB(t)
	now := time.Now().UnixMilli()
	rootPath := filepath.Join(t.TempDir(), "root-permission-stop")
	root := RootRecord{
		ID:        "root-permission-stop",
		Path:      rootPath,
		Kind:      RootKindUser,
		Status:    RootStatusIdle,
		CreatedAt: now,
		UpdatedAt: now,
	}
	mustInsertRoot(t, ctx, db, root)

	scanner := NewScanner(db)
	engine := &Engine{db: db, scanner: scanner}
	batches := []ReconcileBatch{{
		RootID: root.ID,
		Mode:   ReconcileModeRoot,
	}}

	scanner.handleIncrementalRunFailure(ctx, []RootRecord{root}, batches, &runRootError{
		RootID: root.ID,
		Err:    &os.PathError{Op: "open", Path: filepath.Join(rootPath, "CSC"), Err: os.ErrPermission},
	})

	status, err := engine.GetStatus(ctx)
	if err != nil {
		t.Fatalf("get status after permission failure: %v", err)
	}
	if status.PendingDirtyRootCount != 0 || status.PendingDirtyPathCount != 0 {
		t.Fatalf("expected permission failure to stop requeueing failed root, got roots=%d paths=%d", status.PendingDirtyRootCount, status.PendingDirtyPathCount)
	}

	failedRoot, err := db.FindRootByID(ctx, root.ID)
	if err != nil {
		t.Fatalf("load failed root after permission failure: %v", err)
	}
	if failedRoot == nil {
		t.Fatal("expected failed root after permission failure")
	}
	if failedRoot.FeedState != RootFeedStateDegraded {
		t.Fatalf("expected permission failure to degrade feed state, got %#v", failedRoot)
	}
	if failedRoot.Status != RootStatusError {
		t.Fatalf("expected permission failure to persist root error status, got %#v", failedRoot)
	}
	if failedRoot.LastError == nil || *failedRoot.LastError == "" {
		t.Fatalf("expected permission failure to persist last error, got %#v", failedRoot)
	}
}

func TestRunPlannerIncrementalFailureKeepsActualFailedRootID(t *testing.T) {
	_, ctx := openTestFileSearchDB(t)
	now := time.Now().UnixMilli()
	rootAPath := filepath.Join(t.TempDir(), "root-planner-failure-a")
	rootBPath := filepath.Join(t.TempDir(), "root-planner-failure-b")
	rootAFilePath := filepath.Join(rootAPath, "ok.txt")
	rootBFilePath := filepath.Join(rootBPath, "bad.txt")
	rootBOutsidePath := filepath.Join(filepath.Dir(rootBPath), "outside")

	mustWriteTestFile(t, rootAFilePath, "ok")
	mustWriteTestFile(t, rootBFilePath, "bad")

	rootA := RootRecord{ID: "root-planner-failure-a", Path: rootAPath, Kind: RootKindUser, Status: RootStatusIdle, CreatedAt: now, UpdatedAt: now}
	rootB := RootRecord{ID: "root-planner-failure-b", Path: rootBPath, Kind: RootKindUser, Status: RootStatusIdle, CreatedAt: now, UpdatedAt: now}
	planner := NewRunPlanner(newPolicyState(Policy{}))
	_, err := planner.PlanIncrementalRun(ctx, []RootRecord{rootA, rootB}, []ReconcileBatch{
		{
			RootID: rootA.ID,
			Mode:   ReconcileModeSubtree,
			Paths:  []string{rootAPath},
		},
		{
			RootID: rootB.ID,
			Mode:   ReconcileModeSubtree,
			Paths:  []string{rootBOutsidePath},
		},
	})
	if err == nil {
		t.Fatal("expected incremental planner failure for out-of-root subtree path")
	}

	var rootErr *runRootError
	if !errors.As(err, &rootErr) || rootErr == nil {
		t.Fatalf("expected runRootError from incremental planner, got %T: %v", err, err)
	}
	if got, want := rootErr.RootID, rootB.ID; got != want {
		t.Fatalf("expected planner failure to keep actual failed root id, got %q want %q", got, want)
	}
}
