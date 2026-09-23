package util

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/fsnotify/fsnotify"
)

const (
	// directoryWatchBufferBytes replaces fsnotify's 64 KB default ReadDirectoryChangesW buffer per
	// watched directory. Wox watches small directories (Start Menu, themes, plugin sources), so a
	// smaller buffer is enough; an overflow surfaces as fsnotify.ErrEventOverflow through OnError.
	// The option is a no-op on non-Windows backends.
	directoryWatchBufferBytes = 16 * 1024
	// directoryWatchQueueSize bounds how far one subscriber may fall behind before events are
	// dropped for it. Subscribers that block (the app index sleeps for a debounce window inside
	// its callback) must not stall delivery to every other directory, and an installer burst
	// has to fit in the queue while that sleep runs.
	directoryWatchQueueSize = 512
)

// DirectoryWatch is one subscription on a DirectoryWatcher. Close releases it and removes the
// native watch once no other subscriber needs the same directory.
type DirectoryWatch struct {
	owner     *DirectoryWatcher
	directory string
	onEvent   func(fsnotify.Event)
	onError   func(error)
	// queue keeps each subscriber on its own goroutine so ordering per subscriber is preserved
	// and a slow or panicking callback cannot affect the shared dispatch loop.
	queue  chan directoryWatchMessage
	closed bool
}

// directoryWatchMessage carries either an event or a watcher-level error to one subscriber.
type directoryWatchMessage struct {
	event fsnotify.Event
	err   error
}

// DirectoryWatcher multiplexes directory watches onto one fsnotify.Watcher. Each
// fsnotify.Watcher costs a completion port or inotify descriptor plus a goroutine blocked in a
// native wait, which on Windows pins an OS thread; Wox used to create one per feature. Most
// code uses the process-wide instance through WatchDirectory. Consumers that react to
// watcher-level errors by doing expensive work (the file search fallback feed reconciles its
// roots) own a separate instance so another feature's buffer overflow cannot trigger that.
type DirectoryWatcher struct {
	// mu guards the subscriber table only. It is never held across fsnotify calls: on Windows
	// AddWith and Remove wait for the fsnotify I/O goroutine, which may itself be blocked
	// handing an event to dispatch, and dispatch needs mu to enqueue that event.
	mu          sync.Mutex
	watcher     *fsnotify.Watcher
	subscribers map[string][]*DirectoryWatch
	// nativeMu serializes AddWith/Remove so a subscribe that decided "first subscriber" and an
	// unsubscribe that decided "last subscriber" reach fsnotify in the order they were decided.
	// It is acquired while mu is held and released after the native call; dispatch never takes it.
	nativeMu sync.Mutex
}

// NewDirectoryWatcher creates an isolated watcher with its own fsnotify instance.
func NewDirectoryWatcher() *DirectoryWatcher {
	return &DirectoryWatcher{subscribers: map[string][]*DirectoryWatch{}}
}

var sharedDirectoryWatcher = NewDirectoryWatcher()

// WatchDirectory subscribes to non-recursive change events for directory on the process-wide
// watcher. onError is optional; fsnotify reports watcher-level errors without a path, so every
// subscriber of the same watcher receives them. Callbacks run on a goroutine owned by the
// subscription and are invoked in event order.
func WatchDirectory(directory string, onEvent func(fsnotify.Event), onError func(error)) (*DirectoryWatch, error) {
	return sharedDirectoryWatcher.Watch(directory, onEvent, onError)
}

// Watch subscribes to non-recursive change events for directory on this watcher.
func (s *DirectoryWatcher) Watch(directory string, onEvent func(fsnotify.Event), onError func(error)) (*DirectoryWatch, error) {
	if onEvent == nil {
		return nil, fmt.Errorf("directory watch requires an event callback")
	}
	return s.subscribe(filepath.Clean(directory), onEvent, onError)
}

// Close releases every subscription and the native watcher. The watcher cannot be reused.
func (s *DirectoryWatcher) Close() error {
	s.mu.Lock()
	watcher := s.watcher
	s.watcher = nil
	var all []*DirectoryWatch
	for _, watches := range s.subscribers {
		all = append(all, watches...)
	}
	s.subscribers = map[string][]*DirectoryWatch{}
	for _, watch := range all {
		watch.closed = true
		close(watch.queue)
	}
	s.mu.Unlock()
	if watcher == nil {
		return nil
	}
	return watcher.Close()
}

// Directory returns the cleaned path this subscription watches.
func (w *DirectoryWatch) Directory() string {
	return w.directory
}

// Close stops delivering events to this subscription. It is safe to call more than once.
func (w *DirectoryWatch) Close() {
	if w == nil {
		return
	}
	w.owner.unsubscribe(w)
}

// run drains the subscriber queue until Close closes it.
func (w *DirectoryWatch) run() {
	for message := range w.queue {
		w.deliver(message)
	}
}

