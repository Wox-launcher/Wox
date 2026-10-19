package filesearch

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
)

// TestFallbackPoolSharesBudgetAndLifetime checks that closing one feed does not
// stop another feed watching the same directory, and the shared budget is exact.
func TestFallbackPoolSharesBudgetAndLifetime(t *testing.T) {
	pool := newFallbackWatchPool(1)
	first, err := pool.subscribe()
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, err := pool.subscribe()
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	root, other := t.TempDir(), t.TempDir()
	if err := first.Add(root); err != nil {
		t.Fatal(err)
	}
	if err := second.Add(root); err != nil {
		t.Fatalf("duplicate charged twice: %v", err)
	}
	if err := second.Add(other); !errors.Is(err, syscall.ENOSPC) {
		t.Fatalf("budget not enforced: %v", err)
	}
	if got := len(pool.watcher.WatchList()); got != 1 {
		t.Fatalf("native watches=%d", got)
	}
	file := filepath.Join(root, "app.desktop")
	if err := os.WriteFile(file, []byte("first"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, watch := range []*pooledFallbackWatcher{first, second} {
		select {
		case event := <-watch.Events():
			if event.Name != file {
				t.Fatalf("unexpected event: %+v", event)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("subscriber missed event")
		}
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	if got := len(pool.watcher.WatchList()); got != 1 {
		t.Fatalf("remaining subscription lost its watch: %d", got)
	}
	file = filepath.Join(root, "later.desktop")
	if err := os.WriteFile(file, []byte("second"), 0600); err != nil {
		t.Fatal(err)
	}
	deadline := time.After(2 * time.Second)
	for {
		select {
		case event := <-second.Events():
			if event.Name == file {
				goto delivered
			}
		case <-deadline:
			t.Fatal("remaining subscriber stopped receiving events")
		}
	}
delivered:
	if err := second.Remove(root); err != nil {
		t.Fatal(err)
	}
	if err := second.Add(other); err != nil {
		t.Fatalf("released budget not reusable: %v", err)
	}
	if err := second.Close(); err != nil {
		t.Fatal(err)
	}
	if pool.watcher != nil || len(pool.paths) != 0 {
		t.Fatal("last close retained native resources")
	}
}

// TestFallbackPoolOverflowIsDeliveredWithoutAnotherEvent verifies the loss marker
// does not compete for the already-full event channel or depend on future traffic.
func TestFallbackPoolOverflowIsDeliveredWithoutAnotherEvent(t *testing.T) {
	pool := newFallbackWatchPool(2)
	watch, err := pool.subscribe()
	if err != nil {
		t.Fatal(err)
	}
	defer watch.Close()
	root := t.TempDir()
	if err := watch.Add(root); err != nil {
		t.Fatal(err)
	}
	for i := 0; i <= cap(watch.events); i++ {
		pool.deliver(fsnotify.Event{Name: filepath.Join(root, "event"), Op: fsnotify.Write})
	}
	select {
	case err := <-watch.Errors():
		if !errors.Is(err, fsnotify.ErrEventOverflow) {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("lost overflow notification")
	}
	if len(watch.events) != cap(watch.events) {
		t.Fatal("event queue unexpectedly grew")
	}
}
