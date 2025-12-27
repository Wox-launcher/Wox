package appearance

// IsDark returns whether the system is currently in dark mode
func IsDark() bool {
	return isDark()
}

// WatchSystemAppearance starts watching for system appearance changes
// and calls the callback when the appearance changes
// The callback receives true if dark mode, false if light mode
func WatchSystemAppearance(callback func(isDark bool)) {
	watchSystemAppearance(callback)
}

// StopWatching stops watching for system appearance changes
func StopWatching() {
	stopWatching()
}
