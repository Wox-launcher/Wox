package explorer

func StartExplorerMonitor(activated func(pid int), deactivated func(), _ func(string)) {
	// Stub implementation for Linux
}

func StopExplorerMonitor() {
	// Stub implementation for Linux
}

func StartExplorerOpenSaveMonitor(activated func(pid int), deactivated func(), _ func(string)) {
	// Stub implementation for Linux
}

func StopExplorerOpenSaveMonitor() {
	// Stub implementation for Linux
}

func GetActiveExplorerRect() (int, int, int, int, bool) {
	// Stub implementation for Linux
	return 0, 0, 0, 0, false
}

func GetActiveDialogRect() (int, int, int, int, bool) {
	// Stub implementation for Linux
	return 0, 0, 0, 0, false
}
