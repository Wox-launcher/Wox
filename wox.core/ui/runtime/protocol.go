package woxui

import "sync"

var protocolURLDispatcher struct {
	sync.Mutex
	handler func(string)
	pending []string
}

// SetProtocolURLHandler installs the application-level handler and drains URLs received before startup finished.
func SetProtocolURLHandler(handler func(string)) {
	protocolURLDispatcher.Lock()
	protocolURLDispatcher.handler = handler
	if handler == nil {
		protocolURLDispatcher.Unlock()
		return
	}
	pending := append([]string(nil), protocolURLDispatcher.pending...)
	protocolURLDispatcher.pending = nil
	protocolURLDispatcher.Unlock()

	for _, rawURL := range pending {
		handler(rawURL)
	}
}

// dispatchProtocolURL forwards one native protocol event or retains it until the app installs its handler.
func dispatchProtocolURL(rawURL string) {
	if rawURL == "" {
		return
	}

	protocolURLDispatcher.Lock()
	handler := protocolURLDispatcher.handler
	if handler == nil {
		protocolURLDispatcher.pending = append(protocolURLDispatcher.pending, rawURL)
	}
	protocolURLDispatcher.Unlock()

	if handler != nil {
		handler(rawURL)
	}
}
