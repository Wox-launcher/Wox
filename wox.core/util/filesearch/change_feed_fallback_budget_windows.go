//go:build windows

package filesearch

// fallbackWatchBudget bounds ReadDirectoryChangesW handles on volumes without a
// USN journal. Each watch pins a directory handle plus a locked notification
// buffer, so deeper directories rely on the periodic relist instead.
func fallbackWatchBudget() int {
	return 512
}
