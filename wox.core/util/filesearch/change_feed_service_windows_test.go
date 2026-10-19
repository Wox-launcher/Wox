package filesearch

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestServiceFeedSwitchesBothWays uses the real native fallback and an isolated
// broker, so no running service or user-owned application is stopped by the test.
func TestServiceFeedSwitchesBothWays(t *testing.T) {
	root := RootRecord{ID: "apps", Path: t.TempDir()}
	broker := &nativeChangeBroker{volumes: map[string]bool{}, feeds: map[*serviceChangeSubscription]struct{}{}}
	fallback := newWindowsTreeChangeFeed()
	feed := newServiceChangeFeed(broker, fallback)
	defer feed.Close()
	if err := feed.Refresh(context.Background(), []RootRecord{root}); err != nil {
		t.Fatal(err)
	}
	wait := func(covered bool) {
		t.Helper()
		deadline := time.After(5 * time.Second)
		for {
			fallback.mu.Lock()
			count := len(fallback.watches)
			fallback.mu.Unlock()
			if (count == 0) == covered {
				return
			}
			select {
			case <-deadline:
				t.Fatalf("fallback did not switch: covered=%t watches=%d", covered, count)
			case <-time.After(10 * time.Millisecond):
			}
		}
	}
	wait(false)
	broker.setAvailable(root.Path, true)
	wait(true)
	path := filepath.Join(root.Path, "service.lnk")
	broker.publish(ChangeSignal{Kind: ChangeSignalKindDirtyPath, Path: path, FeedType: RootFeedTypeUSN})
	if got := mustReadChangeSignalForPath(t, feed.Signals(), path); got.RootID != root.ID || got.FeedType != RootFeedTypeUSN {
		t.Fatalf("wrong routed signal: %+v", got)
	}
	broker.publish(ChangeSignal{Kind: ChangeSignalKindDirtyPath, Path: filepath.Join(filepath.Dir(root.Path), "outside.lnk")})
	broker.setAvailable(root.Path, false)
	wait(false)
	// Wait until the kernel read is armed, then verify fallback delivers events.
	deadline := time.After(5 * time.Second)
	for {
		select {
		case signal := <-feed.Signals():
			if signal.Reason == "subtree watch registered" {
				goto ready
			}
		case <-deadline:
			t.Fatal("fallback was not armed")
		}
	}
ready:
	path = filepath.Join(root.Path, "fallback.lnk")
	mustWriteTestFile(t, path, "shortcut")
	if got := mustReadChangeSignalForPath(t, feed.Signals(), path); got.FeedType != RootFeedTypeFallback {
		t.Fatalf("not fallback: %+v", got)
	}
	broker.setAvailable(root.Path, true)
	wait(true)
}

// TestWindowsSubtreeWatchHasNoDescendantBudget exercises a tree larger than the
// old 512-watch limit, plus new directories and both halves of a file rename.
func TestWindowsSubtreeWatchHasNoDescendantBudget(t *testing.T) {
	root := RootRecord{ID: "tree", Path: t.TempDir()}
	for i := 0; i < 520; i++ {
		mustMkdirAll(t, filepath.Join(root.Path, fmt.Sprintf("dir-%03d", i), "nested"))
	}
	feed := newWindowsTreeChangeFeed()
	defer feed.Close()
	if err := feed.Refresh(context.Background(), []RootRecord{root}); err != nil {
		t.Fatal(err)
	}
	if signal := mustReadChangeSignal(t, feed.Signals()); signal.Kind != ChangeSignalKindHistoryLost {
		t.Fatalf("not registered: %+v", signal)
	}
	path := filepath.Join(root.Path, "dir-519", "nested", "test.lnk")
	mustWriteTestFile(t, path, "shortcut")
	mustReadChangeSignalForPath(t, feed.Signals(), path)
	newPath := filepath.Join(filepath.Dir(path), "renamed.lnk")
	if err := os.Rename(path, newPath); err != nil {
		t.Fatal(err)
	}
	deadline := time.After(5 * time.Second)
	oldFound, newFound := false, false
	for !oldFound || !newFound {
		select {
		case signal := <-feed.Signals():
			oldFound = oldFound || signal.Path == path && signal.SemanticKind == ChangeSemanticKindRemove
			newFound = newFound || signal.Path == newPath && signal.SemanticKind == ChangeSemanticKindCreate
		case <-deadline:
			t.Fatalf("rename missing: old=%t new=%t", oldFound, newFound)
		}
	}
	created := filepath.Join(root.Path, "new", "deep", "app.lnk")
	mustMkdirAll(t, filepath.Dir(created))
	mustWriteTestFile(t, created, "shortcut")
	mustReadChangeSignalForPath(t, feed.Signals(), created)
	feed.mu.Lock()
	count := len(feed.watches)
	feed.mu.Unlock()
	if count != 1 {
		t.Fatalf("expected one native watch, got %d", count)
	}
	if err := feed.Refresh(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
}

// TestWindowsSubtreeRejectsTruncatedRecords covers malformed kernel record bounds.
func TestWindowsSubtreeRejectsTruncatedRecords(t *testing.T) {
	feed := newWindowsTreeChangeFeed()
	defer feed.Close()
	if err := feed.decode(RootRecord{ID: "root", Path: t.TempDir()}, []byte{1, 2, 3}); err == nil {
		t.Fatal("accepted truncated record")
	}
}
