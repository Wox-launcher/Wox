package filesearch

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestScannerScanAllRootsPersistsDirectorySnapshotsAndFullScanTimestamp(t *testing.T) {
	db, ctx := openTestFileSearchDB(t)
	now := time.Now().UnixMilli()
	rootPath := filepath.Join(t.TempDir(), "root-full-scan")
	levelOnePath := filepath.Join(rootPath, "level-one")
	levelTwoPath := filepath.Join(levelOnePath, "level-two")
	filePath := filepath.Join(levelTwoPath, "target.txt")

	mustWriteTestFile(t, filePath, "target")

	root := RootRecord{
		ID:        "root-full-scan",
		Path:      rootPath,
		Kind:      RootKindUser,
		Status:    RootStatusIdle,
		CreatedAt: now,
		UpdatedAt: now,
	}
	mustInsertRoot(t, ctx, db, root)

	scanner := NewScanner(db)
	scanner.scanAllRoots(ctx)

	rootAfter, err := db.FindRootByID(ctx, root.ID)
	if err != nil {
		t.Fatalf("find root after full scan: %v", err)
	}
	if rootAfter == nil {
		t.Fatalf("expected root %q to exist after full scan", root.ID)
	}
	if rootAfter.LastFullScanAt <= 0 {
		t.Fatalf("expected full scan timestamp to be recorded, got %d", rootAfter.LastFullScanAt)
	}

	directoryCount, err := db.CountDirectoriesByRoot(ctx, root.ID)
	if err != nil {
		t.Fatalf("count directory snapshots by root: %v", err)
	}
	if directoryCount != 3 {
		t.Fatalf("expected 3 live directory snapshots after full scan, got %d", directoryCount)
	}

	directories, err := db.ListDirectoriesByRoot(ctx, root.ID)
	if err != nil {
		t.Fatalf("list directory snapshots after full scan: %v", err)
	}

	seen := map[string]bool{}
	for _, directory := range directories {
		if !directory.Exists {
			t.Fatalf("expected full scan directory snapshot %q to be live", directory.Path)
		}
		seen[directory.Path] = true
	}

	expectedPaths := []string{rootPath, levelOnePath, levelTwoPath}
	for _, expectedPath := range expectedPaths {
		if !seen[expectedPath] {
			t.Fatalf("expected full scan directory snapshot %q to exist", expectedPath)
		}
	}
}

