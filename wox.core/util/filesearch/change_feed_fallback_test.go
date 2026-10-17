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
	nestedPath := filepath.Join(parentPath, "src")
	mustMkdirAll(t, nestedPath)
	filePath := filepath.Join(nestedPath, "main.go")
	mustWriteTestFile(t, filePath, "package main")

	feed := NewFallbackChangeFeed()
	defer feed.Close()

	parent := RootRecord{ID: "root-parent", Path: parentPath, FeedType: RootFeedTypeFallback, FeedState: RootFeedStateReady}
	nested := RootRecord{ID: "root-nested", Path: nestedPath, Kind: RootKindUser, FeedType: RootFeedTypeFallback, FeedState: RootFeedStateReady}
	if err := feed.Refresh(context.Background(), []RootRecord{parent, nested}); err != nil {
		t.Fatalf("refresh fallback change feed: %v", err)
	}

	feed.handleEvent(fsnotify.Event{Name: filePath, Op: fsnotify.Write})

	signal := mustReadChangeSignalForPath(t, feed.Signals(), filePath)
	if signal.RootID != nested.ID {
		t.Fatalf("expected refreshed matcher to choose nested root, got %#v", signal)
	}
	if signal.Path != filePath {
		t.Fatalf("expected dirty path %q, got %q", filePath, signal.Path)
	}
}

func TestFallbackChangeFeedWatchesNestedDirectories(t *testing.T) {
	rootPath := filepath.Join(t.TempDir(), "Programs")
	nestedPath := filepath.Join(rootPath, "Scoop Apps", "Nested")
	mustMkdirAll(t, nestedPath)

	feed := NewFallbackChangeFeed()
	defer feed.Close()
	root := RootRecord{ID: "root-recursive", Path: rootPath, FeedType: RootFeedTypeFallback, FeedState: RootFeedStateReady}
	if err := feed.Refresh(context.Background(), []RootRecord{root}); err != nil {
		t.Fatalf("refresh fallback change feed: %v", err)
	}

	waitForFallbackWatch(t, feed, nestedPath)
	signal := mustReadChangeSignal(t, feed.Signals())
	if signal.Kind != ChangeSignalKindHistoryLost || signal.RootID != root.ID {
		t.Fatalf("expected history loss after initial registration, got %#v", signal)
	}

	filePath := filepath.Join(nestedPath, "deep.txt")
	mustWriteTestFile(t, filePath, "deep")
	signal = mustReadChangeSignalForPath(t, feed.Signals(), filePath)
	if signal.Kind != ChangeSignalKindDirtyPath || signal.RootID != root.ID {
		t.Fatalf("expected dirty path for nested file, got %#v", signal)
	}

	// Directories created after registration must be watched too.
	createdPath := filepath.Join(nestedPath, "created", "inner")
	mustMkdirAll(t, createdPath)
	waitForFallbackWatch(t, feed, createdPath)
	innerFile := filepath.Join(createdPath, "inner.txt")
	mustWriteTestFile(t, innerFile, "inner")
	mustReadChangeSignalForPath(t, feed.Signals(), innerFile)
}

func TestFallbackChangeFeedMarksRootPartialWhenBudgetIsExhausted(t *testing.T) {
	rootPath := filepath.Join(t.TempDir(), "budget")
	mustMkdirAll(t, filepath.Join(rootPath, "a", "b", "c"))

	feed := NewFallbackChangeFeed()
	defer feed.Close()
	feed.budget = 2
	root := RootRecord{ID: "root-budget", Path: rootPath, FeedType: RootFeedTypeFallback, FeedState: RootFeedStateReady}
	if err := feed.Refresh(context.Background(), []RootRecord{root}); err != nil {
		t.Fatalf("refresh fallback change feed: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		feed.mu.Lock()
		partial := feed.partialRoots[root.ID]
		watchedCount := len(feed.watched)
		feed.mu.Unlock()
		if partial {
			if watchedCount != 2 {
				t.Fatalf("expected watches to stop at the budget, got %d", watchedCount)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for root to be marked partial")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestFallbackChangeFeedReportsUnknownRemovalAsDirtyPath(t *testing.T) {
	rootPath := filepath.Join(t.TempDir(), "root")
	mustMkdirAll(t, rootPath)

	feed := NewFallbackChangeFeed()
	defer feed.Close()
	root := RootRecord{ID: "root-remove", Path: rootPath, FeedType: RootFeedTypeFallback, FeedState: RootFeedStateReady}
	if err := feed.Refresh(context.Background(), []RootRecord{root}); err != nil {
		t.Fatalf("refresh fallback change feed: %v", err)
	}

	removedPath := filepath.Join(rootPath, "gone")
	feed.handleEvent(fsnotify.Event{Name: removedPath, Op: fsnotify.Remove})
	signal := mustReadChangeSignalForPath(t, feed.Signals(), removedPath)
	if signal.Kind != ChangeSignalKindDirtyPath || signal.PathTypeKnown || signal.SemanticKind != ChangeSemanticKindRemove {
		t.Fatalf("expected path-level removal with unknown type, got %#v", signal)
	}

	feed.handleEvent(fsnotify.Event{Name: rootPath, Op: fsnotify.Write})
	feed.handleEvent(fsnotify.Event{Name: rootPath, Op: fsnotify.Remove})
	for {
		signal := mustReadChangeSignalForPath(t, feed.Signals(), rootPath)
		if signal.Reason == "fallback watches registered" {
			continue
		}
		if signal.Kind != ChangeSignalKindHistoryLost {
			t.Fatalf("expected removed root to report history loss, got %#v", signal)
		}
		return
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

func TestFallbackChangeFeedRefreshEmitsHistoryLostWhenUnavailableRootRecovers(t *testing.T) {
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
	if signal.Kind != ChangeSignalKindHistoryLost {
		t.Fatalf("expected history lost signal, got %q", signal.Kind)
	}
	if signal.RootID != root.ID {
		t.Fatalf("expected root id %q, got %q", root.ID, signal.RootID)
	}
}

func waitForFallbackWatch(t *testing.T, feed *FallbackChangeFeed, directory string) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for {
		feed.mu.Lock()
		_, watched := feed.watched[directory]
		feed.mu.Unlock()
		if watched {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for watch on %q", directory)
		}
		time.Sleep(10 * time.Millisecond)
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
