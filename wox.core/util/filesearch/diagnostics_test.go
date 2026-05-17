package filesearch

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestEngineDiagnosticsReportsRootsIndexAndDirtyQueue(t *testing.T) {
	db, ctx := openTestFileSearchDB(t)
	engine := &Engine{
		db:             db,
		searchProvider: NewSQLiteSearchProvider(db),
		scanner:        NewScanner(db),
	}

	now := time.Now()
	userRootPath := filepath.Join(t.TempDir(), "user")
	dynamicRootPath := filepath.Join(userRootPath, "hot")
	mustInsertRoot(t, ctx, db, RootRecord{
		ID:        "root-user",
		Path:      userRootPath,
		Kind:      RootKindUser,
		Status:    RootStatusIdle,
		FeedType:  RootFeedTypeFSEvents,
		FeedState: RootFeedStateReady,
		CreatedAt: now.UnixMilli(),
		UpdatedAt: now.UnixMilli(),
	})
	mustInsertRoot(t, ctx, db, RootRecord{
		ID:                  "root-dynamic",
		Path:                dynamicRootPath,
		Kind:                RootKindDynamic,
		Status:              RootStatusSyncing,
		FeedType:            RootFeedTypeFSEvents,
		FeedState:           RootFeedStateDegraded,
		DynamicParentRootID: "root-user",
		CreatedAt:           now.UnixMilli(),
		UpdatedAt:           now.UnixMilli(),
	})
	mustInsertEntrySnapshots(t, ctx, db, EntryRecord{
		Path:       filepath.Join(userRootPath, "alpha.txt"),
		RootID:     "root-user",
		ParentPath: userRootPath,
		Name:       "alpha.txt",
		IsDir:      false,
		Mtime:      now.UnixMilli(),
		Size:       12,
		UpdatedAt:  now.UnixMilli(),
	})
	engine.scanner.enqueueDirtyWithContext(ctx, DirtySignal{
		Kind:          DirtySignalKindPath,
		RootID:        "root-user",
		Path:          filepath.Join(userRootPath, "beta.txt"),
		PathTypeKnown: true,
		At:            now,
	})

	diagnostics, err := engine.GetDiagnostics(context.Background())
	if err != nil {
		t.Fatalf("get diagnostics: %v", err)
	}

	if diagnostics.RootCount != 2 || diagnostics.DynamicRootCount != 1 || diagnostics.UserVisibleRootCount != 1 {
		t.Fatalf("unexpected root counts: %#v", diagnostics)
	}
	if diagnostics.DirtyQueue.PendingRootCount != 1 || diagnostics.DirtyQueue.PendingPathCount != 1 {
		t.Fatalf("expected pending dirty queue counts, got %#v", diagnostics.DirtyQueue)
	}
	if diagnostics.Index.EntryCount != 1 || diagnostics.Index.FileCount != 1 {
		t.Fatalf("expected index snapshot counts, got %#v", diagnostics.Index)
	}
	if got := diagnostics.RootKindCounts[RootKindDynamic]; got != 1 {
		t.Fatalf("expected dynamic root count by kind, got %d", got)
	}
}
