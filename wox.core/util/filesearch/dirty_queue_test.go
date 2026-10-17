package filesearch

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

func TestDirtyQueueFlushReadyKeepsDisjointSubtreesSeparate(t *testing.T) {
	queue := NewDirtyQueue(DirtyQueueConfig{
		DebounceWindow: 50 * time.Millisecond,
	})

	firstDir := filepath.Join(string(filepath.Separator), "root", "a", "b", "c")
	secondDir := filepath.Join(string(filepath.Separator), "root", "a", "d", "e")

	queue.Push(DirtySignal{Kind: DirtySignalKindPath, RootID: "root-a", Path: firstDir, PathTypeKnown: true, PathIsDir: true, At: time.Unix(0, 0)})
	queue.Push(DirtySignal{Kind: DirtySignalKindPath, RootID: "root-a", Path: secondDir, PathTypeKnown: true, PathIsDir: true, At: time.Unix(0, 0)})

	batches := queue.FlushReady(time.Unix(0, int64(60*time.Millisecond)))
	if len(batches) != 1 {
		t.Fatalf("expected one root batch, got %d", len(batches))
	}
	if batches[0].Mode != ReconcileModeSubtree {
		t.Fatalf("expected subtree reconcile, got %s", batches[0].Mode)
	}
	if batches[0].RootID != "root-a" {
		t.Fatalf("expected root-a batch, got %q", batches[0].RootID)
	}
	if batches[0].DirtyPathCount != 2 {
		t.Fatalf("expected 2 dirty paths, got %d", batches[0].DirtyPathCount)
	}
	expectedFirst := filepath.Join(string(filepath.Separator), "root", "a", "b", "c")
	expectedSecond := filepath.Join(string(filepath.Separator), "root", "a", "d", "e")
	if len(batches[0].Paths) != 2 || batches[0].Paths[0] != expectedFirst || batches[0].Paths[1] != expectedSecond {
		t.Fatalf("unexpected subtree paths: %#v", batches[0].Paths)
	}
}

func TestDirtyQueuePendingCountsStayExactAcrossFlush(t *testing.T) {
	queue := NewDirtyQueue(DirtyQueueConfig{
		DebounceWindow: 0,
	})
	firstPath := filepath.Join(string(filepath.Separator), "root", "same.txt")
	secondPath := filepath.Join(string(filepath.Separator), "root", "other.txt")

	queue.Push(DirtySignal{Kind: DirtySignalKindPath, RootID: "root-a", Path: firstPath, PathTypeKnown: true, At: time.Unix(0, 0)})
	queue.Push(DirtySignal{Kind: DirtySignalKindPath, RootID: "root-a", Path: firstPath, PathTypeKnown: true, At: time.Unix(0, 0)})
	queue.Push(DirtySignal{Kind: DirtySignalKindPath, RootID: "root-b", Path: secondPath, PathTypeKnown: true, At: time.Unix(0, 0)})

	if rootCount, pathCount := queue.PendingCounts(); rootCount != 2 || pathCount != 2 {
		t.Fatalf("expected pending counts roots=2 paths=2 before flush, got roots=%d paths=%d", rootCount, pathCount)
	}

	batches := queue.FlushReady(time.Unix(1, 0))
	if len(batches) != 2 {
		t.Fatalf("expected both ready roots to flush, got %#v", batches)
	}
	if rootCount, pathCount := queue.PendingCounts(); rootCount != 0 || pathCount != 0 {
		t.Fatalf("expected pending counts to drain after flush, got roots=%d paths=%d", rootCount, pathCount)
	}
}

func TestDirtyQueueFlushReadyKeepsKnownFileDeltasOutOfSubtreeCoalescing(t *testing.T) {
	queue := NewDirtyQueue(DirtyQueueConfig{
		DebounceWindow: 0,
	})

	for i := 0; i < 8; i++ {
		queue.Push(DirtySignal{
			Kind:          DirtySignalKindPath,
			RootID:        "root-a",
			SemanticKind:  ChangeSemanticKindModify,
			Path:          filepath.Join(string(filepath.Separator), "root", "a", "parent", "file-"+string(rune('0'+i))+".txt"),
			PathTypeKnown: true,
			PathIsDir:     false,
			At:            time.Unix(0, 0),
		})
	}

	batches := queue.FlushReady(time.Unix(1, 0))
	if len(batches) != 1 {
		t.Fatalf("expected one root batch, got %d", len(batches))
	}
	if batches[0].Mode != ReconcileModeDirectDelta {
		t.Fatalf("expected direct-delta reconcile, got %s", batches[0].Mode)
	}
	if batches[0].DirtyPathCount != 8 {
		t.Fatalf("expected 8 dirty paths, got %d", batches[0].DirtyPathCount)
	}
	if len(batches[0].Paths) != 0 {
		t.Fatalf("expected file deltas not to become subtree paths, got %#v", batches[0].Paths)
	}
	if len(batches[0].DirectDeltas) != 8 {
		t.Fatalf("expected 8 direct file deltas, got %#v", batches[0].DirectDeltas)
	}
	for i, delta := range batches[0].DirectDeltas {
		expectedPath := filepath.Join(string(filepath.Separator), "root", "a", "parent", "file-"+string(rune('0'+i))+".txt")
		if delta.Path != expectedPath || delta.SemanticKind != ChangeSemanticKindModify {
			t.Fatalf("unexpected direct delta at %d: %#v", i, delta)
		}
	}
}

