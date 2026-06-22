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

// GetOpenSaveDialogRectByPid is unsupported on Linux until the Explorer monitor exposes file dialog geometry there.
func GetOpenSaveDialogRectByPid(pid int) (int, int, int, int, bool) {
	return 0, 0, 0, 0, false
}

// GetOpenSaveDialogWindowIdByPid is unsupported on Linux until file dialog HWND-equivalent tracking exists.
func GetOpenSaveDialogWindowIdByPid(pid int) string {
	return ""
}

func AddExplorerRawKeyListener(listener ExplorerRawKeyListener) (ExplorerRawKeySubscription, error) {
	// Stub implementation for Linux
	return nil, nil
}

func AddExplorerOpenSaveRawKeyListener(listener ExplorerRawKeyListener) (ExplorerRawKeySubscription, error) {
	// Stub implementation for Linux
	return nil, nil
}
