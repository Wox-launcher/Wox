package filesearch

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultDynamicRootConfigUsesFiveChangePromotionThreshold(t *testing.T) {
	config := defaultDynamicRootConfig()
	if config.MinChangeCount != 5 {
		t.Fatalf("expected default dynamic root promotion threshold to be 5 changes, got %d", config.MinChangeCount)
	}
}

func TestRootRecordPersistsDynamicMetadata(t *testing.T) {
	db, ctx := openTestFileSearchDB(t)
	now := time.Now().UnixMilli()
	rootPath := filepath.Join(t.TempDir(), "root-dynamic-metadata")
	dynamicPath := filepath.Join(rootPath, "workspace", "target")

	mustMkdirAll(t, dynamicPath)
	mustInsertRoot(t, ctx, db, RootRecord{
		ID:        "root-dynamic-parent",
		Path:      rootPath,
		Kind:      RootKindUser,
		Status:    RootStatusIdle,
		CreatedAt: now,
		UpdatedAt: now,
	})

	dynamicRoot := RootRecord{
		ID:                  "root-dynamic-child",
		Path:                dynamicPath,
		Kind:                RootKindDynamic,
		Status:              RootStatusIdle,
		DynamicParentRootID: "root-dynamic-parent",
		PolicyRootPath:      rootPath,
		PromotedAt:          now - 1000,
		LastHotAt:           now,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	mustInsertRoot(t, ctx, db, dynamicRoot)

	loaded, err := db.FindRootByID(ctx, dynamicRoot.ID)
	if err != nil {
		t.Fatalf("find dynamic root: %v", err)
	}
	if loaded == nil {
		t.Fatal("expected dynamic root to be persisted")
	}
	if loaded.Kind != RootKindDynamic {
		t.Fatalf("expected dynamic kind, got %q", loaded.Kind)
	}
	if loaded.DynamicParentRootID != dynamicRoot.DynamicParentRootID ||
		loaded.PolicyRootPath != dynamicRoot.PolicyRootPath ||
		loaded.PromotedAt != dynamicRoot.PromotedAt ||
		loaded.LastHotAt != dynamicRoot.LastHotAt {
		t.Fatalf("unexpected dynamic metadata after reload: %#v", loaded)
	}
}

func TestDynamicRootOwnershipMovesScopedFacts(t *testing.T) {
	db, ctx := openTestFileSearchDB(t)
	now := time.Now()
	rootPath := filepath.Join(t.TempDir(), "root-dynamic-ownership")
	dynamicPath := filepath.Join(rootPath, "workspace", "target")
	dynamicFilePath := filepath.Join(dynamicPath, "owned.txt")

	mustWriteTestFile(t, dynamicFilePath, "owned")
	parentRoot := RootRecord{ID: "root-owner-parent", Path: rootPath, Kind: RootKindUser, Status: RootStatusIdle, CreatedAt: now.UnixMilli(), UpdatedAt: now.UnixMilli()}
	dynamicRoot := RootRecord{ID: "root-owner-dynamic", Path: dynamicPath, Kind: RootKindDynamic, Status: RootStatusIdle, DynamicParentRootID: parentRoot.ID, PolicyRootPath: rootPath, CreatedAt: now.UnixMilli(), UpdatedAt: now.UnixMilli()}
	mustInsertRoot(t, ctx, db, parentRoot)
	mustInsertRoot(t, ctx, db, dynamicRoot)

	if err := db.ReplaceRootEntries(ctx, parentRoot, []EntryRecord{
		makeTestEntryRecord(parentRoot, dynamicPath, true, 0, now),
		makeTestEntryRecord(parentRoot, dynamicFilePath, false, 5, now),
	}, nil); err != nil {
		t.Fatalf("seed parent-owned entries: %v", err)
	}

	if err := db.MoveScopedRowsToRoot(ctx, parentRoot.ID, dynamicRoot.ID, dynamicPath); err != nil {
		t.Fatalf("move scoped rows to dynamic root: %v", err)
	}

	assertEntryRoot(t, ctx, db, dynamicFilePath, dynamicRoot.ID)
}

func TestScannerRootReconcileDoesNotStealDynamicRootOwnership(t *testing.T) {
	db, ctx := openTestFileSearchDB(t)
	now := time.Now()
	rootPath := filepath.Join(t.TempDir(), "root-dynamic-reconcile")
	dynamicPath := filepath.Join(rootPath, "workspace", "target")
	parentFilePath := filepath.Join(rootPath, "parent.txt")
	dynamicFilePath := filepath.Join(dynamicPath, "owned.txt")

	mustWriteTestFile(t, parentFilePath, "parent")
	mustWriteTestFile(t, dynamicFilePath, "owned")
	parentRoot := RootRecord{ID: "root-reconcile-parent", Path: rootPath, Kind: RootKindUser, Status: RootStatusIdle, CreatedAt: now.UnixMilli(), UpdatedAt: now.UnixMilli()}
	dynamicRoot := RootRecord{ID: "root-reconcile-dynamic", Path: dynamicPath, Kind: RootKindDynamic, Status: RootStatusIdle, DynamicParentRootID: parentRoot.ID, PolicyRootPath: rootPath, CreatedAt: now.UnixMilli(), UpdatedAt: now.UnixMilli()}
	mustInsertRoot(t, ctx, db, parentRoot)
	mustInsertRoot(t, ctx, db, dynamicRoot)

	if err := db.ReplaceRootEntries(ctx, dynamicRoot, []EntryRecord{
		makeTestEntryRecord(dynamicRoot, dynamicPath, true, 0, now),
		makeTestEntryRecord(dynamicRoot, dynamicFilePath, false, 5, now),
	}, nil); err != nil {
		t.Fatalf("seed dynamic-owned entries: %v", err)
	}

	scanner := NewScanner(db)
	scanner.dirtyQueueConfig = DirtyQueueConfig{
		DebounceWindow:              defaultDirtyDebounceWindow,
		SiblingMergeThreshold:       8,
		RootEscalationPathThreshold: 1,
	}
	scanner.dirtyQueue = NewDirtyQueue(scanner.dirtyQueueConfig)
	scanner.enqueueDirty(DirtySignal{
		Kind:          DirtySignalKindRoot,
		RootID:        parentRoot.ID,
		Path:          parentRoot.Path,
		PathIsDir:     true,
		PathTypeKnown: true,
		At:            time.Now(),
	})

	if err := scanner.processDirtyQueue(ctx, time.Now().Add(2*defaultDirtyDebounceWindow)); err != nil {
		t.Fatalf("process parent root reconcile: %v", err)
	}

	assertEntryRoot(t, ctx, db, parentFilePath, parentRoot.ID)
	assertEntryRoot(t, ctx, db, dynamicFilePath, dynamicRoot.ID)
}

func TestScannerPromotesHotDirectoryAfterSuccessfulDirtyFlushes(t *testing.T) {
	db, ctx := openTestFileSearchDB(t)
	now := time.Now()
	rootPath := filepath.Join(t.TempDir(), "root-dynamic-promotion")
	hotPath := filepath.Join(rootPath, "workspace", "target")
	hotFilePath := filepath.Join(hotPath, "owned.txt")

	mustWriteTestFile(t, hotFilePath, "owned")
	parentRoot := RootRecord{ID: "root-promote-parent", Path: rootPath, Kind: RootKindUser, Status: RootStatusIdle, CreatedAt: now.UnixMilli(), UpdatedAt: now.UnixMilli()}
	mustInsertRoot(t, ctx, db, parentRoot)

	scanner := NewScanner(db)
	scanner.dynamicRootConfig = DynamicRootConfig{
		Enabled:                    true,
		Window:                     time.Minute,
		MinChangeCount:             2,
		MinFlushCount:              2,
		MinDepthBelowRoot:          2,
		IdleDemotionAfter:          24 * time.Hour,
		MaxDynamicRootsPerUserRoot: 16,
		MaxDynamicRootsGlobal:      64,
	}
	scanner.dirtyQueueConfig = DirtyQueueConfig{DebounceWindow: time.Millisecond}
	scanner.dirtyQueue = NewDirtyQueue(scanner.dirtyQueueConfig)

	for index := 0; index < 2; index++ {
		at := now.Add(time.Duration(index) * time.Second)
		scanner.handleChangeSignal(ctx, ChangeSignal{
			Kind:          ChangeSignalKindDirtyPath,
			RootID:        parentRoot.ID,
			Path:          hotFilePath,
			PathIsDir:     false,
			PathTypeKnown: true,
			At:            at,
		})
		if err := scanner.processDirtyQueue(ctx, at.Add(10*time.Millisecond)); err != nil {
			t.Fatalf("process dirty queue %d: %v", index, err)
		}
	}

	dynamicRoot, err := db.FindRootByPath(ctx, hotPath)
	if err != nil {
		t.Fatalf("find promoted dynamic root: %v", err)
	}
	if dynamicRoot == nil {
		t.Fatal("expected hot directory to be promoted")
	}
	if dynamicRoot.Kind != RootKindDynamic ||
		dynamicRoot.DynamicParentRootID != parentRoot.ID ||
		dynamicRoot.PolicyRootPath != parentRoot.Path {
		t.Fatalf("unexpected promoted root metadata: %#v", dynamicRoot)
	}
	assertEntryRoot(t, ctx, db, hotFilePath, dynamicRoot.ID)
}

func TestScannerDynamicRootCapsPreventPromotion(t *testing.T) {
	db, ctx := openTestFileSearchDB(t)
	now := time.Now()
	rootPath := filepath.Join(t.TempDir(), "root-dynamic-caps")
	existingDynamicPath := filepath.Join(rootPath, "workspace", "existing")
	hotPath := filepath.Join(rootPath, "workspace", "target")
	hotFilePath := filepath.Join(hotPath, "owned.txt")

	mustWriteTestFile(t, filepath.Join(existingDynamicPath, "seed.txt"), "seed")
	mustWriteTestFile(t, hotFilePath, "owned")
	parentRoot := RootRecord{ID: "root-caps-parent", Path: rootPath, Kind: RootKindUser, Status: RootStatusIdle, CreatedAt: now.UnixMilli(), UpdatedAt: now.UnixMilli()}
	existingDynamicRoot := RootRecord{ID: "root-caps-existing", Path: existingDynamicPath, Kind: RootKindDynamic, Status: RootStatusIdle, DynamicParentRootID: parentRoot.ID, PolicyRootPath: rootPath, CreatedAt: now.UnixMilli(), UpdatedAt: now.UnixMilli()}
	mustInsertRoot(t, ctx, db, parentRoot)
	mustInsertRoot(t, ctx, db, existingDynamicRoot)

	scanner := NewScanner(db)
	scanner.dynamicRootConfig = DynamicRootConfig{
		Enabled:                    true,
		Window:                     time.Minute,
		MinChangeCount:             1,
		MinFlushCount:              1,
		MinDepthBelowRoot:          2,
		IdleDemotionAfter:          24 * time.Hour,
		MaxDynamicRootsPerUserRoot: 1,
		MaxDynamicRootsGlobal:      64,
	}
	scanner.dirtyQueueConfig = DirtyQueueConfig{DebounceWindow: time.Millisecond}
	scanner.dirtyQueue = NewDirtyQueue(scanner.dirtyQueueConfig)

	scanner.handleChangeSignal(ctx, ChangeSignal{
		Kind:          ChangeSignalKindDirtyPath,
		RootID:        parentRoot.ID,
		Path:          hotFilePath,
		PathIsDir:     false,
		PathTypeKnown: true,
		At:            now,
	})
	if err := scanner.processDirtyQueue(ctx, now.Add(10*time.Millisecond)); err != nil {
		t.Fatalf("process capped dirty queue: %v", err)
	}

	blockedRoot, err := db.FindRootByPath(ctx, hotPath)
	if err != nil {
		t.Fatalf("find capped hot path: %v", err)
	}
	if blockedRoot != nil {
		t.Fatalf("expected per-parent cap to block promotion, got %#v", blockedRoot)
	}
}

func TestScannerDemotesIdleDynamicRootAndRestoresParentOwnership(t *testing.T) {
	db, ctx := openTestFileSearchDB(t)
	now := time.Now()
	rootPath := filepath.Join(t.TempDir(), "root-dynamic-demotion")
	dynamicPath := filepath.Join(rootPath, "workspace", "target")
	dynamicFilePath := filepath.Join(dynamicPath, "owned.txt")

	mustWriteTestFile(t, dynamicFilePath, "owned")
	parentRoot := RootRecord{ID: "root-demote-parent", Path: rootPath, Kind: RootKindUser, Status: RootStatusIdle, CreatedAt: now.UnixMilli(), UpdatedAt: now.UnixMilli()}
	dynamicRoot := RootRecord{
		ID:                  "root-demote-dynamic",
		Path:                dynamicPath,
		Kind:                RootKindDynamic,
		Status:              RootStatusIdle,
		DynamicParentRootID: parentRoot.ID,
		PolicyRootPath:      rootPath,
		PromotedAt:          now.Add(-25 * time.Hour).UnixMilli(),
		LastHotAt:           now.Add(-25 * time.Hour).UnixMilli(),
		CreatedAt:           now.UnixMilli(),
		UpdatedAt:           now.UnixMilli(),
	}
	mustInsertRoot(t, ctx, db, parentRoot)
	mustInsertRoot(t, ctx, db, dynamicRoot)
	if err := db.ReplaceRootEntries(ctx, dynamicRoot, []EntryRecord{
		makeTestEntryRecord(dynamicRoot, dynamicPath, true, 0, now),
		makeTestEntryRecord(dynamicRoot, dynamicFilePath, false, 5, now),
	}, nil); err != nil {
		t.Fatalf("seed dynamic-owned entries: %v", err)
	}

	scanner := NewScanner(db)
	scanner.dynamicRootConfig = DynamicRootConfig{Enabled: true, IdleDemotionAfter: 24 * time.Hour}
	if err := scanner.demoteIdleDynamicRoots(ctx, now); err != nil {
		t.Fatalf("demote idle dynamic roots: %v", err)
	}

	demotedRoot, err := db.FindRootByID(ctx, dynamicRoot.ID)
	if err != nil {
		t.Fatalf("find demoted dynamic root: %v", err)
	}
	if demotedRoot != nil {
		t.Fatalf("expected dynamic root row to be deleted, got %#v", demotedRoot)
	}
	assertEntryRoot(t, ctx, db, dynamicFilePath, parentRoot.ID)
}

func assertEntryRoot(t *testing.T, ctx context.Context, db *FileSearchDB, path string, rootID string) {
	t.Helper()

	tx, err := db.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin assert entry tx: %v", err)
	}
	defer tx.Rollback()

	row, ok, err := selectStoredEntryByPathTx(ctx, tx, path)
	if err != nil {
		t.Fatalf("select entry %q: %v", path, err)
	}
	if !ok {
		t.Fatalf("expected entry %q", path)
	}
	if row.RootID != rootID {
		t.Fatalf("expected entry %q root %q, got %q", path, rootID, row.RootID)
	}
}
