package filesearch

import (
	"context"
	"fmt"
	"sync"
	"time"

	"wox/util"
)

// serviceChangeFeed consumes the existing filename index stream. Uncovered or
// disconnected volumes keep a local subtree watch until the index catches up.
type serviceChangeFeed struct {
	mu       sync.Mutex
	broker   *nativeChangeBroker
	sub      *serviceChangeSubscription
	fallback ChangeFeed
	roots    []RootRecord
	covered  map[string]bool
	done     chan struct{}
	stopped  chan struct{}
	closed   bool
}

func newConsumerChangeFeed() ChangeFeed {
	return newServiceChangeFeed(sharedNativeChanges, newWindowsTreeChangeFeed())
}

// newServiceChangeFeed accepts isolated sources for lifecycle tests.
func newServiceChangeFeed(broker *nativeChangeBroker, fallback ChangeFeed) *serviceChangeFeed {
	f := &serviceChangeFeed{broker: broker, fallback: fallback, covered: map[string]bool{}, done: make(chan struct{}), stopped: make(chan struct{}), sub: &serviceChangeSubscription{queue: newChangeSignalQueue(256), wake: make(chan struct{}, 1)}}
	broker.mu.Lock()
	broker.feeds[f.sub] = struct{}{}
	broker.mu.Unlock()
	go f.run()
	return f
}

func (f *serviceChangeFeed) Mode() string                 { return "index-service+subtree-fallback" }
func (f *serviceChangeFeed) Signals() <-chan ChangeSignal { return f.sub.queue.Signals() }

// Refresh installs subscription roots before choosing fallback coverage.
func (f *serviceChangeFeed) Refresh(ctx context.Context, roots []RootRecord) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return nil
	}
	f.roots = append([]RootRecord(nil), roots...)
	f.broker.mu.Lock()
	f.sub.roots = f.roots
	f.broker.mu.Unlock()
	return f.refreshLocked(ctx)
}

// refreshLocked reconciles both directions and reports any transition gap.
func (f *serviceChangeFeed) refreshLocked(ctx context.Context) error {
	var fallback []RootRecord
	coverage := make(map[string]bool, len(f.roots))
	f.broker.mu.Lock()
	for _, root := range f.roots {
		covered := f.broker.coveredLocked(root.Path)
		if !covered {
			fallback = append(fallback, root)
		}
		previous, known := f.covered[root.ID]
		if !known || previous != covered {
			f.sub.queue.offer(ChangeSignal{Kind: ChangeSignalKindHistoryLost, RootID: root.ID, Path: root.Path, PathIsDir: true, PathTypeKnown: true, FeedType: RootFeedTypeUSN, Reason: "change source switched", At: time.Now()})
		}
		if !known || previous != covered {
			util.GetLogger().Info(ctx, fmt.Sprintf("filesearch consumer source changed: root=%s path=%s service=%t", root.ID, root.Path, covered))
		}
		coverage[root.ID] = covered
	}
	f.broker.mu.Unlock()
	f.covered = coverage
	return f.fallback.Refresh(ctx, fallback)
}

// run keeps lifecycle changes separate from the index's nonblocking publisher.
func (f *serviceChangeFeed) run() {
	defer close(f.stopped)
	for {
		select {
		case <-f.done:
			return
		case <-f.sub.wake:
			f.mu.Lock()
			if !f.closed {
				_ = f.refreshLocked(context.Background())
			}
			f.mu.Unlock()
		case signal, ok := <-f.fallback.Signals():
			if !ok {
				return
			}
			f.sub.queue.offer(signal)
		}
	}
}

// SnapshotRootFeed exposes current coverage to periodic recovery consumers.
func (f *serviceChangeFeed) SnapshotRootFeed(_ context.Context, root RootRecord) (RootFeedSnapshot, error) {
	f.broker.mu.Lock()
	defer f.broker.mu.Unlock()
	kind := RootFeedTypeFallback
	if f.broker.coveredLocked(root.Path) {
		kind = RootFeedTypeUSN
	}
	return RootFeedSnapshot{FeedType: kind, FeedState: RootFeedStateReady}, nil
}

// Close detaches before releasing the queue and joins the lifecycle worker.
func (f *serviceChangeFeed) Close() error {
	f.mu.Lock()
	if f.closed {
		f.mu.Unlock()
		return nil
	}
	f.closed = true
	f.broker.mu.Lock()
	delete(f.broker.feeds, f.sub)
	f.broker.mu.Unlock()
	close(f.done)
	f.mu.Unlock()
	<-f.stopped
	err := f.fallback.Close()
	f.sub.queue.close()
	return err
}
