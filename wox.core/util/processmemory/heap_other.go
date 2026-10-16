//go:build !windows

package processmemory

// ReleaseIdleNativeHeap is a no-op where no allocator trim is implemented.
func ReleaseIdleNativeHeap() error { return nil }
