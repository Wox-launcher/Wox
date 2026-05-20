package filesearch

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestSnapshotBuilderExcludesDynamicChildEntriesAndTraversal(t *testing.T) {
	rootPath := filepath.Join(t.TempDir(), "root-snapshot-exclusion")
	regularPath := filepath.Join(rootPath, "regular")
	dynamicPath := filepath.Join(rootPath, "workspace", "target")
	dynamicFilePath := filepath.Join(dynamicPath, "owned.txt")
	regularFilePath := filepath.Join(regularPath, "kept.txt")

	mustWriteTestFile(t, dynamicFilePath, "owned")
	mustWriteTestFile(t, regularFilePath, "kept")

	root := RootRecord{ID: "root-snapshot-parent", Path: rootPath, Kind: RootKindUser}
	builder := NewSnapshotBuilder(nil)
	builder.SetRootExclusions(map[string][]string{
		root.ID: []string{dynamicPath},
	})

	batch, err := builder.BuildSubtreeSnapshot(context.Background(), root, rootPath)
	if err != nil {
		t.Fatalf("build excluded subtree snapshot: %v", err)
	}

	seenEntries := map[string]bool{}
	for _, entry := range batch.Entries {
		seenEntries[entry.Path] = true
	}
	if !seenEntries[rootPath] || !seenEntries[regularPath] || !seenEntries[regularFilePath] {
		t.Fatalf("expected non-excluded paths to remain indexed, got %#v", seenEntries)
	}
	if seenEntries[dynamicPath] || seenEntries[dynamicFilePath] {
		t.Fatalf("expected dynamic child directory and descendants to be excluded, got %#v", seenEntries)
	}

	for _, directory := range batch.Directories {
		if directory.Path == dynamicPath {
			t.Fatalf("expected dynamic child directory row to be excluded")
		}
	}
}

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

func TestSnapshotBuilderExcludesDynamicChildFromDirectFiles(t *testing.T) {
	rootPath := filepath.Join(t.TempDir(), "root-direct-exclusion")
	dynamicPath := filepath.Join(rootPath, "target")
	dynamicFilePath := filepath.Join(dynamicPath, "owned.txt")
	directFilePath := filepath.Join(rootPath, "kept.txt")

	mustWriteTestFile(t, dynamicFilePath, "owned")
	mustWriteTestFile(t, directFilePath, "kept")

	root := RootRecord{ID: "root-direct-parent", Path: rootPath, Kind: RootKindUser}
	builder := NewSnapshotBuilder(nil)
	builder.SetRootExclusions(map[string][]string{
		root.ID: []string{dynamicPath},
	})

	batch, err := builder.BuildDirectFilesJobSnapshot(context.Background(), root, Job{
		RootID:    root.ID,
		RootPath:  root.Path,
		ScopePath: root.Path,
		Kind:      JobKindDirectFiles,
	})
	if err != nil {
		t.Fatalf("build direct-files snapshot: %v", err)
	}

	seenEntries := map[string]bool{}
	for _, entry := range batch.Entries {
		seenEntries[entry.Path] = true
	}
	if !seenEntries[rootPath] || !seenEntries[directFilePath] {
		t.Fatalf("expected direct file scope and file, got %#v", seenEntries)
	}
	if seenEntries[dynamicPath] || seenEntries[dynamicFilePath] {
		t.Fatalf("expected dynamic child to be excluded from direct files snapshot, got %#v", seenEntries)
	}
}
