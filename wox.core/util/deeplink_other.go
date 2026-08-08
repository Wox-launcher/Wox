//go:build !linux && !windows

package util

import "context"

// EnsureDeepLinkProtocolHandler is handled by the macOS application bundle metadata.
func EnsureDeepLinkProtocolHandler(ctx context.Context) bool {
	return false
}
