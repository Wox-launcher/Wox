package mainthread

// Call executes f on the main thread and blocks until f returns.
// Do not use it as a fire-and-forget dispatch helper.
func Call(f func()) {
	if f == nil {
		return
	}
	if callDispatcher(f) {
		return
	}
	f()
}
