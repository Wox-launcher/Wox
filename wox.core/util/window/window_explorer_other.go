//go:build !windows && !darwin

package window

// GetActiveFileExplorerPath returns empty string on platforms other than Windows and macOS.
func GetActiveFileExplorerPath() string {
	return ""
}

// IsFileExplorer returns false on platforms other than Windows and macOS.
func IsFileExplorer(pid int) (bool, error) {
	return false, nil
}

// NavigateActiveFileExplorer is not supported on this platform.
func NavigateActiveFileExplorer(targetPath string) bool {
	return false
}
