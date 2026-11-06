package util

import "sync"

// CoalescingQueue bounds producer work while a single consumer is busy. An empty
// key is an ordering barrier; matching keys can replace pending values only after
// the last barrier. Owners supply opaque keys, including any document identity.
type CoalescingQueue[T any] struct {
	mu       sync.Mutex
	pending  []coalescingEntry[T]
	ready    chan struct{}
	capacity int
	key      func(T) string
	closed   bool
}

type coalescingEntry[T any] struct {
	value T
	key   string
}

// NewCoalescingQueue creates a bounded inbox; key is called outside the queue lock.
func NewCoalescingQueue[T any](capacity int, key func(T) string) *CoalescingQueue[T] {
	return &CoalescingQueue[T]{capacity: max(1, capacity), key: key, ready: make(chan struct{}, 1)}
}

func (q *CoalescingQueue[T]) Ready() <-chan struct{} { return q.ready }

// Push replaces before checking capacity, preserving the final value even at capacity.
// False reports overflow. Closed queues intentionally ignore late producers.
func (q *CoalescingQueue[T]) Push(value T) bool {
	key := ""
	if q.key != nil {
		key = q.key(value)
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return true
	}
	if key != "" {
		for i := len(q.pending) - 1; i >= 0; i-- {
			if q.pending[i].key == "" {
				break
			}
			if q.pending[i].key == key {
				q.pending[i].value = value
				return true
			}
		}
	}
	if len(q.pending) >= q.capacity {
		return false
	}
	q.pending = append(q.pending, coalescingEntry[T]{value: value, key: key})
	select {
	case q.ready <- struct{}{}:
	default:
	}
	return true
}

// Pop consumes one value and rearms the wakeup when more work remains.
func (q *CoalescingQueue[T]) Pop() (T, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.pending) == 0 {
		var zero T
		return zero, false
	}
	value := q.pending[0].value
	copy(q.pending, q.pending[1:])
	q.pending[len(q.pending)-1] = coalescingEntry[T]{}
	q.pending = q.pending[:len(q.pending)-1]
	if len(q.pending) > 0 {
		select {
		case q.ready <- struct{}{}:
		default:
		}
	}
	return value, true
}

// Close releases queued payloads. The consumer's cancellation is owned by its caller.
func (q *CoalescingQueue[T]) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.closed = true
	q.pending = nil
}
