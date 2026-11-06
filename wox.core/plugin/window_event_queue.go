package plugin

import "sync"

const windowEventQueueCapacity = 128

// windowEventQueue serializes delivery without blocking the UI on plugin callbacks.
// Only the producer knows which events are replaceable; keys are opaque here.
type windowEventQueue struct {
	mu      sync.Mutex
	pending []WindowEvent
	ready   chan struct{}
	closed  bool
}

func newWindowEventQueue() *windowEventQueue {
	return &windowEventQueue{ready: make(chan struct{}, 1)}
}

// push replaces a pending change before checking capacity. Nonreplaceable events
// form ordering barriers so an edit after submit cannot overwrite the pre-submit edit.
// False reports capacity overflow; late events after close are intentionally ignored.
func (q *windowEventQueue) push(event WindowEvent) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return true
	}
	if event.CoalesceKey != "" {
		for i := len(q.pending) - 1; i >= 0; i-- {
			old := q.pending[i]
			if old.CoalesceKey == "" {
				break
			}
			if old.CoalesceKey == event.CoalesceKey && old.DocumentID == event.DocumentID && old.Type == event.Type {
				q.pending[i] = event
				return true
			}
		}
	}
	if len(q.pending) >= windowEventQueueCapacity {
		return false
	}
	q.pending = append(q.pending, event)
	select {
	case q.ready <- struct{}{}:
	default:
	}
	return true
}

// pop consumes one event and rearms the wakeup if another is pending.
func (q *windowEventQueue) pop() (WindowEvent, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.pending) == 0 {
		return WindowEvent{}, false
	}
	event := q.pending[0]
	copy(q.pending, q.pending[1:])
	q.pending[len(q.pending)-1] = WindowEvent{}
	q.pending = q.pending[:len(q.pending)-1]
	if len(q.pending) > 0 {
		select {
		case q.ready <- struct{}{}:
		default:
		}
	}
	return event, true
}

// close releases payloads immediately and prevents late producers from growing an abandoned queue.
func (q *windowEventQueue) close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.closed = true
	q.pending = nil
}
