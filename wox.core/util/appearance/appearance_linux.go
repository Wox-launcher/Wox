package appearance

func isDark() bool {
	// TODO: implement Linux dark mode detection
	// This would typically involve checking D-Bus for GNOME/KDE theme settings
	return false
}

func watchSystemAppearance(callback func(isDark bool)) {
	// TODO: implement Linux appearance watching
	// This would typically involve D-Bus signals for theme changes
}

func stopWatching() {
	// TODO: implement Linux stop watching
}
