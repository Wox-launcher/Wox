package filesearch

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"wox/util/filesearch/column"
)

func TestSnapshotBuilderDoesNotDescendIntoSymlinkDirectoryCycle(t *testing.T) {
	rootPath := filepath.Join(t.TempDir(), "root-symlink-cycle")
	sourcePath := filepath.Join(rootPath, "source")
	targetFilePath := filepath.Join(sourcePath, "kept.txt")
	linkPath := filepath.Join(sourcePath, "loop")

	mustWriteTestFile(t, targetFilePath, "kept")
	if err := os.Symlink(sourcePath, linkPath); err != nil {
		t.Fatalf("create symlink cycle: %v", err)
	}

	root := RootRecord{ID: "root-symlink-cycle", Path: rootPath, Kind: RootKindUser}
	builder := NewSnapshotBuilder(nil)
	batch, err := builder.BuildSubtreeSnapshot(context.Background(), root, rootPath)
	if err != nil {
		t.Fatalf("build symlink cycle subtree snapshot: %v", err)
	}

	seenEntries := map[string]bool{}
	for _, entry := range batch.Entries {
		seenEntries[entry.Path] = true
	}
	if !seenEntries[targetFilePath] {
		t.Fatalf("expected real file to remain indexed, got %#v", seenEntries)
	}
	if seenEntries[filepath.Join(linkPath, "kept.txt")] || seenEntries[filepath.Join(linkPath, "loop")] {
		t.Fatalf("expected symlink directory not to be traversed recursively, got %#v", seenEntries)
	}
}

func TestSnapshotBuilderTreatsSymlinkScopeAsLinkEntry(t *testing.T) {
	rootPath := filepath.Join(t.TempDir(), "root-symlink-scope")
	targetPath := filepath.Join(t.TempDir(), "target")
	targetFilePath := filepath.Join(targetPath, "target-only.txt")
	linkPath := filepath.Join(rootPath, "linked-target")

	mustWriteTestFile(t, targetFilePath, "target")
	if err := os.MkdirAll(rootPath, 0o755); err != nil {
		t.Fatalf("mkdir root: %v", err)
	}
	if err := os.Symlink(targetPath, linkPath); err != nil {
		t.Fatalf("create symlink scope: %v", err)
	}

	root := RootRecord{ID: "root-symlink-scope", Path: rootPath, Kind: RootKindUser}
	builder := NewSnapshotBuilder(nil)
	batch, err := builder.BuildSubtreeSnapshot(context.Background(), root, linkPath)
	if err != nil {
		t.Fatalf("build symlink scope snapshot: %v", err)
	}

	seenEntries := map[string]EntryRecord{}
	for _, entry := range batch.Entries {
		seenEntries[entry.Path] = entry
	}
	if entry, ok := seenEntries[linkPath]; !ok || entry.IsDir {
		t.Fatalf("expected symlink scope itself as a non-directory entry, got ok=%v entry=%#v", ok, entry)
	}
	if _, ok := seenEntries[filepath.Join(linkPath, "target-only.txt")]; ok {
		t.Fatalf("expected symlink scope not to traverse target contents, got %#v", seenEntries)
	}
}

// TestSnapshotStreamRecordsMatchPathEntries pins the pre-resolved stream rows to
// the path-based conversion, so listing-derived parent IDs, names and metadata
// cannot drift from what incremental path entries write for the same tree.
func TestSnapshotStreamRecordsMatchPathEntries(t *testing.T) {
	rootPath := filepath.Join(t.TempDir(), "root-stream-records")
	mustWriteTestFile(t, filepath.Join(rootPath, "top.txt"), "top")
	mustWriteTestFile(t, filepath.Join(rootPath, "docs", "a.md"), "a")
	mustWriteTestFile(t, filepath.Join(rootPath, "docs", "deep", "b.md"), "bb")
	mustMkdirAll(t, filepath.Join(rootPath, "empty"))
	if err := os.Symlink(filepath.Join(rootPath, "docs"), filepath.Join(rootPath, "docs-link")); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	root := RootRecord{ID: "root-stream-records", Path: rootPath, Kind: RootKindUser}
	builder := NewSnapshotBuilder(nil)
	builder.SetDirectFileBatchSize(2)

	// Root: itself, top.txt, docs, a.md, deep, b.md, empty, docs-link.
	expectedCounts := map[string]int{rootPath: 8, filepath.Join(rootPath, "docs"): 4}
	for scope, expectedCount := range expectedCounts {
		want := map[uint64]column.Record{}
		batch, err := builder.BuildSubtreeSnapshot(context.Background(), root, scope)
		if err != nil {
			t.Fatalf("build path snapshot for %s: %v", scope, err)
		}
		for _, entry := range batch.Entries {
			record, err := directoryEntryRecord(root, entry)
			if err != nil {
				t.Fatal(err)
			}
			want[record.ID] = record
		}

		got := map[uint64]column.Record{}
		err = builder.StreamSubtreeJobBatches(context.Background(), root, Job{RootID: root.ID, Kind: JobKindSubtree, ScopePath: scope}, func(batch SubtreeSnapshotBatch) error {
			if len(batch.Entries) != 0 || len(batch.Directories) != 0 {
				t.Fatalf("stream batch should only carry resolved records")
			}
			for _, record := range batch.Records {
				got[record.ID] = record
			}
			return nil
		})
		if err != nil {
			t.Fatalf("stream %s: %v", scope, err)
		}
		if len(got) != expectedCount {
			t.Fatalf("scope %s: expected %d stream records, got %d", scope, expectedCount, len(got))
		}
		if !reflect.DeepEqual(want, got) {
			t.Fatalf("scope %s: stream records differ\nwant: %#v\ngot:  %#v", scope, want, got)
		}
	}
}
