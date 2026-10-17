package filesearch

import (
	"testing"
	"time"
)

// TestChangeSignalQueueOverflowDeliversLossMarker covers a quiet filesystem
// after the last event was dropped: recovery must not depend on another event.
func TestChangeSignalQueueOverflowDeliversLossMarker(t *testing.T) {
	queue := newChangeSignalQueue(1)
	defer queue.close()

	signal := ChangeSignal{Kind: ChangeSignalKindDirtyPath, RootID: "overflow", Path: "/tmp/overflow/file", Cursor: "before"}
	if !queue.offer(signal) {
		t.Fatal("expected first signal to be delivered")
	}
	signal.Cursor = "lost"
	if queue.offer(signal) {
		t.Fatal("expected full queue to report the signal as not delivered")
	}
	signal.Cursor = "later"
	if queue.offer(signal) {
		t.Fatal("expected signal behind a pending loss marker to be dropped")
	}

	if first := <-queue.Signals(); first.Cursor != "before" {
		t.Fatalf("first signal: %#v", first)
	}
	select {
	case marker := <-queue.Signals():
		if marker.Kind != ChangeSignalKindHistoryLost || marker.Cursor != "" || marker.RootID != "overflow" {
			t.Fatalf("unsafe loss marker: %#v", marker)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("lost overflow marker")
	}

	// Once the marker is delivered the root accepts ordinary signals again.
	deadline := time.Now().Add(2 * time.Second)
	for !queue.offer(ChangeSignal{Kind: ChangeSignalKindDirtyPath, RootID: "overflow", Path: "/tmp/overflow/next"}) {
		if time.Now().After(deadline) {
			t.Fatal("root stayed blocked after its loss marker was delivered")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestChangeSignalQueueFeedUnavailableReplacesPendingLossMarker(t *testing.T) {
	queue := newChangeSignalQueue(1)
	defer queue.close()

	queue.offer(ChangeSignal{Kind: ChangeSignalKindDirtyPath, RootID: "other", Path: "/tmp/other"})
	queue.offer(ChangeSignal{Kind: ChangeSignalKindDirtyPath, RootID: "root", Path: "/tmp/root/a"})
	queue.offer(ChangeSignal{Kind: ChangeSignalKindFeedUnavailable, RootID: "root", Path: "/tmp/root", Reason: "gone"})

	<-queue.Signals()
	select {
	case marker := <-queue.Signals():
		if marker.Kind != ChangeSignalKindFeedUnavailable || marker.RootID != "root" {
			t.Fatalf("expected feed unavailable to replace loss marker, got %#v", marker)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("lost feed unavailable marker")
	}
}
