package overlay

import "sync"

var closeCallbacks = map[string]func(){}
var closeCallbacksMu sync.Mutex

// RequestClose closes an overlay as a user action and fires its callback once.
func RequestClose(id string) {
	if callback := takeCloseCallback(id); callback != nil {
		callback()
	}
	Close(id)
}

// RegisterCloseCallback stores or clears the user-close handler for a window ID.
func RegisterCloseCallback(id string, onClose func()) {
	closeCallbacksMu.Lock()
	if onClose == nil {
		delete(closeCallbacks, id)
	} else {
		closeCallbacks[id] = onClose
	}
	closeCallbacksMu.Unlock()
}

func takeCloseCallback(id string) func() {
	closeCallbacksMu.Lock()
	callback := closeCallbacks[id]
	delete(closeCallbacks, id)
	closeCallbacksMu.Unlock()
	return callback
}