// deliver isolates callback panics so one subscriber cannot end delivery for the others.
func (w *DirectoryWatch) deliver(message directoryWatchMessage) {
	defer func() {
		if recovered := recover(); recovered != nil {
			GetLogger().Error(context.Background(), fmt.Sprintf("directory watch callback for %s panicked: %v", w.directory, recovered))
		}
	}()
	if message.err != nil {
		if w.onError != nil {
			w.onError(message.err)
		}
		return
	}
	w.onEvent(message.event)
}

func (s *DirectoryWatcher) subscribe(directory string, onEvent func(fsnotify.Event), onError func(error)) (*DirectoryWatch, error) {
	s.mu.Lock()
	if s.watcher == nil {
		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			s.mu.Unlock()
			return nil, err
		}
		s.watcher = watcher
		go s.dispatch(watcher)
	}
	watcher := s.watcher
	watch := &DirectoryWatch{owner: s, directory: directory, onEvent: onEvent, onError: onError, queue: make(chan directoryWatchMessage, directoryWatchQueueSize)}
	first := len(s.subscribers[directory]) == 0
	s.subscribers[directory] = append(s.subscribers[directory], watch)
	if !first {
		s.mu.Unlock()
		go watch.run()
		return watch, nil
	}
	s.nativeMu.Lock()
	s.mu.Unlock()
	err := watcher.AddWith(directory, fsnotify.WithBufferSize(directoryWatchBufferBytes))
	s.nativeMu.Unlock()
	if err != nil {
		s.mu.Lock()
		s.removeSubscriberLocked(watch)
		s.mu.Unlock()
		return nil, err
	}
	go watch.run()
	return watch, nil
}

func (s *DirectoryWatcher) unsubscribe(watch *DirectoryWatch) {
	s.mu.Lock()
	if watch.closed {
		s.mu.Unlock()
		return
	}
	watch.closed = true
	close(watch.queue)
	last := s.removeSubscriberLocked(watch)
	watcher := s.watcher
	if !last || watcher == nil {
		s.mu.Unlock()
		return
	}
	s.nativeMu.Lock()
	s.mu.Unlock()
	// The directory may already be gone, in which case fsnotify has dropped the watch itself.
	_ = watcher.Remove(watch.directory)
	s.nativeMu.Unlock()
}

// removeSubscriberLocked drops watch from the table and reports whether its directory has no
// subscribers left.
func (s *DirectoryWatcher) removeSubscriberLocked(watch *DirectoryWatch) bool {
	remaining := make([]*DirectoryWatch, 0, len(s.subscribers[watch.directory]))
	for _, candidate := range s.subscribers[watch.directory] {
		if candidate != watch {
			remaining = append(remaining, candidate)
		}
	}
	if len(remaining) > 0 {
		s.subscribers[watch.directory] = remaining
		return false
	}
	delete(s.subscribers, watch.directory)
	return true
}

// dispatch fans events out to subscribers of the directory that produced them. fsnotify watches
// are non-recursive, so an event names either a direct child of a watched directory or the
// watched directory itself; both lookups stay O(1) without prefix matching.
func (s *DirectoryWatcher) dispatch(watcher *fsnotify.Watcher) {
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			name := filepath.Clean(event.Name)
			s.enqueue(directoryWatchMessage{event: event}, filepath.Dir(name), name)
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			s.enqueueAll(directoryWatchMessage{err: err})
		}
	}
}

// enqueue hands a message to every subscriber of the given directories. Enqueueing happens under
// the lock so a subscription closed concurrently can never receive a send on its closed queue.
func (s *DirectoryWatcher) enqueue(message directoryWatchMessage, directories ...string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, directory := range directories {
		for _, watch := range s.subscribers[directory] {
			watch.offer(message)
		}
	}
}

func (s *DirectoryWatcher) enqueueAll(message directoryWatchMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, watches := range s.subscribers {
		for _, watch := range watches {
			watch.offer(message)
		}
	}
}

// offer never blocks the dispatch loop. A subscriber that has fallen a full queue behind loses
// the event and is told so through the same overflow error fsnotify uses.
func (w *DirectoryWatch) offer(message directoryWatchMessage) {
	select {
	case w.queue <- message:
		return
	default:
	}
	if message.err != nil {
		return
	}
	select {
	case w.queue <- directoryWatchMessage{err: fsnotify.ErrEventOverflow}:
	default:
	}
}

// WatchDirectoryChanges subscribes callback to directory on the shared watcher until ctx ends.
func WatchDirectoryChanges(ctx context.Context, directory string, callback func(event fsnotify.Event)) (*DirectoryWatch, error) {
	watch, err := WatchDirectory(directory, callback, func(watchErr error) {
		GetLogger().Error(ctx, fmt.Sprintf("failed to watch %s: %s", directory, watchErr.Error()))
	})
	if err != nil {
		GetLogger().Error(ctx, fmt.Sprintf("failed to watch directory %s: %s", directory, err.Error()))
		return nil, err
	}
	// Contexts without cancellation (trace contexts) never end, so skip the waiter for them.
	if ctx.Done() != nil {
		Go(ctx, "release directory watch", func() {
			<-ctx.Done()
			watch.Close()
		})
	}
	return watch, nil
}
