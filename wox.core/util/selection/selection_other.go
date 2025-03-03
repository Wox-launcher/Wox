//go:build !darwin

package selection

import "context"

// GetSelected is the implementation for non-macOS platforms
// It directly uses the clipboard method
func GetSelected(ctx context.Context) (Selection, error) {
	// Non-macOS platforms directly use clipboard method
	return getSelectedByClipboard(ctx)
}
