package filesearch

import "sync"

// nativeChangeBroker shares already-resolved index changes with lightweight
// consumers. It owns neither another index nor another native fact stream.
type nativeChangeBroker struct {
	mu      sync.Mutex
	volumes map[string]bool
	feeds   map[*serviceChangeSubscription]struct{}
}

type serviceChangeSubscription struct {
	roots []RootRecord
	queue *changeSignalQueue
	wake  chan struct{}
}

var sharedNativeChanges = &nativeChangeBroker{volumes: map[string]bool{}, feeds: map[*serviceChangeSubscription]struct{}{}}

// setAvailable switches consumers to fallback on failures, builds and shutdown.
func (b *nativeChangeBroker) setAvailable(volume string, available bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.volumes[volume] == available {
		return
	}
	b.volumes[volume] = available
	for feed := range b.feeds {
		select {
		case feed.wake <- struct{}{}:
		default:
		}
	}
}

// coveredLocked requires an active, caught-up source, not merely an installed service.
func (b *nativeChangeBroker) coveredLocked(path string) bool {
	for volume, available := range b.volumes {
		if available && pathWithinScope(volume, path) {
			return true
		}
	}
	return false
}

// publish fans out bounded signals only to roots interested in the changed path.
func (b *nativeChangeBroker) publish(signal ChangeSignal) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for feed := range b.feeds {
		for _, root := range feed.roots {
			if pathWithinScope(root.Path, signal.Path) {
				copy := signal
				copy.RootID = root.ID
				feed.queue.offer(copy)
			} else if signal.PathIsDir && pathWithinScope(signal.Path, root.Path) {
				feed.queue.offer(ChangeSignal{Kind: ChangeSignalKindHistoryLost, RootID: root.ID, Path: root.Path, FeedType: RootFeedTypeUSN, Reason: "ancestor changed"})
			}
		}
	}
}
