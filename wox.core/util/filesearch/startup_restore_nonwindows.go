//go:build !windows

package filesearch

import "time"

func usnRootNeedsStartupReconcile(root RootRecord, now time.Time) bool {
	_ = root
	_ = now
	return true
}
