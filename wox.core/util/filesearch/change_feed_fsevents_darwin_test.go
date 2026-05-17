//go:build darwin

package filesearch

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewPlatformChangeFeedUsesFSEventsOnDarwin(t *testing.T) {
	feed := newPlatformChangeFeed()
	defer feed.Close()

	if feed.Mode() != "fsevents" {
		t.Fatalf("expected darwin platform feed mode fsevents, got %q", feed.Mode())
	}
}

func TestPrepareFSEventsRefreshUsesEarliestFreshCursorAndReconcilesExpiredRoots(t *testing.T) {
	now := time.Now()
	freshCursor := mustEncodeFeedCursorForTest(t, FeedCursor{
		FeedType:  RootFeedTypeFSEvents,
		UpdatedAt: now.Add(-time.Hour).UnixMilli(),
		FSEventID: 300,
	})
	expiredCursor := mustEncodeFeedCursorForTest(t, FeedCursor{
		FeedType:  RootFeedTypeFSEvents,
		UpdatedAt: now.Add(-26 * time.Hour).UnixMilli(),
		FSEventID: 150,
	})

	prepared := prepareFSEventsRefresh([]RootRecord{
		{
			ID:         "root-fresh",
			Path:       "/tmp/root-fresh",
			FeedType:   RootFeedTypeFSEvents,
			FeedCursor: freshCursor,
		},
		{
			ID:         "root-expired",
			Path:       "/tmp/root-expired",
			FeedType:   RootFeedTypeFSEvents,
			FeedCursor: expiredCursor,
		},
	}, now, defaultFeedCursorSafeWindow)

	if prepared.sinceEventID != 300 {
		t.Fatalf("expected earliest fresh event id 300, got %d", prepared.sinceEventID)
	}
	if len(prepared.watchRoots) != 2 {
		t.Fatalf("expected both roots to remain watched, got %d", len(prepared.watchRoots))
	}
	if len(prepared.signals) != 1 {
		t.Fatalf("expected one recovery signal for expired cursor, got %d", len(prepared.signals))
	}
	if prepared.signals[0].Kind != ChangeSignalKindRequiresRootReconcile || prepared.signals[0].RootID != "root-expired" {
		t.Fatalf("unexpected recovery signal: %#v", prepared.signals[0])
	}
}

func TestTranslateFSEventEmitsDirectoryDirtyPathAndCursor(t *testing.T) {
	root := RootRecord{
		ID:       "root-dir",
		Path:     "/tmp/root-dir",
		FeedType: RootFeedTypeFSEvents,
	}

	signals := translateFSEvent(root, "/tmp/root-dir/child", fseventFlagItemIsDir, 1234, time.Unix(100, 0))
	if len(signals) != 1 {
		t.Fatalf("expected one signal, got %d", len(signals))
	}
	if signals[0].Kind != ChangeSignalKindDirtyPath {
		t.Fatalf("expected dirty path signal, got %q", signals[0].Kind)
	}
	if !signals[0].PathTypeKnown || !signals[0].PathIsDir {
		t.Fatalf("expected directory dirty path signal, got %#v", signals[0])
	}
	cursor, ok := decodeFeedCursor(signals[0].Cursor, RootFeedTypeFSEvents)
	if !ok || cursor.FSEventID != 1234 {
		t.Fatalf("expected fsevents cursor to round trip, got %#v ok=%t", cursor, ok)
	}
}

func TestTranslateFSEventEscalatesDroppedHistoryToRequiresRootReconcile(t *testing.T) {
	root := RootRecord{
		ID:       "root-dropped",
		Path:     "/tmp/root-dropped",
		FeedType: RootFeedTypeFSEvents,
	}

	signals := translateFSEvent(root, "/tmp/root-dropped", fseventFlagMustScanSubDirs|fseventFlagKernelDropped, 55, time.Unix(100, 0))
	if len(signals) != 1 {
		t.Fatalf("expected one signal, got %d", len(signals))
	}
	if signals[0].Kind != ChangeSignalKindRequiresRootReconcile {
		t.Fatalf("expected requires root reconcile signal, got %q", signals[0].Kind)
	}
}

func TestFSEventsSnapshotRootFeedUsesCurrentEventID(t *testing.T) {
	feed := NewFSEventsChangeFeed()
	defer feed.Close()

	snapshot, err := feed.SnapshotRootFeed(t.Context(), RootRecord{
		ID:   "root-snapshot",
		Path: "/tmp/root-snapshot",
	})
	if err != nil {
		t.Fatalf("snapshot fsevents root feed: %v", err)
	}
	if snapshot.FeedType != RootFeedTypeFSEvents {
		t.Fatalf("expected fsevents snapshot feed type, got %q", snapshot.FeedType)
	}
	if snapshot.FeedState != RootFeedStateReady {
		t.Fatalf("expected ready feed state, got %q", snapshot.FeedState)
	}
	cursor, ok := decodeFeedCursor(snapshot.FeedCursor, RootFeedTypeFSEvents)
	if !ok {
		t.Fatalf("expected snapshot cursor to decode")
	}
	if cursor.FSEventID == 0 {
		t.Fatalf("expected non-zero fsevents event id, got %#v", cursor)
	}
}

