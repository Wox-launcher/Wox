package filesearch

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"wox/util"
)

func openTestFileSearchDB(t *testing.T) (*FileSearchDB, context.Context) {
	t.Helper()

	// This helper mutates process-global location and environment state.
	// Do not call it from tests that use t.Parallel().
	testRoot, err := os.MkdirTemp("", "wox-filesearch-test-")
	if err != nil {
		t.Fatalf("create test root: %v", err)
	}
	t.Setenv(util.TestWoxDataDirEnv, filepath.Join(testRoot, "wox"))
	t.Setenv(util.TestUserDataDirEnv, filepath.Join(testRoot, "user"))
	ctx := context.Background()

	if err := util.GetLocation().Init(); err != nil {
		t.Fatalf("init test location: %v", err)
	}

	db, err := NewFileSearchDB(ctx)
	if err != nil {
		t.Fatalf("open filesearch db: %v", err)
	}

	t.Cleanup(func() {
		_ = db.Close()
		_ = os.RemoveAll(testRoot)
	})

	return db, ctx
}

func mustInsertRoot(t *testing.T, ctx context.Context, db *FileSearchDB, root RootRecord) {
	t.Helper()

	if err := db.UpsertRoot(ctx, root); err != nil {
		t.Fatalf("insert root: %v", err)
	}
}

func mustInsertEntrySnapshots(t *testing.T, ctx context.Context, db *FileSearchDB, entries ...EntryRecord) {
	t.Helper()

	tx, err := db.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin entry snapshot tx: %v", err)
	}
	defer tx.Rollback()

	for _, entry := range entries {
		// Tests used to seed only the fact rows. SQLite-first reconcile depends on
		// the derived FTS and bigram tables being in sync as well, so the shared
		// helper now writes both layers together.
		current, err := upsertEntryFactsTx(ctx, tx, buildStoredEntryRecord(entry))
		if err != nil {
			t.Fatalf("upsert entry snapshot %q: %v", entry.Path, err)
		}
		if err := insertEntrySearchArtifactsTx(ctx, tx, current); err != nil {
			t.Fatalf("insert entry search artifacts %q: %v", entry.Path, err)
		}
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("commit entry snapshot tx: %v", err)
	}
}

func searchSQLiteForTest(t *testing.T, db *FileSearchDB, raw string, limit int) []SearchResult {
	t.Helper()

	results, err := NewSQLiteSearchProvider(db).Search(context.Background(), SearchQuery{Raw: raw}, limit)
	if err != nil {
		t.Fatalf("search sqlite provider for %q: %v", raw, err)
	}
	return results
}