func TestScannerScanAllRootsCapturesFreshRootFeedSnapshot(t *testing.T) {
	db, ctx := openTestFileSearchDB(t)
	now := time.Now().UnixMilli()
	rootPath := filepath.Join(t.TempDir(), "root-full-scan-snapshot")
	filePath := filepath.Join(rootPath, "target.txt")

	mustWriteTestFile(t, filePath, "target")

	initialCursor := mustEncodeFeedCursorForTest(t, FeedCursor{
		FeedType:  RootFeedTypeFSEvents,
		UpdatedAt: time.Now().Add(-26 * time.Hour).UnixMilli(),
		FSEventID: 12,
	})
	mustInsertRoot(t, ctx, db, RootRecord{
		ID:         "root-full-scan-snapshot",
		Path:       rootPath,
		Kind:       RootKindUser,
		Status:     RootStatusIdle,
		FeedType:   RootFeedTypeFSEvents,
		FeedCursor: initialCursor,
		FeedState:  RootFeedStateUnavailable,
		CreatedAt:  now,
		UpdatedAt:  now,
	})

	expectedCursor := mustEncodeFeedCursorForTest(t, FeedCursor{
		FeedType:  RootFeedTypeFSEvents,
		UpdatedAt: time.Now().UnixMilli(),
		FSEventID: 99,
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

	rootAfter, err := db.FindRootByID(ctx, "root-full-scan-snapshot")
	if err != nil {
		t.Fatalf("find root after snapshotting full scan: %v", err)
	}
	if rootAfter == nil {
		t.Fatalf("expected root to exist after full scan")
	}
	if rootAfter.FeedType != RootFeedTypeFSEvents {
		t.Fatalf("expected full scan to persist fsevents feed type, got %q", rootAfter.FeedType)
	}
	if rootAfter.FeedCursor != expectedCursor {
		t.Fatalf("expected full scan to persist fresh feed cursor %q, got %q", expectedCursor, rootAfter.FeedCursor)
	}
	if rootAfter.FeedState != RootFeedStateReady {
		t.Fatalf("expected full scan to recover root feed state to ready, got %q", rootAfter.FeedState)
	}
}

func TestNewScannerUsesSpecDirtyQueueDefaults(t *testing.T) {
	db, ctx := openTestFileSearchDB(t)
	_ = ctx

	scanner := NewScanner(db)

	if scanner.dirtyQueueConfig.SiblingMergeThreshold != 8 {
		t.Fatalf("expected sibling merge threshold 8, got %d", scanner.dirtyQueueConfig.SiblingMergeThreshold)
	}
	if scanner.dirtyQueueConfig.RootEscalationPathThreshold != 0 {
		t.Fatalf("expected root escalation path threshold 0, got %d", scanner.dirtyQueueConfig.RootEscalationPathThreshold)
	}
	if scanner.dirtyQueueConfig.RootEscalationDirectoryRatio != 0 {
		t.Fatalf("expected root escalation directory ratio 0, got %f", scanner.dirtyQueueConfig.RootEscalationDirectoryRatio)
	}
}

func TestScannerScanAllRootsLeavesExistingSearchResultsAvailableDuringVerification(t *testing.T) {
	db, ctx := openTestFileSearchDB(t)
	now := time.Now().UnixMilli()
	rootPath := filepath.Join(t.TempDir(), "root-provider-reload")
	filePath := filepath.Join(rootPath, "existing.txt")

	mustWriteTestFile(t, filePath, "existing")
	mustInsertRoot(t, ctx, db, RootRecord{
		ID:        "root-provider-reload",
		Path:      rootPath,
		Kind:      RootKindUser,
		Status:    RootStatusIdle,
		CreatedAt: now,
		UpdatedAt: now,
	})

	scanner := NewScanner(db)
	scanner.scanAllRoots(ctx)

	results := searchSQLiteForTest(t, db, "existing", 10)
	if len(results) != 1 || results[0].Path != filePath {
		t.Fatalf("expected sqlite update to include %q, got %#v", filePath, results)
	}
}

func TestScannerStartupRestoreUsesPersistedSQLiteWithoutFullScanForFreshCursor(t *testing.T) {
	db, ctx := openTestFileSearchDB(t)
	now := time.Now()
	rootPath := filepath.Join(t.TempDir(), "root-startup-restore-fresh")
	staleFilePath := filepath.Join(rootPath, "stale.txt")
	lastFullScanAt := now.Add(-time.Hour).UnixMilli()

	root := RootRecord{
		ID:             "root-startup-restore-fresh",
		Path:           rootPath,
		Kind:           RootKindUser,
		Status:         RootStatusIdle,
		FeedType:       RootFeedTypeFSEvents,
		FeedCursor:     mustEncodeFeedCursorForTest(t, FeedCursor{FeedType: RootFeedTypeFSEvents, UpdatedAt: now.UnixMilli(), FSEventID: 88}),
		FeedState:      RootFeedStateReady,
		LastFullScanAt: lastFullScanAt,
		CreatedAt:      now.UnixMilli(),
		UpdatedAt:      now.UnixMilli(),
	}
	mustInsertRoot(t, ctx, db, root)

	if err := db.ReplaceRootEntries(ctx, root, []EntryRecord{
		makeTestEntryRecord(root, staleFilePath, false, 42, now),
	}, nil); err != nil {
		t.Fatalf("seed root entries for startup restore: %v", err)
	}

	scanner := NewScanner(db)
	scanner.changeFeed = newTestSnapshotChangeFeed(nil)

	scanner.startupRestore(ctx)

	results := searchSQLiteForTest(t, db, "stale", 10)
	if len(results) != 1 || results[0].Path != staleFilePath {
		t.Fatalf("expected startup restore to load persisted entry %q, got %#v", staleFilePath, results)
	}

	rootAfter, err := db.FindRootByID(ctx, root.ID)
	if err != nil {
		t.Fatalf("find root after startup restore: %v", err)
	}
	if rootAfter == nil {
		t.Fatalf("expected root after startup restore")
	}
	if rootAfter.LastFullScanAt != lastFullScanAt {
		t.Fatalf("expected startup restore to skip full scan and keep LastFullScanAt=%d, got %d", lastFullScanAt, rootAfter.LastFullScanAt)
	}
}

func TestScannerFullScanUsesGlobalRunProgress(t *testing.T) {
	db, ctx := openTestFileSearchDB(t)
	now := time.Now().UnixMilli()
	rootOnePath := filepath.Join(t.TempDir(), "root-global-progress-one")
	rootTwoPath := filepath.Join(t.TempDir(), "root-global-progress-two")

	mustWriteTestFile(t, filepath.Join(rootOnePath, "nested-a", "alpha.txt"), "alpha")
	mustWriteTestFile(t, filepath.Join(rootOnePath, "nested-b", "beta.txt"), "beta")
	mustWriteTestFile(t, filepath.Join(rootTwoPath, "gamma.txt"), "gamma")

	rootOne := RootRecord{ID: "root-global-progress-one", Path: rootOnePath, Kind: RootKindUser, Status: RootStatusIdle, CreatedAt: now, UpdatedAt: now}
	rootTwo := RootRecord{ID: "root-global-progress-two", Path: rootTwoPath, Kind: RootKindUser, Status: RootStatusIdle, CreatedAt: now, UpdatedAt: now}
	mustInsertRoot(t, ctx, db, rootOne)
	mustInsertRoot(t, ctx, db, rootTwo)

	scanner := NewScanner(db)
	scanner.plannerBudgetOverride = &splitBudget{
		LeafEntryBudget:     3,
		LeafWriteBudget:     3,
		LeafMemoryBudget:    1 << 20,
		DirectFileBatchSize: 1,
	}
	engine := &Engine{db: db, scanner: scanner}

	var (
		statusesMu sync.Mutex
		statuses   []StatusSnapshot
	)
	scanner.SetStateChangeHandler(func(changeCtx context.Context) {
		status, err := engine.GetStatus(changeCtx)
		if err != nil {
			t.Fatalf("get status during full scan: %v", err)
		}
		statusesMu.Lock()
		statuses = append(statuses, status)
		statusesMu.Unlock()
	})

	scanner.scanAllRoots(ctx)

	statusesMu.Lock()
	defer statusesMu.Unlock()

	lastProgress := int64(-1)
	sawExecutingProgress := false
	for _, status := range statuses {
		if status.RunProgressTotal <= 0 {
			continue
		}
		sawExecutingProgress = true
		if status.RunProgressCurrent < lastProgress {
			t.Fatalf("expected run progress to stay monotonic, got %d after %d", status.RunProgressCurrent, lastProgress)
		}
		lastProgress = status.RunProgressCurrent
	}
	if !sawExecutingProgress {
		t.Fatal("expected full scan to emit run progress snapshots")
	}
	if len(statuses) == 0 {
		t.Fatal("expected full scan to emit status snapshots")
	}
	lastStatus := statuses[len(statuses)-1]
	if lastStatus.RunProgressTotal <= 0 {
		t.Fatalf("expected final run progress total > 0, got %d", lastStatus.RunProgressTotal)
	}
	if lastStatus.RunProgressCurrent != lastStatus.RunProgressTotal {
		t.Fatalf("expected final run progress to complete, got %d/%d", lastStatus.RunProgressCurrent, lastStatus.RunProgressTotal)
	}
}

func TestScannerFullScanReportsStreamingRunStages(t *testing.T) {
	db, ctx := openTestFileSearchDB(t)
	now := time.Now().UnixMilli()
	rootPath := filepath.Join(t.TempDir(), "root-run-stages")

	mustWriteTestFile(t, filepath.Join(rootPath, "nested-a", "alpha.txt"), "alpha")
	mustWriteTestFile(t, filepath.Join(rootPath, "nested-b", "beta.txt"), "beta")

	root := RootRecord{ID: "root-run-stages", Path: rootPath, Kind: RootKindUser, Status: RootStatusIdle, CreatedAt: now, UpdatedAt: now}
	mustInsertRoot(t, ctx, db, root)

	scanner := NewScanner(db)
	scanner.plannerBudgetOverride = &splitBudget{
		LeafEntryBudget:     3,
		LeafWriteBudget:     3,
		LeafMemoryBudget:    1 << 20,
		DirectFileBatchSize: 1,
	}
	engine := &Engine{db: db, scanner: scanner}

	stageSeen := map[RunStage]bool{}
	sawPlannerActivityContext := false
	scanner.SetStateChangeHandler(func(changeCtx context.Context) {
		status, err := engine.GetStatus(changeCtx)
		if err != nil {
			t.Fatalf("get status during staged full scan: %v", err)
		}
		if status.ActiveStage != "" {
			stageSeen[status.ActiveStage] = true
		}
		if status.ActiveStage == RunStagePlanning {
			if status.ActiveProgressTotal <= 0 {
				t.Fatalf("expected planner stage to expose a stable denominator, got %d", status.ActiveProgressTotal)
			}
			if strings.TrimSpace(status.ActiveRootPath) == "" || strings.TrimSpace(status.ActiveScopePath) == "" {
				t.Fatalf("expected planner stage to expose active root and scope paths, got root=%q scope=%q", status.ActiveRootPath, status.ActiveScopePath)
			}
			sawPlannerActivityContext = true
		}
	})

	scanner.scanAllRoots(ctx)

	for _, stage := range []RunStage{RunStagePlanning, RunStageExecuting, RunStageFinalizing} {
		if !stageSeen[stage] {
			t.Fatalf("expected full scan to report stage %q, got %#v", stage, stageSeen)
		}
	}
	if !sawPlannerActivityContext {
		t.Fatal("expected planner stage to publish root/scope activity context")
	}
}

func TestScannerFullScanStreamsLargeRootWithoutChangingRootIdentity(t *testing.T) {
	db, ctx := openTestFileSearchDB(t)
	now := time.Now().UnixMilli()
	rootPath := filepath.Join(t.TempDir(), "root-split-identity")

	mustWriteTestFile(t, filepath.Join(rootPath, "nested-a", "alpha.txt"), "alpha")
	mustWriteTestFile(t, filepath.Join(rootPath, "nested-b", "beta.txt"), "beta")
	mustWriteTestFile(t, filepath.Join(rootPath, "nested-c", "gamma.txt"), "gamma")

	root := RootRecord{ID: "root-split-identity", Path: rootPath, Kind: RootKindUser, Status: RootStatusIdle, CreatedAt: now, UpdatedAt: now}
	mustInsertRoot(t, ctx, db, root)

	scanner := NewScanner(db)
	scanner.plannerBudgetOverride = &splitBudget{
		LeafEntryBudget:     3,
		LeafWriteBudget:     3,
		LeafMemoryBudget:    1 << 20,
		DirectFileBatchSize: 1,
	}
	engine := &Engine{db: db, scanner: scanner}

	scopeSet := map[string]struct{}{}
	scanner.SetStateChangeHandler(func(changeCtx context.Context) {
		status, err := engine.GetStatus(changeCtx)
		if err != nil {
			t.Fatalf("get status during split full scan: %v", err)
		}
		if status.ActiveStage != RunStageExecuting {
			return
		}
		if strings.TrimSpace(status.ActiveScopePath) == "" {
			return
		}
		scopeSet[filepath.Clean(status.ActiveScopePath)] = struct{}{}
	})

	scanner.scanAllRoots(ctx)

	rootsAfter, err := db.ListRoots(ctx)
	if err != nil {
		t.Fatalf("list roots after split full scan: %v", err)
	}
	if len(rootsAfter) != 1 {
		t.Fatalf("expected one persisted root after split full scan, got %d", len(rootsAfter))
	}
	if rootsAfter[0].ID != root.ID {
		t.Fatalf("expected persisted root identity %q, got %q", root.ID, rootsAfter[0].ID)
	}
	expectedScopes := []string{
		rootPath,
		filepath.Join(rootPath, "nested-a"),
		filepath.Join(rootPath, "nested-b"),
		filepath.Join(rootPath, "nested-c"),
	}
	for _, expectedScope := range expectedScopes {
		if _, ok := scopeSet[filepath.Clean(expectedScope)]; !ok {
			t.Fatalf("expected streaming scope %q, got %#v", expectedScope, scopeSet)
		}
	}
}

func TestScannerFullScanWideDirectFilesPrunesRemovedFiles(t *testing.T) {
	db, ctx := openTestFileSearchDB(t)
	now := time.Now().UnixMilli()
	rootPath := filepath.Join(t.TempDir(), "root-wide-direct-prune")
	alphaPath := filepath.Join(rootPath, "alpha.txt")
	betaPath := filepath.Join(rootPath, "beta.txt")

	mustWriteTestFile(t, alphaPath, "alpha")
	mustWriteTestFile(t, betaPath, "beta")

	root := RootRecord{ID: "root-wide-direct-prune", Path: rootPath, Kind: RootKindUser, Status: RootStatusIdle, CreatedAt: now, UpdatedAt: now}
	mustInsertRoot(t, ctx, db, root)

	scanner := NewScanner(db)
	scanner.plannerBudgetOverride = &splitBudget{
		LeafEntryBudget:     2,
		LeafWriteBudget:     2,
		LeafMemoryBudget:    1 << 20,
		DirectFileBatchSize: 1,
	}

	scanner.scanAllRoots(ctx)

	if err := os.Remove(betaPath); err != nil {
		t.Fatalf("remove beta file: %v", err)
	}

	scanner.scanAllRoots(ctx)

	entries, err := db.ListEntriesByRoot(ctx, root.ID)
	if err != nil {
		t.Fatalf("list entries after direct-files rescan: %v", err)
	}
	seen := map[string]struct{}{}
	for _, entry := range entries {
		seen[entry.Path] = struct{}{}
	}
	if _, ok := seen[alphaPath]; !ok {
		t.Fatalf("expected surviving direct file %q after rescan", alphaPath)
	}
	if _, ok := seen[betaPath]; ok {
		t.Fatalf("expected removed direct file %q to be pruned after rescan", betaPath)
	}
}

func TestScannerStartupRestoreReconcilesFallbackRoots(t *testing.T) {
	db, ctx := openTestFileSearchDB(t)
	now := time.Now()
	rootPath := filepath.Join(t.TempDir(), "root-startup-restore-fallback")
	staleFilePath := filepath.Join(rootPath, "stale.txt")
	actualFilePath := filepath.Join(rootPath, "actual.txt")

	mustMkdirAll(t, rootPath)
	mustWriteTestFile(t, actualFilePath, "actual")

	root := RootRecord{
		ID:             "root-startup-restore-fallback",
		Path:           rootPath,
		Kind:           RootKindUser,
		Status:         RootStatusIdle,
		FeedType:       RootFeedTypeFallback,
		FeedState:      RootFeedStateReady,
		LastFullScanAt: now.Add(-2 * time.Hour).UnixMilli(),
		CreatedAt:      now.UnixMilli(),
		UpdatedAt:      now.UnixMilli(),
	}
	mustInsertRoot(t, ctx, db, root)

	if err := db.ReplaceRootEntries(ctx, root, []EntryRecord{
		makeTestEntryRecord(root, staleFilePath, false, 12, now.Add(-time.Minute)),
	}, nil); err != nil {
		t.Fatalf("seed stale fallback entries: %v", err)
	}

	scanner := NewScanner(db)
	scanner.changeFeed = newTestSnapshotChangeFeed(nil)

	scanner.startupRestore(ctx)

	results := searchSQLiteForTest(t, db, "actual", 10)
	if len(results) != 1 || results[0].Path != actualFilePath {
		t.Fatalf("expected startup restore to reconcile fallback root to %q, got %#v", actualFilePath, results)
	}

	results = searchSQLiteForTest(t, db, "stale", 10)
	if len(results) != 0 {
		t.Fatalf("expected stale fallback entry %q to be removed after startup reconcile, got %#v", staleFilePath, results)
	}
}
