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

	grouped := map[string][]EntryRecord{}
	for _, entry := range entries {
		grouped[entry.RootID] = append(grouped[entry.RootID], entry)
	}
	for rootID, added := range grouped {
		root, err := db.FindRootByID(ctx, rootID)
		if err != nil || root == nil {
			t.Fatalf("root %s missing: %v", rootID, err)
		}
		existing, err := db.ListEntriesByRoot(ctx, rootID)
		if err != nil {
			t.Fatal(err)
		}
		if err := db.ReplaceRootEntries(ctx, *root, append(existing, added...), nil); err != nil {
			t.Fatal(err)
		}
	}
}

func searchColumnsForTest(t *testing.T, db *FileSearchDB, raw string, limit int) []SearchResult {
	t.Helper()

	results, err := (&columnSearchProvider{db: db}).Search(context.Background(), SearchQuery{Raw: raw}, limit)
	if err != nil {
		t.Fatalf("search column provider for %q: %v", raw, err)
	}
	return results
}
