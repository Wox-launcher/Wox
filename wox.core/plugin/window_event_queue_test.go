package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"
)

// TestWindowEventQueueCoalescesAtCapacity preserves the final value instead of dropping a full-queue update.
func TestWindowEventQueueCoalescesAtCapacity(t *testing.T) {
	q := newWindowEventQueue()
	for i := range windowEventQueueCapacity {
		if !q.push(WindowEvent{Type: WindowEventWoxView, DocumentID: "d", CoalesceKey: fmt.Sprint(i), Data: json.RawMessage(`0`)}) {
			t.Fatal("unexpected overflow")
		}
	}
	if !q.push(WindowEvent{Type: WindowEventWoxView, DocumentID: "d", CoalesceKey: "0", Data: json.RawMessage(`99`)}) {
		t.Fatal("latest change dropped at capacity")
	}
	first, ok := q.pop()
	if !ok || string(first.Data) != "99" {
		t.Fatalf("stale first value: %+v", first)
	}
	for range windowEventQueueCapacity - 1 {
		if _, ok := q.pop(); !ok {
			t.Fatal("unrelated event lost")
		}
	}
	if _, ok := q.pop(); ok {
		t.Fatal("coalescing added another event")
	}
}

// TestWindowEventQueueRetainsOrderingBarriersAndDocuments prevents later edits from crossing a submit.
func TestWindowEventQueueRetainsOrderingBarriersAndDocuments(t *testing.T) {
	q := newWindowEventQueue()
	for _, e := range []WindowEvent{
		{Type: WindowEventWoxView, DocumentID: "a", CoalesceKey: "field", Data: json.RawMessage(`1`)},
		{Type: WindowEventWoxView, DocumentID: "a", Data: json.RawMessage(`"submit"`)},
		{Type: WindowEventWoxView, DocumentID: "a", CoalesceKey: "field", Data: json.RawMessage(`2`)},
		{Type: WindowEventWoxView, DocumentID: "a", CoalesceKey: "field", Data: json.RawMessage(`3`)},
		{Type: WindowEventWoxView, DocumentID: "b", CoalesceKey: "field", Data: json.RawMessage(`4`)},
	} {
		if !q.push(e) {
			t.Fatal("unexpected overflow")
		}
	}
	for _, want := range []string{`1`, `"submit"`, `3`, `4`} {
		e, ok := q.pop()
		if !ok || string(e.Data) != want {
			t.Fatalf("want %s got %+v", want, e)
		}
	}
	q.close()
	q.push(WindowEvent{Data: json.RawMessage(`5`)})
	if _, ok := q.pop(); ok {
		t.Fatal("closed queue accepted retained payload")
	}
}

// TestWindowEventQueueConcurrentProducers checks serialized replacement without a map-lock workaround.
func TestWindowEventQueueConcurrentProducers(t *testing.T) {
	q := newWindowEventQueue()
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range 100 {
				q.push(WindowEvent{CoalesceKey: "field", Data: json.RawMessage(fmt.Sprint(i))})
			}
		}()
	}
	wg.Wait()
	q.push(WindowEvent{CoalesceKey: "field", Data: json.RawMessage(`1000`)})
	e, ok := q.pop()
	if !ok || string(e.Data) != "1000" {
		t.Fatalf("final replacement %+v", e)
	}
	if _, ok := q.pop(); ok {
		t.Fatal("duplicate pending field")
	}
}

// TestSlowWindowCallbackReceivesLatestChange tests the real asynchronous delivery path with a burst.
func TestSlowWindowCallbackReceivesLatestChange(t *testing.T) {
	windowFactory.RLock()
	previous := windowFactory.create
	windowFactory.RUnlock()
	t.Cleanup(func() { SetWindowFactory(previous) })
	var driver *testWindowDriver
	SetWindowFactory(func(_ string, _ CreateWindowOption, emit func(WindowEvent), _ func(string) string) (WindowDriver, error) {
		driver = &testWindowDriver{emit: emit}
		return driver, nil
	})
	started, release := make(chan struct{}), make(chan struct{})
	received := make(chan WindowEvent, 10)
	var once sync.Once
	api := NewAPI(&Instance{}).(*APIImpl)
	r := api.CreateWindow(t.Context(), CreateWindowOption{OnEvent: func(_ context.Context, e WindowEvent) { once.Do(func() { close(started); <-release }); received <- e }})
	if !r.Success {
		t.Fatal(r.ErrMsg)
	}
	defer api.closeWindows()
	driver.emit(WindowEvent{Type: WindowEventReady, DocumentID: "current"})
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("callback did not start")
	}
	for i := range 1000 {
		driver.emit(WindowEvent{Type: WindowEventWoxView, DocumentID: "current", CoalesceKey: "field", Data: json.RawMessage(fmt.Sprint(i))})
	}
	driver.emit(WindowEvent{Type: WindowEventWoxView, DocumentID: "current", Data: json.RawMessage(`"done"`)})
	close(release)
	for i := range 3 {
		select {
		case e := <-received:
			if e.Type == WindowEventError {
				t.Fatalf("coalesced burst overflowed: %+v", e)
			}
			if i == 1 && string(e.Data) != "999" {
				t.Fatalf("did not receive final change: %+v", e)
			}
			if i == 2 && string(e.Data) != `"done"` {
				t.Fatalf("discrete event lost: %+v", e)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("queue lost wakeup")
		}
	}
}
