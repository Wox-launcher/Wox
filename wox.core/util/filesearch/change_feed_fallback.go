package filesearch

import (
	"context"
	"path/filepath"
	"sync"
	"time"

	"wox/util"

	"github.com/fsnotify/fsnotify"
)

type FallbackChangeFeed struct {
	mu sync.RWMutex
	// watcher is this feed's own DirectoryWatcher rather than the process-wide one. A
	// watcher-level error makes the feed reconcile every root it watches, which must not be
	// triggered by an overflow in an unrelated feature's directory. Refreshing roots swaps
	// subscriptions on this one instance instead of recreating fsnotify watchers.
	watcher     *util.DirectoryWatcher
	watches     []*util.DirectoryWatch
	roots       []RootRecord
	rootMatcher rootPathMatcher
	signals     chan ChangeSignal
	closed      bool
}

func NewFallbackChangeFeed() *FallbackChangeFeed {
	return &FallbackChangeFeed{
		signals: make(chan ChangeSignal, 128),
	}
}

func (f *FallbackChangeFeed) Mode() string {
	return "root-only"
}

func (f *FallbackChangeFeed) Signals() <-chan ChangeSignal {
	return f.signals
}

func (f *FallbackChangeFeed) Refresh(_ context.Context, roots []RootRecord) error {
	f.mu.Lock()
	if f.closed {
		f.mu.Unlock()
		return nil
	}
	if f.watcher == nil {
		f.watcher = util.NewDirectoryWatcher()
	}
	watcher := f.watcher
	f.mu.Unlock()

	watchedRoots := make([]RootRecord, 0, len(roots))
	watches := make([]*util.DirectoryWatch, 0, len(roots))
	for _, root := range roots {
		watch, err := watcher.Watch(root.Path, f.handleEvent, func(watchErr error) {
			// Watcher-level errors carry no path; the old private watcher reconciled every
			// root it owned, so each root subscription reports itself the same way.
			f.emit(ChangeSignal{
				Kind:         ChangeSignalKindRequiresRootReconcile,
				SemanticKind: ChangeSemanticKindRequiresRootReconcile,
				RootID:       root.ID,
				FeedType:     RootFeedTypeFallback,
				Path:         root.Path,
				Reason:       watchErr.Error(),
				At:           time.Now(),
			})
		})
		if err != nil {
			f.emit(ChangeSignal{
				Kind:         ChangeSignalKindFeedUnavailable,
				SemanticKind: ChangeSemanticKindFeedUnavailable,
				RootID:       root.ID,
				FeedType:     RootFeedTypeFallback,
				Path:         root.Path,
				Reason:       err.Error(),
				At:           time.Now(),
			})
			continue
		}

		watchedRoots = append(watchedRoots, root)
		watches = append(watches, watch)
		if root.FeedState == RootFeedStateUnavailable {
			f.emit(ChangeSignal{
				Kind:         ChangeSignalKindRequiresRootReconcile,
				SemanticKind: ChangeSemanticKindRequiresRootReconcile,
				RootID:       root.ID,
				FeedType:     RootFeedTypeFallback,
				Path:         root.Path,
				Reason:       "fallback change feed recovered",
				At:           time.Now(),
			})
		}
	}

	f.mu.Lock()
	if f.closed {
		f.mu.Unlock()
		closeDirectoryWatches(watches)
		return nil
	}
	oldWatches := f.watches
	f.watches = watches
	f.roots = append([]RootRecord(nil), watchedRoots...)
	// Optimization: fallback fsnotify can be a Linux primary feed and a Windows
	// fallback. Keep the root matcher in the same critical section as roots so
	// each event sees a consistent immutable snapshot without copying roots.
	f.rootMatcher = newRootPathMatcher(watchedRoots)
	f.mu.Unlock()

	// New roots are subscribed before old ones are released so a refresh never has a gap.
	closeDirectoryWatches(oldWatches)
	return nil
}

func (f *FallbackChangeFeed) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.closed {
		return nil
	}
	f.closed = true
	f.watches = nil
	f.roots = nil
	f.rootMatcher = rootPathMatcher{}
	if f.watcher == nil {
		return nil
	}
	watcher := f.watcher
	f.watcher = nil
	return watcher.Close()
}

