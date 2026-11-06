package launcher

import (
	"context"
	"sync"
	"time"

	"wox/plugin"
	"wox/util"
)

type woxViewLifecycleEvent struct {
	queryID  string
	resultID string
	document string
	kind     plugin.WoxViewPreviewEventType
	done     chan struct{}
}

// woxViewLifecycleQueue preserves visibility order without blocking the UI or dropping a hide on unmount.
type woxViewLifecycleQueue struct {
	mu      sync.Mutex
	pending []woxViewLifecycleEvent
	running bool
}

// enqueueWoxViewLifecycle drains transitions in order even after the view's event worker is canceled.
func (a *previewController) enqueueWoxViewLifecycle(result queryResult, document string, kind plugin.WoxViewPreviewEventType, done chan struct{}) {
	service, ok := a.services.(interface {
		DispatchWoxViewPreviewEvent(context.Context, string, string, string, string, plugin.WoxViewPreviewEvent) error
	})
	if !ok {
		if done != nil {
			close(done)
		}
		return
	}
	queue := &a.woxViewLifecycle
	queue.mu.Lock()
	queue.pending = append(queue.pending, woxViewLifecycleEvent{queryID: result.QueryID, resultID: result.ID, document: document, kind: kind, done: done})
	if queue.running {
		queue.mu.Unlock()
		return
	}
	queue.running = true
	queue.mu.Unlock()
	util.Go(context.Background(), "WoxView preview lifecycle", func() {
		for {
			queue.mu.Lock()
			if len(queue.pending) == 0 {
				queue.running = false
				queue.mu.Unlock()
				return
			}
			next := queue.pending[0]
			queue.pending[0] = woxViewLifecycleEvent{}
			queue.pending = queue.pending[1:]
			queue.mu.Unlock()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			err := service.DispatchWoxViewPreviewEvent(ctx, a.sessionID, next.queryID, next.resultID, next.document, plugin.WoxViewPreviewEvent{Type: next.kind})
			cancel()
			if next.done != nil {
				close(next.done)
			}
			if err != nil {
				util.GetLogger().Debug(context.Background(), "WoxView preview lifecycle delivery: "+err.Error())
			}
		}
	})
}
