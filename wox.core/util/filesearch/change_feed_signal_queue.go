package filesearch

import (
	"sync"
	"time"
)

// changeSignalQueue is the delivery channel shared by every platform change
// feed. Native callbacks and watcher loops must never block on the scanner, so a
// full channel cannot simply wait. Dropping the signal silently (the old Windows
// and Linux behavior) left the index stale forever, while escalating it to a
// root reconcile (the old macOS behavior) re-crawled whole roots during every
// burst. The queue instead converts loss into one history-lost marker per root,
// delivered by a helper goroutine even when no further event arrives, and the
// scanner repairs it with its directory relist recovery.
type changeSignalQueue struct {
	signals   chan ChangeSignal
	done      chan struct{}
	wake      chan struct{}
	closeOnce sync.Once

	mu sync.Mutex
	// lost holds undelivered loss markers by root. While a root has one, its
	// later signals are dropped: the recovery triggered by the marker relists
	// directories after those changes happened and observes them anyway.
	lost map[string]ChangeSignal
}

func newChangeSignalQueue(size int) *changeSignalQueue {
	queue := &changeSignalQueue{
		signals: make(chan ChangeSignal, size),
		done:    make(chan struct{}),
		wake:    make(chan struct{}, 1),
		lost:    make(map[string]ChangeSignal),
	}
	go queue.deliverLossMarkers()
	return queue
}

func (q *changeSignalQueue) Signals() <-chan ChangeSignal {
	return q.signals
}

// offer delivers signal without blocking. It reports false when the signal was
// not delivered and is covered by a pending loss marker for its root instead.
func (q *changeSignalQueue) offer(signal ChangeSignal) bool {
	if signal.RootID == "" {
		return true
	}
	if signal.At.IsZero() {
		signal.At = time.Now()
	}

	q.mu.Lock()
	defer q.mu.Unlock()
	select {
	case <-q.done:
		return true
	default:
	}

	if pending, ok := q.lost[signal.RootID]; ok {
		// An unavailable feed is a stronger fact than a loss marker; let it
		// replace the marker so the scanner still records the feed state.
		if signal.Kind == ChangeSignalKindFeedUnavailable && pending.Kind != ChangeSignalKindFeedUnavailable {
			q.lost[signal.RootID] = signal
		}
		return false
	}

	select {
	case q.signals <- signal:
		return true
	default:
	}

	q.lost[signal.RootID] = newQueueLossMarker(signal)
	select {
	case q.wake <- struct{}{}:
	default:
	}
	return false
}

func (q *changeSignalQueue) close() {
	q.closeOnce.Do(func() {
		close(q.done)
	})
}

// deliverLossMarkers blocks on the consumer so markers are never lost, keeping
// each marker in place until it was handed over.
func (q *changeSignalQueue) deliverLossMarkers() {
	for {
		select {
		case <-q.done:
			return
		case <-q.wake:
		}

		for {
			q.mu.Lock()
			var marker ChangeSignal
			found := false
			for _, pending := range q.lost {
				marker = pending
				found = true
				break
			}
			q.mu.Unlock()
			if !found {
				break
			}

			select {
			case q.signals <- marker:
			case <-q.done:
				return
			}

			q.mu.Lock()
			if q.lost[marker.RootID] == marker {
				delete(q.lost, marker.RootID)
			}
			q.mu.Unlock()
		}
	}
}

// newQueueLossMarker keeps recovery signals as they are and turns ordinary path
// signals into history loss. The marker carries no cursor: a dropped signal must
// never let the scanner acknowledge feed progress beyond it.
func newQueueLossMarker(signal ChangeSignal) ChangeSignal {
	if signal.Kind == ChangeSignalKindHistoryLost || signal.Kind == ChangeSignalKindFeedUnavailable {
		return signal
	}
	return ChangeSignal{
		Kind:         ChangeSignalKindHistoryLost,
		SemanticKind: ChangeSemanticKindHistoryLost,
		RootID:       signal.RootID,
		FeedType:     signal.FeedType,
		Path:         signal.Path,
		Reason:       "change signal queue overflow",
		At:           signal.At,
	}
}
