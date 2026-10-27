//go:build !linux

package util

// IsOmarchy is always false outside Linux.
func IsOmarchy() bool { return false }
