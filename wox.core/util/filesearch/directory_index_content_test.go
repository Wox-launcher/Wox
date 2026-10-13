package filesearch

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
)

// TestDirectoryIndexContentKeyset covers normalized extensions, overlay IDs,
// separate shards and compaction between pages without duplicate candidates.
func TestDirectoryIndexContentKeyset(t *testing.T) {
	ctx := context.Background()
	index := newDirectoryIndex(filepath.Join(t.TempDir(), "index"))
	defer index.close()
	want := map[string]bool{}
	for _, id := range []string{"first", "second"} {
		root := RootRecord{ID: id, Path: filepath.Join(t.TempDir(), id)}
		if err := index.attach(root); err != nil {
			t.Fatal(err)
		}
		var entries []EntryRecord
		for i := 0; i < 21; i++ {
			path := filepath.Join(root.Path, fmt.Sprintf("doc-%02d.TXT", i))
			entries = append(entries, EntryRecord{RootID: id, Path: path, ParentPath: root.Path, Size: 42, Mtime: 100})
			want[path] = true
		}
		entries = append(entries, EntryRecord{RootID: id, Path: filepath.Join(root.Path, "excluded.bin"), ParentPath: root.Path})
		if err := index.replaceRoot(ctx, root, func(yield func(SubtreeSnapshotBatch) error) error {
			return yield(SubtreeSnapshotBatch{RootID: id, ScopePath: root.Path, Entries: entries})
		}); err != nil {
			t.Fatal(err)
		}
		newEntry := entries[0]
		newEntry.Path = filepath.Join(root.Path, "added.txt")
		if err := index.applyEntries(ctx, id, []EntryRecord{newEntry}, []string{entries[1].Path}, "cursor"); err != nil {
			t.Fatal(err)
		}
		want[newEntry.Path] = true
		delete(want, entries[1].Path)
	}
	seen := map[string]bool{}
	var cursor ContentCandidateCursor
	for page := 0; page < 100; page++ {
		candidates, err := index.contentCandidates(ctx, cursor, map[string]bool{"txt": true}, 3)
		if err != nil {
			t.Fatal(err)
		}
		if len(candidates) == 0 {
			break
		}
		for _, candidate := range candidates {
			if seen[candidate.Path] || !want[candidate.Path] || candidate.Size != 42 || candidate.Mtime != 100 || !candidate.MetadataKnown || candidate.Extension != "txt" {
				t.Fatalf("bad candidate: %+v", candidate)
			}
			seen[candidate.Path] = true
			cursor = candidate.Cursor
		}
		if page == 1 {
			for _, shard := range index.shards {
				if err := shard.store.Compact(ctx); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	if len(seen) != len(want) {
		t.Fatalf("got %d candidates, want %d", len(seen), len(want))
	}
}