func closeDirectoryWatches(watches []*util.DirectoryWatch) {
	for _, watch := range watches {
		watch.Close()
	}
}

func (f *FallbackChangeFeed) SnapshotRootFeed(ctx context.Context, root RootRecord) (RootFeedSnapshot, error) {
	_ = ctx
	_ = root
	return RootFeedSnapshot{
		FeedType:   RootFeedTypeFallback,
		FeedCursor: "",
		FeedState:  RootFeedStateReady,
	}, nil
}

func (f *FallbackChangeFeed) handleEvent(event fsnotify.Event) {
	f.mu.RLock()
	matcher := f.rootMatcher
	f.mu.RUnlock()
	if matcher.empty() {
		return
	}

	cleanPath := filepath.Clean(event.Name)
	root, ok := matcher.findClean(cleanPath)
	if !ok {
		return
	}

	cleanRootPath := filepath.Clean(root.Path)
	pathIsDir, pathTypeKnown := statPathType(cleanPath)
	if shouldSkipSystemPathForRoot(root, cleanPath, pathIsDir) {
		// Bug fix: fallback feeds are used on Linux and as a Windows fallback, so
		// internal Wox storage must be filtered here too. Dropping it before signal
		// creation keeps ~/.wox/filesearch writes from waking the dirty queue.
		return
	}
	semanticKind := classifyFallbackSemanticKind(event)

	kind := ChangeSignalKindDirtyPath
	if cleanPath == cleanRootPath {
		kind = ChangeSignalKindDirtyRoot
	} else if filepath.Dir(cleanPath) == cleanRootPath && !pathTypeKnown && (semanticKind == ChangeSemanticKindRemove || semanticKind == ChangeSemanticKindRename) {
		// Direct children used to force a root reconcile for every create/write.
		// Keep root scope only when a removed direct child is already gone, because
		// that is the one case where fallback fsnotify cannot tell whether stale
		// recursive rows may live under the deleted path.
		kind = ChangeSignalKindDirtyRoot
	}

	f.emit(ChangeSignal{
		Kind:          kind,
		SemanticKind:  semanticKind,
		RootID:        root.ID,
		FeedType:      RootFeedTypeFallback,
		Path:          cleanPath,
		PathIsDir:     pathIsDir,
		PathTypeKnown: pathTypeKnown,
		At:            time.Now(),
	})
}

func classifyFallbackSemanticKind(event fsnotify.Event) ChangeSemanticKind {
	switch {
	case event.Op&fsnotify.Rename != 0:
		return ChangeSemanticKindRename
	case event.Op&fsnotify.Remove != 0:
		return ChangeSemanticKindRemove
	case event.Op&fsnotify.Create != 0:
		return ChangeSemanticKindCreate
	case event.Op&fsnotify.Write != 0:
		return ChangeSemanticKindModify
	case event.Op&fsnotify.Chmod != 0:
		return ChangeSemanticKindMetadata
	default:
		return ChangeSemanticKindUnknown
	}
}

func (f *FallbackChangeFeed) emit(signal ChangeSignal) {
	if signal.RootID == "" {
		return
	}
	if signal.At.IsZero() {
		signal.At = time.Now()
	}

	f.mu.RLock()
	closed := f.closed
	f.mu.RUnlock()
	if closed {
		return
	}

	select {
	case f.signals <- signal:
	default:
		// Keep the fallback feed lossy rather than blocking the watcher loop.
	}
}

func findRootForPathInRoots(roots []RootRecord, path string) (RootRecord, bool) {
	cleanPath := filepath.Clean(path)
	bestIndex := -1
	bestLength := -1
	for index, root := range roots {
		if !pathWithinScope(root.Path, cleanPath) {
			continue
		}
		if len(root.Path) <= bestLength {
			continue
		}
		bestIndex = index
		bestLength = len(root.Path)
	}

	if bestIndex < 0 {
		return RootRecord{}, false
	}

	return roots[bestIndex], true
}
