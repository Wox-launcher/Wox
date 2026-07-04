package filesearch

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func TestContentIndexBasicFlow(t *testing.T) {
	db := newTestContentSearchDB(t)
	defer db.Close()
	ctx := context.Background()

	// Index a file.
	updated, err := db.IndexContent(ctx, "/test/readme.txt", 1000000, 500, "txt", "hello world readme content")
	if err != nil {
		t.Fatalf("IndexContent: %v", err)
	}
	if !updated {
		t.Error("expected updated=true for new file")
	}

	// Search for "hello" — should find it.
	results, err := db.SearchContent(ctx, "hello", 10)
	if err != nil {
		t.Fatalf("SearchContent: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("no results for 'hello'")
	}
	if results[0].Path != "/test/readme.txt" {
		t.Errorf("path: %q, want /test/readme.txt", results[0].Path)
	}

	// Search for "readme content" — multi-term AND.
	results, err = db.SearchContent(ctx, "readme content", 10)
	if err != nil {
		t.Fatalf("SearchContent multi: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("no results for 'readme content'")
	}
	if results[0].Path != "/test/readme.txt" {
		t.Errorf("path: %q, want /test/readme.txt", results[0].Path)
	}
}

func TestContentIndexSameHashSkips(t *testing.T) {
	db := newTestContentSearchDB(t)
	defer db.Close()
	ctx := context.Background()

	// Index a file.
	_, err := db.IndexContent(ctx, "/test/file.txt", 1000, 100, "txt", "hello world")
	if err != nil {
		t.Fatalf("IndexContent: %v", err)
	}

	// Index again with same content — should skip.
	updated, err := db.IndexContent(ctx, "/test/file.txt", 1000, 100, "txt", "hello world")
	if err != nil {
		t.Fatalf("IndexContent again: %v", err)
	}
	if updated {
		t.Error("expected updated=false for same content")
	}
}

func TestContentIndexModifyReplaces(t *testing.T) {
	db := newTestContentSearchDB(t)
	defer db.Close()
	ctx := context.Background()

	// Index with initial content.
	_, err := db.IndexContent(ctx, "/test/file.txt", 1000, 100, "txt", "old content here")
	if err != nil {
		t.Fatalf("IndexContent: %v", err)
	}

	// Verify old content is searchable.
	results, _ := db.SearchContent(ctx, "old content", 10)
	if len(results) == 0 {
		t.Fatal("old content should be searchable")
	}

	// Index with new content.
	updated, err := db.IndexContent(ctx, "/test/file.txt", 2000, 200, "txt", "completely new text different words")
	if err != nil {
		t.Fatalf("IndexContent modify: %v", err)
	}
	if !updated {
		t.Error("expected updated=true for modified content")
	}

	// New content should be searchable.
	results, _ = db.SearchContent(ctx, "completely different", 10)
	if len(results) == 0 {
		t.Fatal("new content should be searchable")
	}
	if results[0].Path != "/test/file.txt" {
		t.Errorf("path: %q", results[0].Path)
	}

	// Old content should NOT be searchable.
	results, _ = db.SearchContent(ctx, "old content here", 10)
	for _, r := range results {
		if r.Path == "/test/file.txt" {
			t.Error("old content should not match after modify")
		}
	}
}

func TestContentIndexDelete(t *testing.T) {
	db := newTestContentSearchDB(t)
	defer db.Close()
	ctx := context.Background()

	_, err := db.IndexContent(ctx, "/test/file.txt", 1000, 100, "txt", "unique searchable content")
	if err != nil {
		t.Fatalf("IndexContent: %v", err)
	}

	// Verify searchable.
	results, _ := db.SearchContent(ctx, "unique searchable", 10)
	if len(results) == 0 {
		t.Fatal("should be searchable before delete")
	}

	// Delete.
	if err := db.DeleteContent(ctx, "/test/file.txt"); err != nil {
		t.Fatalf("DeleteContent: %v", err)
	}

	// Should no longer be searchable.
	results, _ = db.SearchContent(ctx, "unique searchable", 10)
	for _, r := range results {
		if r.Path == "/test/file.txt" {
			t.Error("deleted file should not be searchable")
		}
	}
}

func TestContentIndexReset(t *testing.T) {
	db := newTestContentSearchDB(t)
	defer db.Close()
	ctx := context.Background()

	_, err := db.IndexContent(ctx, "/test/a.txt", 1000, 100, "txt", "hello world")
	if err != nil {
		t.Fatalf("IndexContent a: %v", err)
	}
	_, err = db.IndexContent(ctx, "/test/b.txt", 2000, 200, "txt", "foo bar baz")
	if err != nil {
		t.Fatalf("IndexContent b: %v", err)
	}

	stats, _ := db.ContentStats(ctx)
	if stats.DocCount != 2 {
		t.Errorf("before reset: DocCount=%d, want 2", stats.DocCount)
	}

	// Reset.
	if err := db.ResetContentIndex(ctx); err != nil {
		t.Fatalf("ResetContentIndex: %v", err)
	}

	stats, _ = db.ContentStats(ctx)
	if stats.DocCount != 0 {
		t.Errorf("after reset: DocCount=%d, want 0", stats.DocCount)
	}

	// Search should return nothing.
	results, _ := db.SearchContent(ctx, "hello", 10)
	if len(results) != 0 {
		t.Errorf("after reset: got %d results, want 0", len(results))
	}
}

func TestContentIndexCJK(t *testing.T) {
	db := newTestContentSearchDB(t)
	defer db.Close()
	ctx := context.Background()

	_, err := db.IndexContent(ctx, "/test/notes.md", 1000, 100, "md", "区块链技术 content search test")
	if err != nil {
		t.Fatalf("IndexContent: %v", err)
	}

	// Search for "区块链" → bigrams 区块, 块链.
	results, err := db.SearchContent(ctx, "区块链", 10)
	if err != nil {
		t.Fatalf("SearchContent CJK: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("no results for CJK search")
	}
	if results[0].Path != "/test/notes.md" {
		t.Errorf("path: %q, want /test/notes.md", results[0].Path)
	}
}

func TestContentIndexCamelCase(t *testing.T) {
	db := newTestContentSearchDB(t)
	defer db.Close()
	ctx := context.Background()

	_, err := db.IndexContent(ctx, "/test/main.go", 1000, 100, "go", "package main func getUserById")
	if err != nil {
		t.Fatalf("IndexContent: %v", err)
	}

	// "getUser" tokenizes to "get user" — both should match.
	results, _ := db.SearchContent(ctx, "getUser", 10)
	if len(results) == 0 {
		t.Fatal("no results for 'getUser'")
	}
	if results[0].Path != "/test/main.go" {
		t.Errorf("path: %q, want /test/main.go", results[0].Path)
	}
}

func TestContentIndexStats(t *testing.T) {
	db := newTestContentSearchDB(t)
	defer db.Close()
	ctx := context.Background()

	db.IndexContent(ctx, "/test/a.txt", 1000, 100, "txt", "hello world content")
	db.IndexContent(ctx, "/test/b.go", 2000, 200, "go", "package main func")

	stats, err := db.ContentStats(ctx)
	if err != nil {
		t.Fatalf("ContentStats: %v", err)
	}
	if stats.DocCount != 2 {
		t.Errorf("DocCount: %d, want 2", stats.DocCount)
	}
	if stats.IndexedTextBytes <= 0 {
		t.Errorf("IndexedTextBytes: %d, want > 0", stats.IndexedTextBytes)
	}
}

func TestContentCrawlState(t *testing.T) {
	db := newTestContentSearchDB(t)
	defer db.Close()
	ctx := context.Background()

	// Default state should be empty (not crawled).
	state, err := db.GetContentCrawlState(ctx)
	if err != nil {
		t.Fatalf("GetContentCrawlState: %v", err)
	}
	if state != "" {
		t.Errorf("initial state: %q, want empty", state)
	}

	// Set to in_progress.
	if err := db.SetContentCrawlState(ctx, "in_progress"); err != nil {
		t.Fatalf("SetContentCrawlState: %v", err)
	}
	state, _ = db.GetContentCrawlState(ctx)
	if state != "in_progress" {
		t.Errorf("state: %q, want in_progress", state)
	}

	// Set to complete.
	if err := db.SetContentCrawlState(ctx, "complete"); err != nil {
		t.Fatalf("SetContentCrawlState complete: %v", err)
	}
	state, _ = db.GetContentCrawlState(ctx)
	if state != "complete" {
		t.Errorf("state: %q, want complete", state)
	}
}

func TestContentIndexNonExistentDelete(t *testing.T) {
	db := newTestContentSearchDB(t)
	defer db.Close()
	ctx := context.Background()

	// Delete non-existent path should not error.
	if err := db.DeleteContent(ctx, "/nonexistent"); err != nil {
		t.Errorf("delete non-existent: %v", err)
	}
}

func TestContentIndexListPaths(t *testing.T) {
	db := newTestContentSearchDB(t)
	defer db.Close()
	ctx := context.Background()

	db.IndexContent(ctx, "/test/b.txt", 2000, 200, "txt", "foo")
	db.IndexContent(ctx, "/test/a.txt", 1000, 100, "txt", "hello")

	paths, err := db.ListContentEntryPaths(ctx)
	if err != nil {
		t.Fatalf("ListContentEntryPaths: %v", err)
	}
	if len(paths) != 2 {
		t.Fatalf("paths: %d, want 2", len(paths))
	}
	// Should be sorted.
	if paths[0] != "/test/a.txt" || paths[1] != "/test/b.txt" {
		t.Errorf("paths: %v, want sorted [a.txt, b.txt]", paths)
	}
}

// newTestContentSearchDB creates a fresh ContentSearchDB in a temp directory for testing.
func newTestContentSearchDB(t *testing.T) *ContentSearchDB {
	t.Helper()
	dir := t.TempDir()
	dbPath := dir + string(rune(filepath.Separator)) + "test.db"
	dsn := dbPath + "?_journal_mode=WAL&_synchronous=NORMAL&_cache_size=2000&_foreign_keys=true&_busy_timeout=5000"
	sqlDB, err := sql.Open("sqlite3", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	db := &ContentSearchDB{db: sqlDB, dbPath: dbPath}
	if err := db.initTables(context.Background()); err != nil {
		t.Fatalf("initTables: %v", err)
	}
	return db
}
