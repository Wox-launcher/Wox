package overlay

func showWindow(opts WindowOptions) {
	// Stub implementation for Linux. Register the close callback so the
	// API contract matches other platforms even though overlay is not
	// natively supported here yet.
	_ = opts
}

func Close(id string) {
	// Stub implementation for Linux
	clickCallbacksMu.Lock()
	delete(clickCallbacks, id)
	clickCallbacksMu.Unlock()
	closeCallbacksMu.Lock()
	delete(closeCallbacks, id)
	closeCallbacksMu.Unlock()
	ReleaseNativeAttachment(id)
}
