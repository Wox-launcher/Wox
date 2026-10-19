//go:build !linux

package filesearch

import "github.com/fsnotify/fsnotify"

type standaloneFallbackWatcher struct{ watcher *fsnotify.Watcher }

// newFallbackNativeWatcher keeps non-Linux explicit fallback users isolated.
func newFallbackNativeWatcher() (fallbackNativeWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	return &standaloneFallbackWatcher{watcher: watcher}, nil
}

func (w *standaloneFallbackWatcher) Add(path string) error {
	return w.watcher.AddWith(path, fsnotify.WithBufferSize(fallbackWatchBufferBytes))
}
func (w *standaloneFallbackWatcher) Remove(path string) error      { return w.watcher.Remove(path) }
func (w *standaloneFallbackWatcher) Close() error                  { return w.watcher.Close() }
func (w *standaloneFallbackWatcher) Events() <-chan fsnotify.Event { return w.watcher.Events }
func (w *standaloneFallbackWatcher) Errors() <-chan error          { return w.watcher.Errors }
