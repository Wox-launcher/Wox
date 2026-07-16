package mainthread

import "sync"

var dispatcherState struct {
	sync.RWMutex
	call func(func())
}

// SetDispatcher routes main-thread calls through an externally owned UI event loop.
func SetDispatcher(call func(func())) {
	dispatcherState.Lock()
	dispatcherState.call = call
	dispatcherState.Unlock()
}

func callDispatcher(fn func()) bool {
	dispatcherState.RLock()
	call := dispatcherState.call
	dispatcherState.RUnlock()
	if call == nil {
		return false
	}
	call(fn)
	return true
}
