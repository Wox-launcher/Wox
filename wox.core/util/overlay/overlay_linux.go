package overlay

import "sync"

var closeCallbacks = make(map[string]func())
var closeCallbacksMu sync.RWMutex

func Show(opts OverlayOptions) {
	// Stub implementation for Linux. Register the close callback so the
	// API contract matches other platforms even though overlay is not
	// natively supported here yet.
	if opts.OnClose != nil {
		closeCallbacksMu.Lock()
		closeCallbacks[opts.Name] = opts.OnClose
		closeCallbacksMu.Unlock()
	} else {
		closeCallbacksMu.Lock()
		delete(closeCallbacks, opts.Name)
		closeCallbacksMu.Unlock()
	}
}

func Close(name string) {
	// Stub implementation for Linux
	closeCallbacksMu.Lock()
	delete(closeCallbacks, name)
	closeCallbacksMu.Unlock()
}