func TestDirtyQueueFlushReadyUsesMaxPendingWaitWindow(t *testing.T) {
	base := time.Unix(100, 0)

	t.Run("flushes-when-earliest-signal-exceeds-max-wait", func(t *testing.T) {
		queue := NewDirtyQueue(DirtyQueueConfig{
			DebounceWindow:       2 * time.Minute,
			MaxPendingWaitWindow: 5 * time.Second,
		})

		queue.Push(DirtySignal{Kind: DirtySignalKindPath, RootID: "root-a", Path: filepath.Join(string(filepath.Separator), "root", "first.txt"), PathTypeKnown: true, At: base})
		queue.Push(DirtySignal{Kind: DirtySignalKindPath, RootID: "root-a", Path: filepath.Join(string(filepath.Separator), "root", "latest.txt"), PathTypeKnown: true, At: base.Add(4 * time.Second)})

		batches := queue.FlushReadyWithDebounce(base.Add(6*time.Second), 2*time.Minute)
		if len(batches) != 1 {
			t.Fatalf("expected max pending wait to flush one batch, got %#v", batches)
		}
		if batches[0].Mode != ReconcileModeDirectDelta {
			t.Fatalf("expected direct-delta batch, got %s", batches[0].Mode)
		}
	})

	t.Run("waits-while-quiet-and-max-wait-are-both-pending", func(t *testing.T) {
		queue := NewDirtyQueue(DirtyQueueConfig{
			DebounceWindow:       2 * time.Minute,
			MaxPendingWaitWindow: 5 * time.Second,
		})

		queue.Push(DirtySignal{Kind: DirtySignalKindPath, RootID: "root-a", Path: filepath.Join(string(filepath.Separator), "root", "first.txt"), PathTypeKnown: true, At: base})
		queue.Push(DirtySignal{Kind: DirtySignalKindPath, RootID: "root-a", Path: filepath.Join(string(filepath.Separator), "root", "latest.txt"), PathTypeKnown: true, At: base.Add(4 * time.Second)})

		batches := queue.FlushReadyWithDebounce(base.Add(4500*time.Millisecond), 2*time.Minute)
		if len(batches) != 0 {
			t.Fatalf("expected dirty queue to wait before quiet or max window expires, got %#v", batches)
		}
	})

	t.Run("disabled-max-wait-keeps-latest-based-behavior", func(t *testing.T) {
		queue := NewDirtyQueue(DirtyQueueConfig{
			DebounceWindow:       2 * time.Minute,
			MaxPendingWaitWindow: 0,
		})

		queue.Push(DirtySignal{Kind: DirtySignalKindPath, RootID: "root-a", Path: filepath.Join(string(filepath.Separator), "root", "first.txt"), PathTypeKnown: true, At: base})
		queue.Push(DirtySignal{Kind: DirtySignalKindPath, RootID: "root-a", Path: filepath.Join(string(filepath.Separator), "root", "latest.txt"), PathTypeKnown: true, At: base.Add(4 * time.Second)})

		batches := queue.FlushReadyWithDebounce(base.Add(6*time.Second), 2*time.Minute)
		if len(batches) != 0 {
			t.Fatalf("expected disabled max wait to keep waiting for the quiet window, got %#v", batches)
		}
	})
}

