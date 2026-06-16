package filesearch

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReconcilerSubtreeRefreshesOnlyRequestedScope(t *testing.T) {
	db, ctx := openTestFileSearchDB(t)
	now := time.Now().UnixMilli()
	rootPath := filepath.Join(t.TempDir(), "root-subtree-reconcile")
	scopePath := filepath.Join(rootPath, "scope")
	scopeFilePath := filepath.Join(scopePath, "new.txt")
	siblingDirPath := filepath.Join(rootPath, "sibling")
	siblingFilePath := filepath.Join(siblingDirPath, "keep.txt")
	outsideDirPath := filepath.Join(rootPath, "outside")

	mustMkdirAll(t, scopePath)
	mustMkdirAll(t, siblingDirPath)
	mustWriteTestFile(t, scopeFilePath, "new")
	mustWriteTestFile(t, siblingFilePath, "keep")

	root := RootRecord{
		ID:        "root-reconcile-subtree",
		Path:      rootPath,
		Kind:      RootKindUser,
		Status:    RootStatusIdle,
		CreatedAt: now,
		UpdatedAt: now,
	}
	mustInsertRoot(t, ctx, db, root)

	oldScopeFilePath := filepath.Join(scopePath, "old.txt")
	siblingStaleFilePath := filepath.Join(siblingDirPath, "stale.txt")
	mustInsertEntrySnapshots(t, ctx, db,
		EntryRecord{
			Path:           oldScopeFilePath,
			RootID:         root.ID,
			ParentPath:     scopePath,
			Name:           "old.txt",
			NormalizedName: "old.txt",
			NormalizedPath: "old.txt",
			IsDir:          false,
			Mtime:          int64(10),
			Size:           int64(1),
			UpdatedAt:      now,
		},
		EntryRecord{
			Path:           siblingStaleFilePath,
			RootID:         root.ID,
			ParentPath:     siblingDirPath,
			Name:           "stale.txt",
			NormalizedName: "stale.txt",
			NormalizedPath: "stale.txt",
			IsDir:          false,
			Mtime:          int64(20),
			Size:           int64(2),
			UpdatedAt:      now,
		},
	)

	if _, err := db.db.ExecContext(ctx, `
		INSERT INTO directories (path, root_id, parent_path, last_scan_time, "exists")
		VALUES (?, ?, ?, ?, ?),
		       (?, ?, ?, ?, ?),
		       (?, ?, ?, ?, ?)
	`, scopePath, root.ID, rootPath, now, false,
		siblingDirPath, root.ID, rootPath, now, true,
		outsideDirPath, root.ID, rootPath, now, false,
	); err != nil {
		t.Fatalf("insert directory snapshots: %v", err)
	}

	reconciler := NewReconciler(db, nil)
	result, err := reconciler.Reconcile(ctx, ReconcileBatch{
		RootID: root.ID,
		Mode:   ReconcileModeSubtree,
		Paths:  []string{scopePath},
	})
	if err != nil {
		t.Fatalf("reconcile subtree: %v", err)
	}

	if result.Mode != ReconcileModeSubtree {
		t.Fatalf("expected subtree mode result, got %s", result.Mode)
	}
	if !result.ReloadNeeded {
		t.Fatalf("expected subtree reconcile to request reload")
	}

	entryState, directoryState := snapshotRootState(t, db, ctx, root.ID)

	if _, ok := entryState[oldScopeFilePath]; ok {
		t.Fatalf("expected old scoped entry to be removed")
	}
	if _, ok := entryState[scopeFilePath]; !ok {
		t.Fatalf("expected new scoped entry to be written")
	}
	if _, ok := entryState[siblingStaleFilePath]; !ok {
		t.Fatalf("expected sibling entry outside scope to remain")
	}

	scopeDirectory, ok := directoryState[scopePath]
	if !ok || !scopeDirectory.Exists {
		t.Fatalf("expected scoped directory snapshot to be refreshed")
	}
	if _, ok := directoryState[siblingDirPath]; !ok {
		t.Fatalf("expected sibling directory snapshot to remain")
	}
	if _, ok := directoryState[outsideDirPath]; !ok {
		t.Fatalf("expected outside directory snapshot to remain")
	}

	directoryCount, err := db.CountDirectoriesByRoot(ctx, root.ID)
	if err != nil {
		t.Fatalf("count directories by root: %v", err)
	}
	if directoryCount != 2 {
		t.Fatalf("expected 2 live directory snapshots after subtree reconcile, got %d", directoryCount)
	}
}

