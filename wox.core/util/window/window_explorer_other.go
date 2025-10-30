//go:build !windows && !darwin

package window

// GetActiveFileExplorerPath returns empty string on platforms other than Windows and macOS.
func GetActiveFileExplorerPath() string {
	return ""
}
