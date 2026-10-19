package filesearch

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"wox/util"

	"github.com/fsnotify/fsnotify"
)

const (
	fallbackSignalBufferSize = 8192
	// fallbackWatchBufferBytes only applies to ReadDirectoryChangesW. Each watched
	// directory keeps one buffer, so the Windows fallback stays far below the
	// fsnotify default of 64 KB.
	fallbackWatchBufferBytes = 16 * 1024
	// fallbackPartialRootRelistInterval bounds how stale a directory left
	// unwatched by the watch budget can become before a relist repairs it.
	fallbackPartialRootRelistInterval = 10 * time.Minute
)

// FallbackChangeFeed watches roots with fsnotify. It is the primary feed on
// Linux. Windows production feeds use native subtree watches instead.
//
// fsnotify watches are not recursive. The previous feed only watched each root
// directory, so changes below the first level were never reported and the index
// silently went stale until the next full reconcile. The feed now registers one
// watch per directory, bounded by a platform watch budget. Roots that do not
// fit the budget are marked partial and get a periodic history-lost signal, so
// the scanner's directory relist repairs whatever the missing watches did not
// report.
type FallbackChangeFeed struct {
	mu          sync.Mutex
	watcher     fallbackNativeWatcher
	roots       []RootRecord
	rootMatcher rootPathMatcher
	filter      DirectoryWatchFilter
	// watched holds every directory with a native watch. fsnotify keys watches
	// by path, so a renamed directory keeps reporting its old path until the
	// watch is removed and registered again under the new name.
	watched map[string]struct{}
	budget  int
	// partialRoots have directories that exceed the watch budget.
	partialRoots map[string]bool
	// maintenance is processed by one worker goroutine. Native Add/Remove calls
	// never run on the event dispatch goroutine or under mu: on Windows they
	// wait for the fsnotify I/O goroutine, which may be blocked handing an event
	// to dispatch.
	maintenance []fallbackWatchTask
	wake        chan struct{}
	done        chan struct{}
	queue       *changeSignalQueue
	closed      bool
}

// fallbackWatchTask is one unit of watch maintenance.
type fallbackWatchTask struct {
	// removePaths drops native watches for directories that were removed or renamed.
	removePaths []string
	// registerPath starts a breadth-first watch registration below this directory.
	registerPath string
	// notifyRootID asks for a dirty signal on registerPath after registration, so
	// entries created before the new watches existed are rescanned.
	notifyRootID string
	// initialRootID marks the first registration of a root. Changes made before
	// its watches existed are unknown, so the root reports history loss.
	initialRootID  string
	rootWatchAdded bool
}

func NewFallbackChangeFeed() *FallbackChangeFeed {
	return newFallbackChangeFeed(fallbackSignalBufferSize)
}

// newFallbackChangeFeed sizes the bounded delivery queue for its consumer.
func newFallbackChangeFeed(bufferSize int) *FallbackChangeFeed {
	return &FallbackChangeFeed{
		watched:      map[string]struct{}{},
		budget:       fallbackWatchBudget(),
		partialRoots: map[string]bool{},
		wake:         make(chan struct{}, 1),
		done:         make(chan struct{}),
		queue:        newChangeSignalQueue(bufferSize),
	}
}

func (f *FallbackChangeFeed) Mode() string {
	return "recursive-fsnotify"
}

func (f *FallbackChangeFeed) Signals() <-chan ChangeSignal {
	return f.queue.Signals()
}

func (f *FallbackChangeFeed) SetDirectoryWatchFilter(filter DirectoryWatchFilter) {
	f.mu.Lock()
	f.filter = filter
	f.mu.Unlock()
}