func TestReconcilerRootRefreshesWholeRoot(t *testing.T) {
	db, ctx := openTestFileSearchDB(t)
	now := time.Now().UnixMilli()
	rootPath := filepath.Join(t.TempDir(), "root-full-reconcile")
	scopePath := filepath.Join(rootPath, "scope")
	scopeFilePath := filepath.Join(scopePath, "new.txt")
	siblingDirPath := filepath.Join(rootPath, "sibling")
	siblingFilePath := filepath.Join(siblingDirPath, "keep.txt")

	mustMkdirAll(t, scopePath)
	mustMkdirAll(t, siblingDirPath)
	mustWriteTestFile(t, scopeFilePath, "new")
	mustWriteTestFile(t, siblingFilePath, "keep")

	root := RootRecord{
		ID:        "root-reconcile-root",
		Path:      rootPath,
		Kind:      RootKindUser,
		Status:    RootStatusIdle,
		CreatedAt: now,
		UpdatedAt: now,
	}
	mustInsertRoot(t, ctx, db, root)

	oldScopeFilePath := filepath.Join(scopePath, "old.txt")
	siblingStaleFilePath := filepath.Join(siblingDirPath, "stale.txt")
	mustInsertEntrySnapshots(t, ctx, db,
		EntryRecord{
			Path:           oldScopeFilePath,
			RootID:         root.ID,
			ParentPath:     scopePath,
			Name:           "old.txt",
			NormalizedName: "old.txt",
			NormalizedPath: "old.txt",
			IsDir:          false,
			Mtime:          int64(10),
			Size:           int64(1),
			UpdatedAt:      now,
		},
		EntryRecord{
			Path:           siblingStaleFilePath,
			RootID:         root.ID,
			ParentPath:     siblingDirPath,
			Name:           "stale.txt",
			NormalizedName: "stale.txt",
			NormalizedPath: "stale.txt",
			IsDir:          false,
			Mtime:          int64(20),
			Size:           int64(2),
			UpdatedAt:      now,
		},
	)

	reconciler := NewReconciler(db, nil)
	result, err := reconciler.Reconcile(ctx, ReconcileBatch{
		RootID: root.ID,
		Mode:   ReconcileModeRoot,
	})
	if err != nil {
		t.Fatalf("reconcile root: %v", err)
	}

	if result.Mode != ReconcileModeRoot {
		t.Fatalf("expected root mode result, got %s", result.Mode)
	}
	if !result.ReloadNeeded {
		t.Fatalf("expected root reconcile to request reload")
	}

	entryState, _ := snapshotRootState(t, db, ctx, root.ID)

	if _, ok := entryState[oldScopeFilePath]; ok {
		t.Fatalf("expected stale scoped entry to be removed by root reconcile")
	}
	if _, ok := entryState[siblingStaleFilePath]; ok {
		t.Fatalf("expected stale sibling entry to be removed by root reconcile")
	}
	if _, ok := entryState[rootPath]; !ok {
		t.Fatalf("expected root entry to be present after root reconcile")
	}
	if _, ok := entryState[scopePath]; !ok {
		t.Fatalf("expected scoped directory entry to be present after root reconcile")
	}
	if _, ok := entryState[scopeFilePath]; !ok {
		t.Fatalf("expected scoped file entry to be present after root reconcile")
	}
	if _, ok := entryState[siblingDirPath]; !ok {
		t.Fatalf("expected sibling directory entry to be present after root reconcile")
	}
	if _, ok := entryState[siblingFilePath]; !ok {
		t.Fatalf("expected sibling file entry to be present after root reconcile")
	}

	directoryCount, err := db.CountDirectoriesByRoot(ctx, root.ID)
	if err != nil {
		t.Fatalf("count directories by root after root reconcile: %v", err)
	}
	if directoryCount != 3 {
		t.Fatalf("expected 3 live directory snapshots after root reconcile, got %d", directoryCount)
	}

	rootAfter, err := db.FindRootByID(ctx, root.ID)
	if err != nil {
		t.Fatalf("find root after root reconcile: %v", err)
	}
	if rootAfter == nil {
		t.Fatalf("expected root %q to exist after root reconcile", root.ID)
	}
	if rootAfter.LastReconcileAt <= 0 {
		t.Fatalf("expected root reconcile timestamp to be recorded, got %d", rootAfter.LastReconcileAt)
	}
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()

	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func mustWriteTestFile(t *testing.T, path string, content string) {
	t.Helper()

	mustMkdirAll(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file %s: %v", path, err)
	}
}
