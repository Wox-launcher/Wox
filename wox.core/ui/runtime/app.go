package woxui

import (
	window "wox/ui/runtime/internal/window"
)

var ErrPlatformUnsupported = window.ErrPlatformUnsupported

// Run initializes the platform, calls start on the UI thread, and owns that thread's event loop.
func Run(start func() error) error { return window.Run(start) }

// Call executes fn synchronously on the native UI thread owned by Run.
func Call(fn func()) error { return window.Call(fn) }

// Post queues fn on the native UI thread, even when called from that thread.
// Native callbacks can use it to defer teardown until their current dispatch returns.
func Post(fn func()) error { return window.Post(fn) }

// SetProtocolURLHandler installs the application-level handler and drains URLs received before startup finished.
func SetProtocolURLHandler(handler func(string)) { window.SetProtocolURLHandler(handler) }
