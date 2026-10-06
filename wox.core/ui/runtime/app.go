package woxui

import "errors"

// ErrPlatformUnsupported reports that the current OS has no native backend yet.
var ErrPlatformUnsupported = errors.New("woxui: platform backend is not implemented")

// Run initializes the platform, calls start on the UI thread, and owns that thread's event loop.
func Run(start func() error) error {
	if start == nil {
		return errors.New("start callback is required")
	}
	return platformRun(start)
}

// Call executes fn synchronously on the native UI thread owned by Run.
func Call(fn func()) error {
	if fn == nil {
		return errors.New("UI callback is required")
	}
	return platformCall(fn)
}

// Post queues fn on the native UI thread, even when called from that thread.
// Native callbacks can use it to defer teardown until their current dispatch returns.
func Post(fn func()) error {
	if fn == nil {
		return errors.New("UI callback is required")
	}
	return platformPost(fn)
}
