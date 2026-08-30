//go:build windows

package filesearchservice

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestSnapshotVolumeRootsListsPublishedDrives(t *testing.T) {
	roots := snapshotVolumeRoots(&columnSnapshot{volumes: []*volumeColumn{
		{root: `C:\`},
		nil,
		{root: `D:\`},
	}})
	if len(roots) != 2 || roots[0] != `C:\` || roots[1] != `D:\` {
		t.Fatalf("snapshot volumes = %v, want C:\\ and D:\\", roots)
	}
}

func TestVolumeColumnSearchMaterializesOnlyMatchingPaths(t *testing.T) {
	volume := testVolumeColumn(t)
	results, err := volume.search(context.Background(), "notes", 10, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Path != `C:\Users\notes.txt` {
		t.Fatalf("results=%+v", results)
	}

	pinyinResults, err := volume.search(context.Background(), "xiangmu", 10, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(pinyinResults) != 1 || pinyinResults[0].Path != `C:\Users\项目.txt` {
		t.Fatalf("pinyin results=%+v", pinyinResults)
	}
}

func TestVolumeColumnDeltaRenamesDirectoryWithoutRewritingChildren(t *testing.T) {
	volume := testVolumeColumn(t)
	volume.delta[10] = deltaNode{parentReference: 5, name: "People", isDir: true}

	results, err := volume.search(context.Background(), "notes", 10, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Path != `C:\People\notes.txt` {
		t.Fatalf("results=%+v", results)
	}

	volume.delta[11] = deltaNode{deleted: true}
	results, err = volume.search(context.Background(), "notes", 10, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Fatalf("deleted results=%+v", results)
	}
}

func TestColumnSnapshotCountsIncludeUSNDeltas(t *testing.T) {
	volume := testVolumeColumn(t)
	snapshot := &columnSnapshot{volumes: []*volumeColumn{volume}}
	entryCount, fileCount := snapshot.counts()
	if entryCount != 4 || fileCount != 2 {
		t.Fatalf("base counts = %d/%d", entryCount, fileCount)
	}

	volume.deltaMu.Lock()
	volume.applyDelta(13, deltaNode{parentReference: 10, name: "new.txt"})
	volume.applyDelta(11, deltaNode{deleted: true})
	volume.applyDelta(10, deltaNode{deleted: true})
	volume.applyDelta(10, deltaNode{parentReference: 5, name: "People", isDir: true})
	volume.deltaMu.Unlock()

	entryCount, fileCount = snapshot.counts()
	if entryCount != 4 || fileCount != 2 {
		t.Fatalf("delta counts = %d/%d", entryCount, fileCount)
	}
}

func TestMappedVolumeColumnPreservesSearch(t *testing.T) {
	volume := testVolumeColumn(t)
	mapped, err := writeMappedVolumeColumn(filepath.Join(t.TempDir(), "volume.columns"), volume)
	if err != nil {
		t.Fatal(err)
	}
	defer mapped.storage.close()

	results, err := mapped.search(context.Background(), "notes", 10, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Path != `C:\Users\notes.txt` {
		t.Fatalf("results=%+v", results)
	}
}

func TestColumnIndexPauseReleasesPinnedSnapshotFiles(t *testing.T) {
	root := t.TempDir()
	handle, err := openDirectoryHandle(root)
	if err != nil {
		t.Fatal(err)
	}
	indexDir := &secureIndexDirectory{path: root, handle: handle}

	snapshotDir, err := indexDir.createSnapshotDirectory()
	if err != nil {
		t.Fatal(err)
	}
	file, err := snapshotDir.createFile("volume-0.columns")
	if err != nil {
		t.Fatal(err)
	}
	mapped, err := writeMappedVolumeColumnFile(file, testVolumeColumn(t), "")
	if err != nil {
		file.Close()
		t.Fatal(err)
	}
	index := &ColumnIndex{indexDir: indexDir}
	index.snapshot.Store(&columnSnapshot{volumes: []*volumeColumn{mapped}, storage: snapshotDir})
	if err := index.Pause(context.Background()); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("snapshot files remain after close: %v", entries)
	}
}

func testVolumeColumn(t *testing.T) *volumeColumn {
	t.Helper()
	builder := newVolumeBuilder(`C:\`)
	builder.add(mftNode{Reference: 5, ParentReference: 5, IsDir: true})
	builder.add(mftNode{Reference: 10, ParentReference: 5, Name: "Users", IsDir: true})
	builder.add(mftNode{Reference: 11, ParentReference: 10, Name: "notes.txt"})
	builder.add(mftNode{Reference: 12, ParentReference: 10, Name: "项目.txt"})
	volume, err := builder.finish(usnJournalData{JournalID: 1, NextUSN: 100})
	if err != nil {
		t.Fatal(err)
	}
	return volume
}