func (f *FallbackChangeFeed) Refresh(_ context.Context, roots []RootRecord) error {
	f.mu.Lock()
	if f.closed {
		f.mu.Unlock()
		return nil
	}
	if f.watcher == nil {
		watcher, err := newFallbackNativeWatcher()
		if err != nil {
			f.mu.Unlock()
			for _, root := range roots {
				f.emitFeedUnavailable(root, err.Error())
			}
			return nil
		}
		f.watcher = watcher
		go f.dispatch(watcher)
		go f.maintainWatches()
		go f.relistPartialRoots()
	}
	watcher := f.watcher

	previous := make(map[string]RootRecord, len(f.roots))
	for _, root := range f.roots {
		previous[root.ID] = root
	}
	f.roots = append([]RootRecord(nil), roots...)
	// Optimization: keep the root matcher in the same critical section as roots
	// so each event sees a consistent immutable snapshot without copying roots.
	f.rootMatcher = newRootPathMatcher(roots)

	stale := make([]string, 0)
	for directory := range f.watched {
		if _, ok := f.rootMatcher.findClean(directory); !ok {
			stale = append(stale, directory)
			delete(f.watched, directory)
		}
	}
	activeRootIDs := make(map[string]bool, len(roots))
	for _, root := range roots {
		activeRootIDs[root.ID] = true
	}
	for rootID := range f.partialRoots {
		if !activeRootIDs[rootID] {
			delete(f.partialRoots, rootID)
		}
	}
	f.mu.Unlock()

	if len(stale) > 0 {
		f.enqueueTask(fallbackWatchTask{removePaths: stale})
	}

	for _, root := range roots {
		if old, ok := previous[root.ID]; ok && filepath.Clean(old.Path) == filepath.Clean(root.Path) {
			continue
		}
		rootPath := filepath.Clean(root.Path)
		info, err := os.Stat(rootPath)
		if err == nil && !info.IsDir() {
			err = fmt.Errorf("root path %q is not a directory", rootPath)
		}
		if err != nil {
			f.emitFeedUnavailable(root, err.Error())
			continue
		}

		// The root watch is registered synchronously so a refresh never leaves
		// the root itself unobserved; descendants follow on the worker.
		f.mu.Lock()
		_, alreadyWatched := f.watched[rootPath]
		f.mu.Unlock()
		rootWatchAdded := false
		if !alreadyWatched {
			if err := watcher.Add(rootPath); err != nil {
				f.emitFeedUnavailable(root, err.Error())
				continue
			}
			f.mu.Lock()
			f.watched[rootPath] = struct{}{}
			f.mu.Unlock()
			rootWatchAdded = true
		}

		if root.FeedState == RootFeedStateUnavailable {
			f.emitHistoryLost(root, "fallback change feed recovered")
		}
		f.enqueueTask(fallbackWatchTask{registerPath: rootPath, initialRootID: root.ID, rootWatchAdded: rootWatchAdded})
	}
	return nil
}