func TestFSEventsChangeFeedEmitsSignalForCreatedFile(t *testing.T) {
	ctx := t.Context()
	rootPath := newStableFSEventsRoot(t, "live-fsevents")
	root := RootRecord{
		ID:       "root-live-fsevents",
		Path:     rootPath,
		FeedType: RootFeedTypeFSEvents,
	}

	feed := NewFSEventsChangeFeed()
	defer feed.Close()

	if err := feed.Refresh(ctx, []RootRecord{root}); err != nil {
		t.Fatalf("refresh live fsevents feed: %v", err)
	}
	time.Sleep(500 * time.Millisecond)

	filePath := filepath.Join(rootPath, fmt.Sprintf("created-%d.txt", time.Now().UnixNano()))
	if err := os.WriteFile(filePath, []byte("created"), 0o644); err != nil {
		t.Fatalf("write file for fsevents feed: %v", err)
	}

	timeout := time.After(5 * time.Second)
	for {
		select {
		case <-timeout:
			t.Fatalf("timed out waiting for fsevents signal for %q", filePath)
		case signal := <-feed.Signals():
			if signal.RootID != root.ID {
				continue
			}
			if signal.Kind == ChangeSignalKindDirtyPath && filepath.Clean(signal.Path) == filepath.Clean(filePath) {
				return
			}
			if signal.Kind == ChangeSignalKindDirtyRoot && filepath.Clean(signal.Path) == filepath.Clean(rootPath) {
				return
			}
		}
	}
}

func TestFSEventsChangeFeedContextCancelDoesNotStopActiveStream(t *testing.T) {
	rootPath := newStableFSEventsRoot(t, "cancel-keeps-stream")
	root := RootRecord{
		ID:       "root-cancel-keeps-stream",
		Path:     rootPath,
		FeedType: RootFeedTypeFSEvents,
	}

	feed := NewFSEventsChangeFeed()
	defer feed.Close()

	refreshCtx, cancel := context.WithCancel(t.Context())
	if err := feed.Refresh(refreshCtx, []RootRecord{root}); err != nil {
		t.Fatalf("refresh live fsevents feed: %v", err)
	}
	cancel()
	time.Sleep(500 * time.Millisecond)

	filePath := filepath.Join(rootPath, fmt.Sprintf("created-after-cancel-%d.txt", time.Now().UnixNano()))
	if err := os.WriteFile(filePath, []byte("created"), 0o644); err != nil {
		t.Fatalf("write file for fsevents feed after context cancel: %v", err)
	}

	timeout := time.After(5 * time.Second)
	for {
		select {
		case <-timeout:
			t.Fatalf("timed out waiting for fsevents signal after refresh context cancel for %q", filePath)
		case signal := <-feed.Signals():
			if signal.RootID != root.ID {
				continue
			}
			if signal.Kind == ChangeSignalKindDirtyPath && filepath.Clean(signal.Path) == filepath.Clean(filePath) {
				return
			}
			if signal.Kind == ChangeSignalKindDirtyRoot && filepath.Clean(signal.Path) == filepath.Clean(rootPath) {
				return
			}
		}
	}
}

func TestFSEventsChangeFeedRefreshGenerationAndCloseLifecycle(t *testing.T) {
	firstRoot := RootRecord{
		ID:       "root-refresh-generation-first",
		Path:     newStableFSEventsRoot(t, "refresh-generation-first"),
		FeedType: RootFeedTypeFSEvents,
	}
	secondRoot := RootRecord{
		ID:       "root-refresh-generation-second",
		Path:     newStableFSEventsRoot(t, "refresh-generation-second"),
		FeedType: RootFeedTypeFSEvents,
	}

	feed := NewFSEventsChangeFeed()
	defer feed.Close()

	if err := feed.Refresh(t.Context(), []RootRecord{firstRoot}); err != nil {
		t.Fatalf("refresh first fsevents root: %v", err)
	}
	feed.mu.RLock()
	firstGeneration := feed.streamGeneration
	firstStreamActive := feed.stream != nil
	feed.mu.RUnlock()
	if firstGeneration != 1 || !firstStreamActive {
		t.Fatalf("expected first refresh to start generation 1, got generation=%d active=%t", firstGeneration, firstStreamActive)
	}

	if err := feed.Refresh(t.Context(), []RootRecord{secondRoot}); err != nil {
		t.Fatalf("refresh second fsevents root: %v", err)
	}
	feed.mu.RLock()
	secondGeneration := feed.streamGeneration
	secondStreamActive := feed.stream != nil
	roots := append([]RootRecord(nil), feed.roots...)
	feed.mu.RUnlock()
	if secondGeneration != 2 || !secondStreamActive {
		t.Fatalf("expected second refresh to start generation 2, got generation=%d active=%t", secondGeneration, secondStreamActive)
	}
	if len(roots) != 1 || roots[0].ID != secondRoot.ID {
		t.Fatalf("expected second refresh to replace watched roots, got %#v", roots)
	}

	if err := feed.Close(); err != nil {
		t.Fatalf("close fsevents feed: %v", err)
	}
	feed.mu.RLock()
	streamActiveAfterClose := feed.stream != nil
	feed.mu.RUnlock()
	if streamActiveAfterClose {
		t.Fatalf("expected close to stop active stream")
	}
}

func newStableFSEventsRoot(t *testing.T, prefix string) string {
	t.Helper()

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory for fsevents root: %v", err)
	}

	basePath := filepath.Join(cwd, ".tmp-fsevents-roots")
	if err := os.MkdirAll(basePath, 0o755); err != nil {
		t.Fatalf("create fsevents root base: %v", err)
	}

	rootPath, err := os.MkdirTemp(basePath, prefix+"-")
	if err != nil {
		t.Fatalf("create fsevents root: %v", err)
	}

	t.Cleanup(func() {
		_ = os.RemoveAll(rootPath)
	})

	return rootPath
}
