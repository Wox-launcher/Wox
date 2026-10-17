package filesearch

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode"
	"wox/util"
	"wox/util/filesearch/column"
	"wox/util/fuzzymatch"
)

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

// mustSeedDirectories stores live directory entries for a root.
func mustSeedDirectories(t *testing.T, ctx context.Context, db *FileSearchDB, rootID string, paths ...string) {
	t.Helper()

	entries := make([]EntryRecord, 0, len(paths))
	for _, path := range paths {
		entries = append(entries, EntryRecord{
			Path:       path,
			RootID:     rootID,
			ParentPath: filepath.Dir(path),
			Name:       filepath.Base(path),
			IsDir:      true,
		})
	}
	mustInsertEntrySnapshots(t, ctx, db, entries...)
}

// columnDirectoriesForTest returns every reachable directory row of a root,
// including structural rows, which is where scanned directory facts live.
func columnDirectoriesForTest(t *testing.T, db *FileSearchDB, rootID string) map[string]bool {
	t.Helper()

	directories := map[string]bool{}
	db.directory.mu.RLock()
	shard := db.directory.shards[rootID]
	db.directory.mu.RUnlock()
	if shard == nil {
		return directories
	}
	err := shard.store.Read(func(view column.View) error {
		return visitShardRecords(context.Background(), view, func(record column.Record) error {
			if record.Flags&column.Directory == 0 {
				return nil
			}
			if path, ok := view.Path(record.ID); ok {
				directories[path] = true
			}
			return nil
		})
	})
	if err != nil && !errors.Is(err, column.ErrNotReady) {
		t.Fatalf("list column directories for %q: %v", rootID, err)
	}
	return directories
}

func searchColumnsForTest(t *testing.T, db *FileSearchDB, raw string, limit int) []SearchResult {
	t.Helper()

	results, err := (&columnSearchProvider{db: db}).Search(context.Background(), SearchQuery{Raw: raw}, limit)
	if err != nil {
		t.Fatalf("search column provider for %q: %v", raw, err)
	}
	return results
}

func (s *Scanner) enqueueDirty(signal DirtySignal) {
	s.enqueueDirtyWithContext(context.Background(), signal)
}
func (s *Scanner) enqueueDirtyForPath(ctx context.Context, path string) bool {
	root, ok := s.findRootForPath(ctx, path)
	if !ok {
		return false
	}

	cleanPath := filepath.Clean(path)
	cleanRootPath := filepath.Clean(root.Path)
	// Bug fix: manual dirty routing must not be the one path that follows a
	// symlink-to-directory into a subtree scan. Use the same Lstat-first type
	// helper as watcher feeds so symlink entries stay direct file-like deltas.
	pathIsDir, pathTypeKnown := statPathType(cleanPath)

	kind := DirtySignalKindPath
	if cleanPath == cleanRootPath {
		kind = DirtySignalKindRoot
	} else if filepath.Dir(cleanPath) == cleanRootPath && !pathTypeKnown {
		// Manual dirty-path routing has no create/remove semantic attached. If a
		// missing direct child might have been a deleted directory, root scope is
		// the smallest safe boundary that can prune unknown recursive rows.
		kind = DirtySignalKindRoot
	}

	s.enqueueDirtyWithContext(ctx, DirtySignal{
		Kind:          kind,
		SemanticKind:  ChangeSemanticKindUnknown,
		RootID:        root.ID,
		Path:          cleanPath,
		PathIsDir:     pathIsDir,
		PathTypeKnown: pathTypeKnown,
	})
	return true
}
func (s *Scanner) findRootForPath(ctx context.Context, path string) (RootRecord, bool) {
	roots, err := s.db.ListRoots(ctx)
	if err != nil {
		util.GetLogger().Warn(ctx, "filesearch failed to resolve dirty root: "+err.Error())
		return RootRecord{}, false
	}

	return findRootForPathInRoots(roots, path)
}

func findRootForPathInRoots(roots []RootRecord, path string) (RootRecord, bool) {
	cleanPath := filepath.Clean(path)
	bestIndex := -1
	bestLength := -1
	for index, root := range roots {
		if !pathWithinScope(root.Path, cleanPath) {
			continue
		}
		if len(root.Path) <= bestLength {
			continue
		}
		bestIndex = index
		bestLength = len(root.Path)
	}

	if bestIndex < 0 {
		return RootRecord{}, false
	}

	return roots[bestIndex], true
}

// replaceRoot streams scanner batches into compact columns and publishes only a
// complete build. The old generation stays available if traversal is interrupted.
func (d *directoryIndex) replaceRoot(ctx context.Context, root RootRecord, stream func(func(SubtreeSnapshotBatch) error) error) error {
	d.mu.RLock()
	defer d.mu.RUnlock()
	shard := d.shards[root.ID]
	if d.closed || shard == nil {
		return errors.New("directory shard is not attached")
	}
	shard.mu.Lock()
	defer shard.mu.Unlock()
	revision := shard.store.Revision()
	builder := column.NewBuilder(root.Path, directoryEntryID(root.Path))
	if err := stream(func(batch SubtreeSnapshotBatch) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if batch.RootID != root.ID || !pathWithinScope(root.Path, batch.ScopePath) {
			return errors.New("snapshot batch belongs to another root")
		}
		for _, entry := range batch.Entries {
			record, err := directoryEntryRecord(root, entry)
			if err != nil {
				return err
			}
			if err := builder.Add(record); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	base, err := builder.Finish()
	if err != nil {
		return err
	}
	return shard.store.Publish(base, root.FeedCursor, "configured-root-v1", revision)
}

// writeScope consumes bounded scanner batches. Full runs append compact rows;
// incremental scope replacement publishes atomically, including stale deletion.
func (d *directoryIndex) writeScope(ctx context.Context, rootID string, scope string, directFiles bool, stream func(func(SubtreeSnapshotBatch) error) error) error {
	return d.writeScopes(ctx, rootID, []string{scope}, directFiles, stream)
}

func buildPinyinFields(input string) (string, string) {
	if !containsHanRune(input) {
		// Optimization: ASCII-only file names are already indexed by normalized
		// name, name_key, and path FTS. Treating every ASCII name as "pinyin"
		// filled two extra FTS tables for tens of thousands of code files without
		// helping Chinese pinyin recall, so only names that actually contain Han
		// characters get pinyin payloads. Mixed names still keep their ASCII
		// letters below so queries like "xiangmu2" can match "项目2".
		return "", ""
	}

	var full strings.Builder
	var initials strings.Builder

	for _, r := range input {
		switch {
		case unicode.Is(unicode.Han, r):
			pinyins, ok := fuzzymatch.LookupCharPinyin(r)
			if !ok || len(pinyins) == 0 {
				continue
			}

			pinyin := strings.ToLower(strings.TrimSpace(pinyins[0]))
			if pinyin == "" {
				continue
			}

			full.WriteString(pinyin)
			initials.WriteByte(pinyin[0])
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			lower := strings.ToLower(string(r))
			full.WriteString(lower)
			initials.WriteString(lower)
		}
	}

	return full.String(), initials.String()
}

func containsHanRune(input string) bool {
	for _, r := range input {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}
