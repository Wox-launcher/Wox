package filesearch

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func TestDirectoryIndexBuildReplayAndContentView(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	index := newDirectoryIndex(filepath.Join(dir, "columns"))
	root := RootRecord{ID: "test-root", Path: filepath.Join(dir, "fixture")}
	if err := index.attach(root); err != nil {
		t.Fatal(err)
	}
	entry := EntryRecord{RootID: root.ID, Path: filepath.Join(root.Path, "项目报告.txt"), Size: 128, Mtime: 42}
	stream := func(yield func(SubtreeSnapshotBatch) error) error {
		return yield(SubtreeSnapshotBatch{RootID: root.ID, ScopePath: root.Path, Entries: []EntryRecord{entry}})
	}
	if err := index.replaceRoot(ctx, root, stream); err != nil {
		t.Fatal(err)
	}
	if results, err := index.search(ctx, SearchQuery{Raw: "xm"}, 10); err != nil || len(results) != 1 || results[0].Path != entry.Path || results[0].Size != 128 {
		t.Fatalf("search: %+v %v", results, err)
	}
	failed := errors.New("scan interrupted")
	if err := index.replaceRoot(ctx, root, func(yield func(SubtreeSnapshotBatch) error) error { return failed }); !errors.Is(err, failed) {
		t.Fatal(err)
	}
	if results, err := index.search(ctx, SearchQuery{Raw: "xm"}, 10); err != nil || len(results) != 1 {
		t.Fatalf("failed scan lost old index: %+v %v", results, err)
	}
	entry.Mtime, entry.Size = 43, 256
	if err := index.applyEntries(ctx, root.ID, []EntryRecord{entry}, nil, "checkpoint"); err != nil {
		t.Fatal(err)
	}
	if err := index.close(); err != nil {
		t.Fatal(err)
	}
	index = newDirectoryIndex(filepath.Join(dir, "columns"))
	defer index.close()
	if err := index.attach(root); err != nil {
		t.Fatal(err)
	}
	var candidates []EntryRecord
	if err := index.visit(ctx, root.ID, func(entry EntryRecord) error { candidates = append(candidates, entry); return nil }); err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 || candidates[0].Mtime != 43 || candidates[0].Size != 256 || candidates[0].Path != entry.Path {
		t.Fatalf("content view: %+v", candidates)
	}
	if err := index.applyEntries(ctx, root.ID, nil, []string{entry.Path}, "deleted"); err != nil {
		t.Fatal(err)
	}
	if results, err := index.search(ctx, SearchQuery{Raw: "xm"}, 10); err != nil || len(results) != 0 {
		t.Fatalf("delete: %+v %v", results, err)
	}
	if err := index.detach(root.ID); err != nil {
		t.Fatal(err)
	}
}
