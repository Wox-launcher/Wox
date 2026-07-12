package util

import (
	"context"
	"sync"
	"time"
)

// Debouncer batches incoming items and flushes them at specified intervals.
type Debouncer[T any] struct {
	items    []T
	interval int64
	onFlush  func([]T, string)
	ticker   *time.Ticker

	// Delay before the first flush occurs
	firstFlushDelay int64
	firstFlushTimer *time.Timer
	firstFlushed    bool
	flushedItems    bool
	done            bool

	cancel  context.CancelFunc
	m       sync.Mutex
	flushMu sync.Mutex
}

func NewDebouncer[T any](firstFlushDelay int64, interval int64, onFlush func([]T, string)) *Debouncer[T] {
	return &Debouncer[T]{
		firstFlushDelay: firstFlushDelay,
		interval:        interval,
		onFlush:         onFlush,
	}
}

func (r *Debouncer[T]) Start(ctx context.Context) {
	cancelCtx, cancelFunc := context.WithCancel(ctx)

	r.ticker = time.NewTicker(time.Duration(r.interval) * time.Millisecond)
	r.cancel = cancelFunc

	if r.firstFlushDelay >= 0 {
		r.firstFlushTimer = time.AfterFunc(time.Duration(r.firstFlushDelay)*time.Millisecond, func() {
			r.m.Lock()
			shouldFlush := false
			if !r.firstFlushed {
				r.firstFlushed = true
				shouldFlush = true
			}
			r.m.Unlock()
			if shouldFlush {
				r.flush(ctx, "first")
			}
		})
	}

	go func() {
		for {
			select {
			case <-r.ticker.C:
				r.m.Lock()
				shouldFlush := r.firstFlushed || r.firstFlushDelay == 0
				r.m.Unlock()
				if shouldFlush {
					r.flush(ctx, "tick")
				}
			case <-cancelCtx.Done():
				r.ticker.Stop()
				if r.firstFlushTimer != nil {
					r.firstFlushTimer.Stop()
				}
				return
			}
		}
	}()
}

func (r *Debouncer[T]) Add(ctx context.Context, result []T) {
	r.m.Lock()
	r.items = append(r.items, result...)
	shouldFlushFirstItems := r.firstFlushed && !r.flushedItems && !r.done && len(r.items) > 0
	r.m.Unlock()

	// If the deadline fired while the queue was empty, do not make the first
	// real result wait for the next periodic tick.
	if shouldFlushFirstItems {
		r.flush(ctx, "ready")
	}
}

func (r *Debouncer[T]) Done(ctx context.Context) {
	r.m.Lock()
	r.done = true
	r.firstFlushed = true
	if r.firstFlushTimer != nil {
		r.firstFlushTimer.Stop()
	}
	r.m.Unlock()

	r.cancel()
	r.flush(ctx, "done")
}

// FlushNow cancels the first-flush timer and serially flushes all currently queued items.
func (r *Debouncer[T]) FlushNow(ctx context.Context, reason string) {
	r.m.Lock()
	r.firstFlushed = true
	if r.firstFlushTimer != nil {
		r.firstFlushTimer.Stop()
	}
	r.m.Unlock()

	r.flush(ctx, reason)
}

func (r *Debouncer[T]) flush(_ context.Context, reason string) {
	// Keep callbacks ordered while allowing Add to continue during snapshot
	// building, serialization, or any other work performed by onFlush.
	r.flushMu.Lock()
	defer r.flushMu.Unlock()

	r.m.Lock()
	if r.done && reason != "done" {
		r.m.Unlock()
		return
	}
	flushedResults := r.items
	r.items = nil
	if len(flushedResults) > 0 {
		r.flushedItems = true
	}
	r.m.Unlock()

	if len(flushedResults) == 0 {
		// we still need to notify the reason even there is no item
		// user may want to know the "done" event
		r.onFlush([]T{}, reason)
		return
	}

	r.onFlush(flushedResults, reason)
}
