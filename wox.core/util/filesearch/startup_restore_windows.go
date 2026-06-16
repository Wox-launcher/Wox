//go:build windows

package filesearch

import "time"

func usnRootNeedsStartupReconcile(root RootRecord, now time.Time) bool {
	journal, ok := resolveWindowsUSNJournal(root.Path)
	if !ok {
		return true
	}

	prepared := prepareUSNVolumeRefresh([]RootRecord{root}, journal, now, defaultFeedCursorSafeWindow)
	return len(prepared.signals) > 0
}