func (f *FallbackChangeFeed) Close() error {
	f.mu.Lock()
	if f.closed {
		f.mu.Unlock()
		return nil
	}
	f.closed = true
	close(f.done)
	watcher := f.watcher
	f.watcher = nil
	f.roots = nil
	f.rootMatcher = rootPathMatcher{}
	f.watched = map[string]struct{}{}
	f.maintenance = nil
	f.mu.Unlock()

	f.queue.close()
	if watcher == nil {
		return nil
	}
	return watcher.Close()
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

func (f *FallbackChangeFeed) dispatch(watcher fallbackNativeWatcher) {
	for {
		select {
		case event, ok := <-watcher.Events():
			if !ok {
				return
			}
			f.handleEvent(event)
		case err, ok := <-watcher.Errors():
			if !ok {
				return
			}
			f.handleWatcherError(err)
		}
	}
}

// handleWatcherError reports history loss for every root. fsnotify errors carry
// no path, and a kernel queue overflow (IN_Q_OVERFLOW) may have dropped events
// anywhere below any watched directory.
func (f *FallbackChangeFeed) handleWatcherError(watchErr error) {
	f.mu.Lock()
	roots := append([]RootRecord(nil), f.roots...)
	f.mu.Unlock()
	for _, root := range roots {
		f.emitHistoryLost(root, watchErr.Error())
	}
}

func (f *FallbackChangeFeed) handleEvent(event fsnotify.Event) {
	cleanPath := filepath.Clean(event.Name)
	removedOrRenamed := event.Op&(fsnotify.Remove|fsnotify.Rename) != 0

	f.mu.Lock()
	root, ok := f.rootMatcher.findClean(cleanPath)
	if !ok {
		f.mu.Unlock()
		return
	}
	_, wasWatched := f.watched[cleanPath]
	filter := f.filter
	var forgotten []string
	if wasWatched && removedOrRenamed {
		forgotten = f.forgetWatchedTreeLocked(cleanPath)
	}
	f.mu.Unlock()
	if len(forgotten) > 0 {
		f.enqueueTask(fallbackWatchTask{removePaths: forgotten})
	}

	pathIsDir, pathTypeKnown := statPathType(cleanPath)
	if wasWatched && !pathTypeKnown {
		// The directory is already gone, but it had a watch, so it was a directory.
		pathIsDir, pathTypeKnown = true, true
	}
	if shouldSkipSystemPathForRoot(root, cleanPath, pathIsDir) {
		// Bug fix: fallback feeds are used on Linux and as a Windows fallback, so
		// internal Wox storage must be filtered here too. Dropping it before signal
		// creation keeps ~/.wox/filesearch writes from waking the dirty queue.
		return
	}

	if cleanPath == filepath.Clean(root.Path) {
		// Events on the root directory itself carry no child change; children
		// report their own events. A moved or deleted root cannot be followed.
		if removedOrRenamed {
			f.emitHistoryLost(root, "fallback root directory moved or removed")
		}
		return
	}

	if pathTypeKnown && pathIsDir && event.Op&fsnotify.Create != 0 && (filter == nil || filter(root, cleanPath)) {
		f.enqueueTask(fallbackWatchTask{registerPath: cleanPath, notifyRootID: root.ID})
	}

	// A removed path whose type is unknown keeps PathTypeKnown=false. Consumers
	// delete it by path, which covers both files and directories, instead of the
	// old whole-root reconcile used when a direct root child disappeared.
	f.queue.offer(ChangeSignal{
		Kind:          ChangeSignalKindDirtyPath,
		SemanticKind:  classifyFallbackSemanticKind(event),
		RootID:        root.ID,
		FeedType:      RootFeedTypeFallback,
		Path:          cleanPath,
		PathIsDir:     pathIsDir,
		PathTypeKnown: pathTypeKnown,
		At:            time.Now(),
	})
}

// forgetWatchedTreeLocked drops directory and its watched descendants from the
// bookkeeping and returns them for native removal.
func (f *FallbackChangeFeed) forgetWatchedTreeLocked(directory string) []string {
	forgotten := make([]string, 0, 1)
	for watched := range f.watched {
		if pathWithinScope(directory, watched) {
			forgotten = append(forgotten, watched)
			delete(f.watched, watched)
		}
	}
	return forgotten
}

func (f *FallbackChangeFeed) enqueueTask(task fallbackWatchTask) {
	f.mu.Lock()
	if f.closed {
		f.mu.Unlock()
		return
	}
	// Bound pending discoveries as well as installed watches. Native removals
	// must still drain; their paths are bounded by the existing watch budget.
	if task.registerPath != "" && len(f.maintenance) >= 1024 {
		root, ok := f.rootMatcher.findClean(task.registerPath)
		f.mu.Unlock()
		if ok {
			f.markPartialRoot(root, "watch maintenance queue full")
			f.emitHistoryLost(root, "watch maintenance queue overflow")
		}
		return
	}
	f.maintenance = append(f.maintenance, task)
	f.mu.Unlock()
	select {
	case f.wake <- struct{}{}:
	default:
	}
}

// maintainWatches runs watch maintenance tasks in arrival order, so removing a
// renamed directory always happens before a directory reusing its path is
// registered again.
func (f *FallbackChangeFeed) maintainWatches() {
	for {
		select {
		case <-f.done:
			return
		case <-f.wake:
		}
		for {
			f.mu.Lock()
			if f.closed || len(f.maintenance) == 0 {
				f.mu.Unlock()
				break
			}
			task := f.maintenance[0]
			f.maintenance[0] = fallbackWatchTask{}
			f.maintenance = f.maintenance[1:]
			if len(f.maintenance) == 0 {
				f.maintenance = nil
			}
			watcher := f.watcher
			f.mu.Unlock()
			if watcher == nil {
				break
			}

			for _, path := range task.removePaths {
				// fsnotify may already have dropped the watch of a deleted directory.
				_ = watcher.Remove(path)
			}
			if task.registerPath != "" {
				f.registerTree(watcher, task)
			}
		}
	}
}

// registerTree adds watches breadth first, so shallow user-visible directories
// win when the budget runs out.
func (f *FallbackChangeFeed) registerTree(watcher fallbackNativeWatcher, task fallbackWatchTask) {
	added := 0
	budgetExhausted := false
	queue := []string{task.registerPath}
	for len(queue) > 0 && !budgetExhausted {
		directory := queue[0]
		queue = queue[1:]

		f.mu.Lock()
		if f.closed {
			f.mu.Unlock()
			return
		}
		root, ok := f.rootMatcher.findClean(directory)
		_, alreadyWatched := f.watched[directory]
		budgetReached := len(f.watched) >= f.budget
		filter := f.filter
		f.mu.Unlock()
		if !ok {
			continue
		}
		isRootDirectory := directory == filepath.Clean(root.Path)
		if !isRootDirectory && filter != nil && !filter(root, directory) {
			continue
		}
		if alreadyWatched && directory != task.registerPath {
			// Another registration already owns this subtree.
			continue
		}

		if !alreadyWatched {
			if budgetReached {
				budgetExhausted = true
				f.markPartialRoot(root, "watch budget reached")
				break
			}
			if err := watcher.Add(directory); err != nil {
				if errors.Is(err, syscall.ENOSPC) || errors.Is(err, syscall.EMFILE) {
					// The kernel limit is lower than the configured budget (for
					// example, other applications hold inotify watches).
					f.mu.Lock()
					f.budget = len(f.watched)
					f.mu.Unlock()
					budgetExhausted = true
					f.markPartialRoot(root, err.Error())
					break
				}
				// Unreadable or already deleted directories cannot be indexed either.
				continue
			}
			f.mu.Lock()
			f.watched[directory] = struct{}{}
			f.mu.Unlock()
			added++
		}

		// Filter before queueing, and read in chunks so a wide excluded tree
		// cannot allocate an unbounded directory listing or traversal queue.
		dir, err := os.Open(directory)
		if err != nil {
			continue
		}
		for !budgetExhausted {
			entries, readErr := dir.ReadDir(256)
			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}
				child := filepath.Join(directory, entry.Name())
				if shouldSkipSystemPathForRoot(root, child, true) || filter != nil && !filter(root, child) {
					continue
				}
				if len(queue) >= f.budget {
					budgetExhausted = true
					f.markPartialRoot(root, "watch traversal budget reached")
					break
				}
				queue = append(queue, child)
			}
			if readErr != nil {
				break
			}
		}
		_ = dir.Close()
	}

	f.mu.Lock()
	root, rootFound := f.rootMatcher.findClean(task.registerPath)
	f.mu.Unlock()
	if !rootFound {
		return
	}
	if task.notifyRootID != "" && added > 0 {
		f.queue.offer(ChangeSignal{
			Kind:          ChangeSignalKindDirtyPath,
			SemanticKind:  ChangeSemanticKindCreate,
			RootID:        root.ID,
			FeedType:      RootFeedTypeFallback,
			Path:          task.registerPath,
			PathIsDir:     true,
			PathTypeKnown: true,
			At:            time.Now(),
		})
	}
	if task.initialRootID != "" && (task.rootWatchAdded || added > 0) {
		f.emitHistoryLost(root, "fallback watches registered")
	}
}

