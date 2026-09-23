package util

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
)

func waitForEvent(t *testing.T, events <-chan fsnotify.Event, name string) {
	t.Helper()
	deadline := time.After(3 * time.Second)
	for {
		select {
		case event := <-events:
			if filepath.Clean(event.Name) == filepath.Clean(name) {
				return
			}
		case <-deadline:
			t.Fatalf("timed out waiting for event on %s", name)
		}
	}
}

func TestWatchDirectorySharesOneNativeWatchBetweenSubscribers(t *testing.T) {
	directory := t.TempDir()
	first := make(chan fsnotify.Event, 16)
	second := make(chan fsnotify.Event, 16)
	firstWatch, err := WatchDirectory(directory, func(event fsnotify.Event) { first <- event }, nil)
	if err != nil {
		t.Fatalf("first subscription: %v", err)
	}
	defer firstWatch.Close()
	secondWatch, err := WatchDirectory(directory, func(event fsnotify.Event) { second <- event }, nil)
	if err != nil {
		t.Fatalf("second subscription: %v", err)
	}

	sharedDirectoryWatcher.mu.Lock()
	watchList := sharedDirectoryWatcher.watcher.WatchList()
	subscriberCount := len(sharedDirectoryWatcher.subscribers[filepath.Clean(directory)])
	sharedDirectoryWatcher.mu.Unlock()
	if subscriberCount != 2 {
		t.Fatalf("subscribers = %d, want 2", subscriberCount)
	}
	if count := countWatchListEntries(watchList, directory); count != 1 {
		t.Fatalf("native watches for %s = %d, want 1", directory, count)
	}

	created := filepath.Join(directory, "first.txt")
	if err := os.WriteFile(created, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	waitForEvent(t, first, created)
	waitForEvent(t, second, created)

	// Closing one subscriber must keep the native watch alive for the other.
	secondWatch.Close()
	secondWatch.Close()
	sharedDirectoryWatcher.mu.Lock()
	watchList = sharedDirectoryWatcher.watcher.WatchList()
	sharedDirectoryWatcher.mu.Unlock()
	if count := countWatchListEntries(watchList, directory); count != 1 {
		t.Fatalf("native watches after closing one subscriber = %d, want 1", count)
	}
	afterClose := filepath.Join(directory, "second.txt")
	if err := os.WriteFile(afterClose, []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}
	waitForEvent(t, first, afterClose)
	select {
	case event := <-second:
		if filepath.Clean(event.Name) == afterClose {
			t.Fatalf("closed subscription still received %v", event)
		}
	case <-time.After(200 * time.Millisecond):
	}

	firstWatch.Close()
	sharedDirectoryWatcher.mu.Lock()
	watchList = sharedDirectoryWatcher.watcher.WatchList()
	_, stillRegistered := sharedDirectoryWatcher.subscribers[filepath.Clean(directory)]
	sharedDirectoryWatcher.mu.Unlock()
	if stillRegistered || countWatchListEntries(watchList, directory) != 0 {
		t.Fatalf("native watch should be removed after the last subscriber closes")
	}
}

func TestWatchDirectoryIsolatesPanickingSubscriber(t *testing.T) {
	directory := t.TempDir()
	healthy := make(chan fsnotify.Event, 16)
	panicking, err := WatchDirectory(directory, func(fsnotify.Event) { panic("subscriber failure") }, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer panicking.Close()
	healthyWatch, err := WatchDirectory(directory, func(event fsnotify.Event) { healthy <- event }, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer healthyWatch.Close()

	for index := 0; index < 2; index++ {
		created := filepath.Join(directory, "file"+string(rune('a'+index))+".txt")
		if err := os.WriteFile(created, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		waitForEvent(t, healthy, created)
	}
}

// TestDirectoryWatcherSubscribeDuringEventBurstDoesNotDeadlock guards the lock discipline: native
// AddWith/Remove must never run while holding the table lock the dispatch loop needs.
func TestDirectoryWatcherSubscribeDuringEventBurstDoesNotDeadlock(t *testing.T) {
	watcher := NewDirectoryWatcher()
	defer watcher.Close()
	busy := t.TempDir()
	other := t.TempDir()
	slow, err := watcher.Watch(busy, func(fsnotify.Event) { time.Sleep(2 * time.Millisecond) }, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer slow.Close()

	stop := make(chan struct{})
	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		for index := 0; ; index++ {
			select {
			case <-stop:
				return
			default:
			}
			_ = os.WriteFile(filepath.Join(busy, fmt.Sprintf("f%d.txt", index%50)), []byte{byte(index)}, 0o644)
		}
	}()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for index := 0; index < 200; index++ {
			watch, err := watcher.Watch(other, func(fsnotify.Event) {}, nil)
			if err != nil {
				t.Error(err)
				return
			}
			watch.Close()
		}
	}()
	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("subscribe/unsubscribe stalled while events were flowing")
	}
	close(stop)
	<-writerDone
}

func countWatchListEntries(watchList []string, directory string) int {
	count := 0
	for _, entry := range watchList {
		if filepath.Clean(entry) == filepath.Clean(directory) {
			count++
		}
	}
	return count
}
