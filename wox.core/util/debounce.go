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

	cancel context.CancelFunc
	m      sync.Mutex
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

	if r.firstFlushDelay > 0 {
		r.firstFlushTimer = time.AfterFunc(time.Duration(r.firstFlushDelay)*time.Millisecond, func() {
			r.m.Lock()
			if !r.firstFlushed {
				r.firstFlushed = true
				r.m.Unlock()
				r.flush(ctx, "first")
			} else {
				r.m.Unlock()
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
	defer r.m.Unlock()

	r.items = append(r.items, result...)
}

func (r *Debouncer[T]) Done(ctx context.Context) {
	r.cancel()
	r.flush(ctx, "done")
}

func (r *Debouncer[T]) flush(_ context.Context, reason string) {
	r.m.Lock()
	defer r.m.Unlock()

	if len(r.items) == 0 {
		// we still need to notify the reason even there is no item
		// user may want to know the "done" event
		r.onFlush([]T{}, reason)
		return
	}

	flushedResults := r.items
	r.items = nil
	r.onFlush(flushedResults, reason)
}
