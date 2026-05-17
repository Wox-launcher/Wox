package filesearch

import (
	"path/filepath"
	"testing"
	"time"
)

func TestDirtyQueueFlushReadyKeepsDisjointSubtreesSeparate(t *testing.T) {
	queue := NewDirtyQueue(DirtyQueueConfig{
		DebounceWindow:               50 * time.Millisecond,
		SiblingMergeThreshold:        8,
		RootEscalationPathThreshold:  512,
		RootEscalationDirectoryRatio: 0.10,
	})

	firstDir := filepath.Join(string(filepath.Separator), "root", "a", "b", "c")
	secondDir := filepath.Join(string(filepath.Separator), "root", "a", "d", "e")

	queue.Push(DirtySignal{Kind: DirtySignalKindPath, RootID: "root-a", Path: firstDir, PathTypeKnown: true, PathIsDir: true, At: time.Unix(0, 0)})
	queue.Push(DirtySignal{Kind: DirtySignalKindPath, RootID: "root-a", Path: secondDir, PathTypeKnown: true, PathIsDir: true, At: time.Unix(0, 0)})

	batches := queue.FlushReady(time.Unix(0, int64(60*time.Millisecond)), map[string]int{"root-a": 100})
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

func TestDirtyQueueFlushReadyKeepsKnownFileDeltasOutOfSubtreeCoalescing(t *testing.T) {
	queue := NewDirtyQueue(DirtyQueueConfig{
		DebounceWindow:               0,
		SiblingMergeThreshold:        8,
		RootEscalationPathThreshold:  512,
		RootEscalationDirectoryRatio: 0.10,
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

	batches := queue.FlushReady(time.Unix(1, 0), map[string]int{"root-a": 100})
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

func TestDirtyQueueFlushReadyEscalatesLargeBatchToRoot(t *testing.T) {
	t.Run("path-threshold", func(t *testing.T) {
		queue := NewDirtyQueue(DirtyQueueConfig{
			DebounceWindow:               0,
			SiblingMergeThreshold:        99,
			RootEscalationPathThreshold:  10,
			RootEscalationDirectoryRatio: 0.10,
		})

		for i := 0; i < 11; i++ {
			queue.Push(DirtySignal{
				Kind:          DirtySignalKindPath,
				RootID:        "root-a",
				Path:          filepath.Join(string(filepath.Separator), "root", "a", "dir-"+string(rune('a'+i)), "grand"),
				PathTypeKnown: true,
				PathIsDir:     true,
				At:            time.Unix(0, 0),
			})
		}

		batches := queue.FlushReady(time.Unix(1, 0), map[string]int{"root-a": 100})
		if len(batches) != 1 {
			t.Fatalf("expected one root batch, got %d", len(batches))
		}
		if batches[0].Mode != ReconcileModeRoot {
			t.Fatalf("expected root reconcile, got %s", batches[0].Mode)
		}
		if batches[0].DirtyPathCount != 11 {
			t.Fatalf("expected 11 dirty paths, got %d", batches[0].DirtyPathCount)
		}
	})

	t.Run("disabled-thresholds", func(t *testing.T) {
		queue := NewDirtyQueue(DirtyQueueConfig{
			DebounceWindow:               0,
			SiblingMergeThreshold:        99,
			RootEscalationPathThreshold:  0,
			RootEscalationDirectoryRatio: 0,
		})

		for i := 0; i < 4; i++ {
			queue.Push(DirtySignal{
				Kind:          DirtySignalKindPath,
				RootID:        "root-a",
				Path:          filepath.Join(string(filepath.Separator), "root", "a", "dir-"+string(rune('a'+i)), "grand"),
				PathTypeKnown: true,
				PathIsDir:     true,
				At:            time.Unix(0, 0),
			})
		}

		batches := queue.FlushReady(time.Unix(1, 0), map[string]int{"root-a": 10})
		if len(batches) != 1 {
			t.Fatalf("expected one root batch, got %d", len(batches))
		}
		if batches[0].Mode != ReconcileModeSubtree {
			t.Fatalf("expected subtree reconcile with thresholds disabled, got %s", batches[0].Mode)
		}
		if batches[0].DirtyPathCount != 4 {
			t.Fatalf("expected 4 dirty paths, got %d", batches[0].DirtyPathCount)
		}
	})

	t.Run("directory-ratio", func(t *testing.T) {
		queue := NewDirtyQueue(DirtyQueueConfig{
			DebounceWindow:               0,
			SiblingMergeThreshold:        99,
			RootEscalationPathThreshold:  512,
			RootEscalationDirectoryRatio: 0.25,
		})

		for i := 0; i < 4; i++ {
			queue.Push(DirtySignal{
				Kind:          DirtySignalKindPath,
				RootID:        "root-a",
				Path:          filepath.Join(string(filepath.Separator), "root", "a", "dir-"+string(rune('a'+i)), "grand"),
				PathTypeKnown: true,
				PathIsDir:     true,
				At:            time.Unix(0, 0),
			})
		}

		batches := queue.FlushReady(time.Unix(1, 0), map[string]int{"root-a": 10})
		if len(batches) != 1 {
			t.Fatalf("expected one root batch, got %d", len(batches))
		}
		if batches[0].Mode != ReconcileModeRoot {
			t.Fatalf("expected root reconcile, got %s", batches[0].Mode)
		}
		if batches[0].DirtyPathCount != 4 {
			t.Fatalf("expected 4 dirty paths, got %d", batches[0].DirtyPathCount)
		}
	})

	t.Run("root-signal", func(t *testing.T) {
		queue := NewDirtyQueue(DirtyQueueConfig{
			DebounceWindow:               0,
			SiblingMergeThreshold:        8,
			RootEscalationPathThreshold:  512,
			RootEscalationDirectoryRatio: 0.10,
		})

		queue.Push(DirtySignal{
			Kind:   DirtySignalKindRoot,
			RootID: "root-a",
			At:     time.Unix(0, 0),
		})

		batches := queue.FlushReady(time.Unix(1, 0), map[string]int{"root-a": 10})
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
		DebounceWindow:               0,
		SiblingMergeThreshold:        8,
		RootEscalationPathThreshold:  512,
		RootEscalationDirectoryRatio: 0.10,
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

	batches := queue.FlushReady(time.Unix(1, 0), map[string]int{"root-a": 100})
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

func TestDirtyQueueFlushReadyStillCoalescesDirectorySubtrees(t *testing.T) {
	queue := NewDirtyQueue(DirtyQueueConfig{
		DebounceWindow:               0,
		SiblingMergeThreshold:        8,
		RootEscalationPathThreshold:  512,
		RootEscalationDirectoryRatio: 0.10,
	})

	for i := 0; i < 8; i++ {
		queue.Push(DirtySignal{
			Kind:          DirtySignalKindPath,
			RootID:        "root-a",
			SemanticKind:  ChangeSemanticKindModify,
			Path:          filepath.Join(string(filepath.Separator), "root", "a", "parent", "child-"+string(rune('0'+i))),
			PathTypeKnown: true,
			PathIsDir:     true,
			At:            time.Unix(0, 0),
		})
	}

	batches := queue.FlushReady(time.Unix(1, 0), map[string]int{"root-a": 100})
	if len(batches) != 1 {
		t.Fatalf("expected one root batch, got %d", len(batches))
	}
	if batches[0].Mode != ReconcileModeSubtree {
		t.Fatalf("expected subtree reconcile, got %s", batches[0].Mode)
	}
	expectedPath := filepath.Join(string(filepath.Separator), "root", "a", "parent")
	if len(batches[0].Paths) != 1 || batches[0].Paths[0] != expectedPath {
		t.Fatalf("expected sibling directory collapse to %s, got %#v", expectedPath, batches[0].Paths)
	}
	if len(batches[0].DirectDeltas) != 0 {
		t.Fatalf("expected directory signals not to become direct deltas, got %#v", batches[0].DirectDeltas)
	}
}
