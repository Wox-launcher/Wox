package filesearch

import "github.com/fsnotify/fsnotify"

// fallbackNativeWatcher separates recursive registration policy from ownership
// of native directory handles. Linux subscriptions share one inotify instance.
type fallbackNativeWatcher interface {
	Add(string) error
	Remove(string) error
	Close() error
	Events() <-chan fsnotify.Event
	Errors() <-chan error
}