// Only explicit root signals produce a root reconcile. Large path bursts used
// to escalate by count or by a per-root directory ratio, which queried SQLite
// on every flush and re-crawled whole roots for localized changes.
func TestDirtyQueueFlushReadyOnlyRootSignalsReconcileRoot(t *testing.T) {
	t.Run("many-paths-stay-scoped", func(t *testing.T) {
		queue := NewDirtyQueue(DirtyQueueConfig{
			DebounceWindow: 0,
		})

		for i := 0; i < 64; i++ {
			queue.Push(DirtySignal{
				Kind:          DirtySignalKindPath,
				RootID:        "root-a",
				Path:          filepath.Join(string(filepath.Separator), "root", "a", fmt.Sprintf("dir-%d", i), "grand"),
				PathTypeKnown: true,
				PathIsDir:     true,
				At:            time.Unix(0, 0),
			})
		}

		batches := queue.FlushReady(time.Unix(1, 0))
		if len(batches) != 1 {
			t.Fatalf("expected one root batch, got %d", len(batches))
		}
		if batches[0].Mode != ReconcileModeSubtree {
			t.Fatalf("expected subtree reconcile, got %s", batches[0].Mode)
		}
		if batches[0].DirtyPathCount != 64 {
			t.Fatalf("expected 64 dirty paths, got %d", batches[0].DirtyPathCount)
		}
	})

	t.Run("root-signal", func(t *testing.T) {
		queue := NewDirtyQueue(DirtyQueueConfig{
			DebounceWindow: 0,
		})

		queue.Push(DirtySignal{
			Kind:   DirtySignalKindRoot,
			RootID: "root-a",
			At:     time.Unix(0, 0),
		})

		batches := queue.FlushReady(time.Unix(1, 0))
		if len(batches) != 1 {
			t.Fatalf("expected one root batch, got %d", len(batches))
		}
		if batches[0].Mode != ReconcileModeRoot {
			t.Fatalf("expected root reconcile, got %s", batches[0].Mode)
		}
		if batches[0].DirtyPathCount != 1 {
			t.Fatalf("expected 1 dirty path, got %d", batches[0].DirtyPathCount)
		}
		if len(batches[0].Paths) != 0 {
			t.Fatalf("expected empty root path list, got %#v", batches[0].Paths)
		}
	})
}

func TestDirtyQueueFlushReadyPreservesAbsolutePathVolumeRoot(t *testing.T) {
	queue := NewDirtyQueue(DirtyQueueConfig{
		DebounceWindow: 0,
	})

	rootPath := filepath.Join(t.TempDir(), "root")
	dirPath := filepath.Join(rootPath, "nested")
	expectedScope := dirPath

	queue.Push(DirtySignal{
		Kind:          DirtySignalKindPath,
		RootID:        "root-a",
		Path:          dirPath,
		PathTypeKnown: true,
		PathIsDir:     true,
		At:            time.Unix(0, 0),
	})

	batches := queue.FlushReady(time.Unix(1, 0))
	if len(batches) != 1 {
		t.Fatalf("expected one root batch, got %d", len(batches))
	}
	if batches[0].Mode != ReconcileModeSubtree {
		t.Fatalf("expected subtree reconcile, got %s", batches[0].Mode)
	}
	if len(batches[0].Paths) != 1 || batches[0].Paths[0] != expectedScope {
		t.Fatalf("expected scope %q, got %#v", expectedScope, batches[0].Paths)
	}
}

// Metadata-only directory events relist one level, and sibling directories are
// never merged into a recursive rescan of their parent. A recursive scope still
// subsumes relists beneath it.
func TestDirtyQueueFlushReadyRelistsDirectoryMetadataWithoutMerging(t *testing.T) {
	queue := NewDirtyQueue(DirtyQueueConfig{
		DebounceWindow: 0,
	})

	parent := filepath.Join(string(filepath.Separator), "root", "a", "parent")
	for i := 0; i < 8; i++ {
		queue.Push(DirtySignal{
			Kind:          DirtySignalKindPath,
			RootID:        "root-a",
			SemanticKind:  ChangeSemanticKindMetadata,
			Path:          filepath.Join(parent, "child-"+string(rune('0'+i))),
			PathTypeKnown: true,
			PathIsDir:     true,
			At:            time.Unix(0, 0),
		})
	}
	renamed := filepath.Join(string(filepath.Separator), "root", "b")
	queue.Push(DirtySignal{Kind: DirtySignalKindPath, RootID: "root-a", SemanticKind: ChangeSemanticKindRename, Path: renamed, PathTypeKnown: true, PathIsDir: true, At: time.Unix(0, 0)})
	queue.Push(DirtySignal{Kind: DirtySignalKindPath, RootID: "root-a", SemanticKind: ChangeSemanticKindModify, Path: filepath.Join(renamed, "inner"), PathTypeKnown: true, PathIsDir: true, At: time.Unix(0, 0)})

	batches := queue.FlushReady(time.Unix(1, 0))
	if len(batches) != 1 {
		t.Fatalf("expected one root batch, got %d", len(batches))
	}
	if batches[0].Mode != ReconcileModeSubtree {
		t.Fatalf("expected subtree reconcile, got %s", batches[0].Mode)
	}
	if len(batches[0].Paths) != 1 || batches[0].Paths[0] != renamed {
		t.Fatalf("expected only the renamed directory to rescan recursively, got %#v", batches[0].Paths)
	}
	if len(batches[0].DirectoryPaths) != 8 {
		t.Fatalf("expected 8 unmerged one-level relists, got %#v", batches[0].DirectoryPaths)
	}
	for _, path := range batches[0].DirectoryPaths {
		if filepath.Dir(path) != parent {
			t.Fatalf("unexpected relist path %q", path)
		}
	}
	if len(batches[0].DirectDeltas) != 0 {
		t.Fatalf("expected directory signals not to become direct deltas, got %#v", batches[0].DirectDeltas)
	}
}
