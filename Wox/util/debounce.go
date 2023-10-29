package util

import (
	"context"
	"sync"
	"time"
)

type Debouncer[T any] struct {
	items    []T
	interval int64
	onFlush  func([]T, string)
	ticker   *time.Ticker
	cancel   context.CancelFunc
	m        sync.Mutex
}

func NewDebouncer[T any](interval int64, onFlush func([]T, string)) *Debouncer[T] {
	return &Debouncer[T]{
		interval: interval,
		onFlush:  onFlush,
	}
}

func (r *Debouncer[T]) Start(ctx context.Context) {
	cancelCtx, cancelFunc := context.WithCancel(ctx)

	r.ticker = time.NewTicker(time.Duration(r.interval) * time.Millisecond)
	r.cancel = cancelFunc

	go func() {
		for {
			select {
			case <-r.ticker.C:
				r.flush(ctx, "tick")
			case <-cancelCtx.Done():
				r.ticker.Stop()
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

func (r *Debouncer[T]) flush(ctx context.Context, reason string) {
	r.m.Lock()
	defer r.m.Unlock()

	if len(r.items) > 0 {
		flushedResults := r.items
		r.items = nil
		r.onFlush(flushedResults, reason)
	}
}
