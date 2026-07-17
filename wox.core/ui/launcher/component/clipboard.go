package component

import "sync"

// ClipboardProvider supplies cross-platform text clipboard access to text fields
// without pulling native clipboard dependencies into the lower UI layers.
type ClipboardProvider interface {
	// ReadText returns the current clipboard text, or an error if unavailable.
	ReadText() (string, error)
	// WriteText replaces the clipboard text.
	WriteText(text string) error
}

var (
	clipboardMu       sync.RWMutex
	clipboardProvider ClipboardProvider
)

// SetClipboardProvider registers the clipboard backend used by text fields for copy/cut/paste.
// Calling it more than once replaces the previous provider.
func SetClipboardProvider(provider ClipboardProvider) {
	clipboardMu.Lock()
	defer clipboardMu.Unlock()
	clipboardProvider = provider
}

func currentClipboard() ClipboardProvider {
	clipboardMu.RLock()
	defer clipboardMu.RUnlock()
	return clipboardProvider
}