func (f *FallbackChangeFeed) markPartialRoot(root RootRecord, reason string) {
	f.mu.Lock()
	alreadyPartial := f.partialRoots[root.ID]
	f.partialRoots[root.ID] = true
	watchedCount := len(f.watched)
	f.mu.Unlock()
	if alreadyPartial {
		return
	}
	util.GetLogger().Warn(context.Background(), fmt.Sprintf(
		"filesearch fallback feed cannot watch every directory, relisting periodically: root=%s path=%s watched=%d reason=%s interval=%s",
		root.ID,
		root.Path,
		watchedCount,
		reason,
		fallbackPartialRootRelistInterval,
	))
}

// relistPartialRoots periodically reports history loss for roots whose deeper
// directories have no watch, letting the scanner relist changed directories.
func (f *FallbackChangeFeed) relistPartialRoots() {
	ticker := time.NewTicker(fallbackPartialRootRelistInterval)
	defer ticker.Stop()
	for {
		select {
		case <-f.done:
			return
		case <-ticker.C:
		}
		f.mu.Lock()
		partial := make([]RootRecord, 0, len(f.partialRoots))
		for _, root := range f.roots {
			if f.partialRoots[root.ID] {
				partial = append(partial, root)
			}
		}
		f.mu.Unlock()
		for _, root := range partial {
			f.emitHistoryLost(root, "periodic relist of directories beyond the watch budget")
		}
	}
}

func (f *FallbackChangeFeed) emitHistoryLost(root RootRecord, reason string) {
	f.queue.offer(ChangeSignal{
		Kind:          ChangeSignalKindHistoryLost,
		SemanticKind:  ChangeSemanticKindHistoryLost,
		RootID:        root.ID,
		FeedType:      RootFeedTypeFallback,
		Path:          root.Path,
		PathIsDir:     true,
		PathTypeKnown: true,
		Reason:        reason,
		At:            time.Now(),
	})
}

func (f *FallbackChangeFeed) emitFeedUnavailable(root RootRecord, reason string) {
	f.queue.offer(ChangeSignal{
		Kind:          ChangeSignalKindFeedUnavailable,
		SemanticKind:  ChangeSemanticKindFeedUnavailable,
		RootID:        root.ID,
		FeedType:      RootFeedTypeFallback,
		Path:          root.Path,
		PathIsDir:     true,
		PathTypeKnown: true,
		Reason:        reason,
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
