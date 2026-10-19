package filesearch

import (
	"path/filepath"
	"sync"
	"syscall"

	"github.com/fsnotify/fsnotify"
)

// fallbackWatchPool charges one shared fallback budget per unique directory. A
// subscriber owns one bounded delivery queue, not a queue or goroutine per path.
// The portable implementation can be tested without a Linux desktop session.
type fallbackWatchPool struct {
	opMu    sync.Mutex
	mu      sync.Mutex
	watcher *fsnotify.Watcher
	budget  int
	paths   map[string]map[*pooledFallbackWatcher]struct{}
	clients map[*pooledFallbackWatcher]struct{}
}

type pooledFallbackWatcher struct {
	pool   *fallbackWatchPool
	paths  map[string]struct{}
	events chan fsnotify.Event
	errors chan error
	closed bool
}

func newFallbackWatchPool(budget int) *fallbackWatchPool {
	return &fallbackWatchPool{budget: budget, paths: map[string]map[*pooledFallbackWatcher]struct{}{}, clients: map[*pooledFallbackWatcher]struct{}{}}
}

// subscribe lazily opens one native instance and gives each feed bounded delivery.
func (p *fallbackWatchPool) subscribe() (*pooledFallbackWatcher, error) {
	p.opMu.Lock()
	defer p.opMu.Unlock()
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.watcher == nil {
		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			return nil, err
		}
		p.watcher = watcher
		go p.dispatch(watcher)
	}
	w := &pooledFallbackWatcher{pool: p, paths: map[string]struct{}{}, events: make(chan fsnotify.Event, 256), errors: make(chan error, 1)}
	p.clients[w] = struct{}{}
	return w, nil
}

func (w *pooledFallbackWatcher) Events() <-chan fsnotify.Event { return w.events }
func (w *pooledFallbackWatcher) Errors() <-chan error          { return w.errors }

// Add references an existing watch without charging it again. Re-adding to the
// backend also restores a watch automatically removed after a directory rename.
func (w *pooledFallbackWatcher) Add(path string) error {
	path = filepath.Clean(path)
	p := w.pool
	p.opMu.Lock()
	defer p.opMu.Unlock()
	p.mu.Lock()
	if w.closed {
		p.mu.Unlock()
		return fsnotify.ErrClosed
	}
	if len(p.paths[path]) == 0 && len(p.paths) >= p.budget {
		p.mu.Unlock()
		return syscall.ENOSPC
	}
	watcher := p.watcher
	p.mu.Unlock()
	if err := watcher.Add(path); err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.paths[path] == nil {
		p.paths[path] = map[*pooledFallbackWatcher]struct{}{}
	}
	p.paths[path][w] = struct{}{}
	w.paths[path] = struct{}{}
	return nil
}

// Remove releases only this feed's reference; other feeds keep receiving events.
func (w *pooledFallbackWatcher) Remove(path string) error {
	path = filepath.Clean(path)
	p := w.pool
	p.opMu.Lock()
	defer p.opMu.Unlock()
	p.mu.Lock()
	delete(w.paths, path)
	delete(p.paths[path], w)
	last := len(p.paths[path]) == 0
	if last {
		delete(p.paths, path)
	}
	watcher := p.watcher
	p.mu.Unlock()
	if last && watcher != nil {
		return watcher.Remove(path)
	}
	return nil
}

// Close drops references before closing channels, then releases the last native
// instance outside the dispatch lock so shutdown cannot deadlock event delivery.
func (w *pooledFallbackWatcher) Close() error {
	p := w.pool
	p.opMu.Lock()
	defer p.opMu.Unlock()
	p.mu.Lock()
	if w.closed {
		p.mu.Unlock()
		return nil
	}
	w.closed = true
	var removed []string
	for path := range w.paths {
		delete(p.paths[path], w)
		if len(p.paths[path]) == 0 {
			delete(p.paths, path)
			removed = append(removed, path)
		}
	}
	w.paths = nil
	delete(p.clients, w)
	close(w.events)
	close(w.errors)
	watcher := p.watcher
	last := len(p.clients) == 0
	if last {
		p.watcher = nil
	}
	p.mu.Unlock()
	if last {
		return watcher.Close()
	}
	for _, path := range removed {
		_ = watcher.Remove(path)
	}
	return nil
}

// dispatch never waits for a consumer. A full queue always leaves an overflow
// error pending on a separate channel, even if no further native event arrives.
func (p *fallbackWatchPool) dispatch(watcher *fsnotify.Watcher) {
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			p.deliver(event)
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			p.mu.Lock()
			for client := range p.clients {
				select {
				case client.errors <- err:
				default:
				}
			}
			p.mu.Unlock()
		}
	}
}

// deliver looks up only a changed path and its parent, independent of watch count.
func (p *fallbackWatchPool) deliver(event fsnotify.Event) {
	p.mu.Lock()
	defer p.mu.Unlock()
	path := filepath.Clean(event.Name)
	parent := filepath.Dir(path)
	for client := range p.paths[parent] {
		client.offer(event)
	}
	if parent == path {
		return
	}
	for client := range p.paths[path] {
		if _, already := p.paths[parent][client]; !already {
			client.offer(event)
		}
	}
}

// offer reserves a separate error slot so a stalled consumer cannot hide loss.
func (w *pooledFallbackWatcher) offer(event fsnotify.Event) {
	select {
	case w.events <- event:
	default:
		select {
		case w.errors <- fsnotify.ErrEventOverflow:
		default:
		}
	}
}
