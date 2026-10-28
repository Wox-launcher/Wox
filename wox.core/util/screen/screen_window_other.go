//go:build !windows

package screen

// DisplayIDForWindow is unnecessary on platforms with a unified logical desktop.
func DisplayIDForWindow(windowID string) string { return "" }
