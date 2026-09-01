package filesearch

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
)

func TestFallbackChangeFeedHandleEventEmitsDirtyPathForDirectChildCreate(t *testing.T) {
	rootPath := filepath.Join(t.TempDir(), "root")
	mustMkdirAll(t, rootPath)
	filePath := filepath.Join(rootPath, "child.txt")
	mustWriteTestFile(t, filePath, "child")

	feed := NewFallbackChangeFeed()
	defer feed.Close()

	root := RootRecord{
		ID:        "root-direct-child",
		Path:      rootPath,
		FeedType:  RootFeedTypeFallback,
		FeedState: RootFeedStateReady,
	}
	if err := feed.Refresh(context.Background(), []RootRecord{root}); err != nil {
		t.Fatalf("refresh fallback change feed: %v", err)
	}

	feed.handleEvent(fsnotify.Event{Name: filePath, Op: fsnotify.Create})

	signal := mustReadChangeSignalForPath(t, feed.Signals(), filePath)
	if signal.Kind != ChangeSignalKindDirtyPath {
		t.Fatalf("expected dirty path signal, got %q", signal.Kind)
	}
	if signal.RootID != root.ID {
		t.Fatalf("expected root id %q, got %q", root.ID, signal.RootID)
	}
	if signal.Path != filePath {
		t.Fatalf("expected dirty path %q, got %q", filePath, signal.Path)
	}
}

func TestFallbackChangeFeedHandleEventUsesRefreshedLongestRootMatcher(t *testing.T) {
	parentPath := filepath.Join(t.TempDir(), "workspace")
	dynamicPath := filepath.Join(parentPath, "src")
	mustMkdirAll(t, dynamicPath)
	filePath := filepath.Join(dynamicPath, "main.go")
	mustWriteTestFile(t, filePath, "package main")

	feed := NewFallbackChangeFeed()
	defer feed.Close()

	parent := RootRecord{ID: "root-parent", Path: parentPath, FeedType: RootFeedTypeFallback, FeedState: RootFeedStateReady}
	dynamic := RootRecord{ID: "root-dynamic", Path: dynamicPath, Kind: RootKindDynamic, DynamicParentRootID: parent.ID, FeedType: RootFeedTypeFallback, FeedState: RootFeedStateReady}
	if err := feed.Refresh(context.Background(), []RootRecord{parent, dynamic}); err != nil {
		t.Fatalf("refresh fallback change feed: %v", err)
	}

	feed.handleEvent(fsnotify.Event{Name: filePath, Op: fsnotify.Write})

	signal := mustReadChangeSignalForPath(t, feed.Signals(), filePath)
	if signal.RootID != dynamic.ID {
		t.Fatalf("expected refreshed matcher to choose dynamic root, got %#v", signal)
	}
	if signal.Path != filePath {
		t.Fatalf("expected dirty path %q, got %q", filePath, signal.Path)
	}
}

func TestFallbackChangeFeedKeepsOneWatchPerRoot(t *testing.T) {
	rootPath := filepath.Join(t.TempDir(), "Programs")
	mustMkdirAll(t, filepath.Join(rootPath, "Scoop Apps", "Nested"))

	feed := NewFallbackChangeFeed()
	defer feed.Close()
	if err := feed.Refresh(context.Background(), []RootRecord{{ID: "root-bounded", Path: rootPath}}); err != nil {
		t.Fatalf("refresh fallback change feed: %v", err)
	}

	feed.mu.RLock()
	watches := feed.watcher.WatchList()
	feed.mu.RUnlock()
	if len(watches) != 1 || watches[0] != rootPath {
		t.Fatalf("expected one root watch, got %#v", watches)
	}
}

func TestFallbackChangeFeedRefreshEmitsFeedUnavailableForUnwatchableRoot(t *testing.T) {
	rootPath := filepath.Join(t.TempDir(), "missing-root")

	feed := NewFallbackChangeFeed()
	defer feed.Close()

	root := RootRecord{
		ID:        "root-unwatchable",
		Path:      rootPath,
		FeedType:  RootFeedTypeFallback,
		FeedState: RootFeedStateReady,
	}
	if err := feed.Refresh(context.Background(), []RootRecord{root}); err != nil {
		t.Fatalf("refresh fallback change feed: %v", err)
	}

	signal := mustReadChangeSignal(t, feed.Signals())
	if signal.Kind != ChangeSignalKindFeedUnavailable {
		t.Fatalf("expected feed unavailable signal, got %q", signal.Kind)
	}
	if signal.RootID != root.ID {
		t.Fatalf("expected root id %q, got %q", root.ID, signal.RootID)
	}
	if signal.Reason == "" {
		t.Fatalf("expected unwatchable root to include reason")
	}
}

func TestFallbackChangeFeedRefreshEmitsRequiresRootReconcileWhenUnavailableRootRecovers(t *testing.T) {
	rootPath := filepath.Join(t.TempDir(), "root-recovered")
	mustMkdirAll(t, rootPath)

	feed := NewFallbackChangeFeed()
	defer feed.Close()

	root := RootRecord{
		ID:        "root-recovered",
		Path:      rootPath,
		FeedType:  RootFeedTypeFallback,
		FeedState: RootFeedStateUnavailable,
	}
	if err := feed.Refresh(context.Background(), []RootRecord{root}); err != nil {
		t.Fatalf("refresh fallback change feed: %v", err)
	}

	signal := mustReadChangeSignal(t, feed.Signals())
	if signal.Kind != ChangeSignalKindRequiresRootReconcile {
		t.Fatalf("expected requires root reconcile signal, got %q", signal.Kind)
	}
	if signal.RootID != root.ID {
		t.Fatalf("expected root id %q, got %q", root.ID, signal.RootID)
	}
}

func mustReadChangeSignal(t *testing.T, signals <-chan ChangeSignal) ChangeSignal {
	t.Helper()

	select {
	case signal := <-signals:
		return signal
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for change signal")
		return ChangeSignal{}
	}
}

func mustReadChangeSignalForPath(t *testing.T, signals <-chan ChangeSignal, targetPath string) ChangeSignal {
	t.Helper()

	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	for {
		select {
		case signal := <-signals:
			if signal.Path == targetPath {
				return signal
			}
		case <-timer.C:
			t.Fatalf("timed out waiting for change signal for %q", targetPath)
			return ChangeSignal{}
		}
	}
}
