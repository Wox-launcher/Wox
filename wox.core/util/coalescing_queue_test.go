package util

import (
	"sync"
	"testing"
)

// TestCoalescingQueueCapacityAndWakeups checks replacement, overflow, FIFO barriers and shutdown.
func TestCoalescingQueueCapacityAndWakeups(t *testing.T) {
	type event struct {
		key   string
		value int
	}
	q := NewCoalescingQueue(3, func(e event) string { return e.key })
	for _, e := range []event{{"a", 1}, {"", 2}, {"a", 3}, {"a", 4}} {
		if !q.Push(e) {
			t.Fatal("replacement at capacity was rejected")
		}
	}
	if q.Push(event{"b", 5}) {
		t.Fatal("queue exceeded capacity")
	}
	for _, want := range []int{1, 2, 4} {
		select {
		case <-q.Ready():
		default:
			t.Fatal("lost wakeup")
		}
		got, ok := q.Pop()
		if !ok || got.value != want {
			t.Fatalf("got %+v, want %d", got, want)
		}
	}
	q.Push(event{"a", 6})
	q.Close()
	q.Push(event{"a", 7})
	if _, ok := q.Pop(); ok {
		t.Fatal("closed queue retained payloads")
	}
}

// TestCoalescingQueueConcurrentProducers keeps the last acknowledged value under contention.
func TestCoalescingQueueConcurrentProducers(t *testing.T) {
	q := NewCoalescingQueue(1, func(int) string { return "value" })
	var group sync.WaitGroup
	for range 8 {
		group.Add(1)
		go func() {
			defer group.Done()
			for i := range 100 {
				if !q.Push(i) {
					t.Error("coalescing overflow")
				}
			}
		}()
	}
	group.Wait()
	q.Push(1000)
	got, ok := q.Pop()
	if !ok || got != 1000 {
		t.Fatalf("last value lost: %d", got)
	}
}
