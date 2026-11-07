package snippets

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

type testExpansionBackend struct {
	callback func(inputEvent)
	focus    atomic.Uint64
	result   chan replacement
	text     chan string
}

type interruptedExpansionBackend struct {
	testExpansionBackend
	cancelled chan bool
}

type delayedCleanupBackend struct {
	testExpansionBackend
	applied chan struct{}
	release chan struct{}
	calls   int
}

func (b *delayedCleanupBackend) replace(_ context.Context, target uint64, count int, text string, checks replacementChecks) error {
	if !checks.valid() || !checks.current() {
		return errors.New("invalid context")
	}
	b.calls++
	if b.calls == 1 {
		checks.applied()
		close(b.applied)
		<-b.release
		return nil
	}
	b.result <- replacement{target: target, deletes: count}
	b.text <- text
	return nil
}

func TestExpansionUndoDuringClipboardCleanup(t *testing.T) {
	b := &delayedCleanupBackend{
		testExpansionBackend: testExpansionBackend{result: make(chan replacement, 1), text: make(chan string, 1)},
		applied:              make(chan struct{}),
		release:              make(chan struct{}),
	}
	b.focus.Store(1)
	s, err := startExpansion(context.Background(), b, []Record{{ID: "a", Keyword: "x", Text: "one\ntwo"}})
	if err != nil {
		t.Fatal(err)
	}
	defer s.close()
	defer close(b.release)
	b.callback(inputEvent{kind: inputText, down: true, key: 1, target: 1, text: "x"})
	b.callback(inputEvent{key: 1, target: 1})
	select {
	case <-b.applied:
	case <-time.After(time.Second):
		t.Fatal("expansion was not applied")
	}
	b.callback(inputEvent{kind: inputBackspace, down: true, key: 8, target: 1})
	b.callback(inputEvent{key: 8, target: 1})
	b.release <- struct{}{}
	select {
	case undo := <-b.result:
		if undo.deletes != 6 || <-b.text != "x" {
			t.Fatalf("undo = %#v", undo)
		}
	case <-time.After(time.Second):
		t.Fatal("undo was not queued during clipboard cleanup")
	}
}

func (b *interruptedExpansionBackend) replace(_ context.Context, _ uint64, _ int, _ string, checks replacementChecks) error {
	if !checks.valid() {
		b.cancelled <- false
		return errors.New("invalid context")
	}
	// Model an event drained after the last potentially blocking focus query.
	b.callback(inputEvent{kind: inputText, down: true, key: 9, target: 1, text: "x"})
	b.cancelled <- !checks.current()
	return errors.New("replacement cancelled")
}

func TestExpansionCancelsInputAfterFocusCheck(t *testing.T) {
	b := &interruptedExpansionBackend{cancelled: make(chan bool, 1)}
	b.focus.Store(1)
	s, err := startExpansion(context.Background(), b, []Record{{ID: "a", Keyword: ";a", Text: "hello"}})
	if err != nil {
		t.Fatal(err)
	}
	defer s.close()
	for i, char := range ";a" {
		b.callback(inputEvent{kind: inputText, down: true, key: uint32(i + 1), target: 1, text: string(char)})
		b.callback(inputEvent{key: uint32(i + 1), target: 1})
	}
	select {
	case cancelled := <-b.cancelled:
		if !cancelled {
			t.Fatal("input received after the focus check did not invalidate replacement")
		}
	case <-time.After(time.Second):
		t.Fatal("replacement did not reach final input check")
	}
}

func (b *testExpansionBackend) start(callback func(inputEvent)) (func() error, error) {
	b.callback = callback
	return func() error { return nil }, nil
}

func (b *testExpansionBackend) target() uint64 { return b.focus.Load() }

func (b *testExpansionBackend) replace(_ context.Context, target uint64, count int, text string, checks replacementChecks) error {
	if !checks.valid() || !checks.current() {
		return errors.New("invalid context")
	}
	b.result <- replacement{target: target, deletes: count}
	b.text <- text
	return nil
}

// TestExpansionWorker covers the real callback-to-worker path and immediate undo.
func TestExpansionWorker(t *testing.T) {
	b := &testExpansionBackend{result: make(chan replacement, 2), text: make(chan string, 2)}
	b.focus.Store(1)
	s, err := startExpansion(context.Background(), b, []Record{{ID: "a", Keyword: ";a ", Text: "hello"}})
	if err != nil {
		t.Fatal(err)
	}
	defer s.close()
	for i, char := range ";a " {
		b.callback(inputEvent{kind: inputText, down: true, key: uint32(i + 1), target: 1, text: string(char)})
		b.callback(inputEvent{key: uint32(i + 1), target: 1})
	}
	select {
	case got := <-b.result:
		if got.deletes != 3 || <-b.text != "hello" {
			t.Fatalf("replacement = %#v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("expansion did not run")
	}
	// The fake backend reports before completed() stores undo; wait for its state.
	deadline := time.Now().Add(time.Second)
	for {
		s.engine.mu.Lock()
		ready := s.engine.undo != nil
		s.engine.mu.Unlock()
		if ready {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("undo was not recorded")
		}
		time.Sleep(time.Millisecond)
	}
	b.callback(inputEvent{kind: inputBackspace, down: true, key: 8, target: 1})
	b.callback(inputEvent{key: 8, target: 1})
	select {
	case got := <-b.result:
		if got.deletes != 4 || <-b.text != ";a " {
			t.Fatalf("undo = %#v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("undo did not run")
	}
}
