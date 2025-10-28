//go:build !windows

package window

// GetActiveFileExplorerPath returns empty string on non-Windows platforms.
func GetActiveFileExplorerPath() string {
	return ""
}

