//go:build linux

package filesearch

import (
	"os"
	"strconv"
	"strings"
)

const (
	// inotify watches are cheap (about 1 KB of kernel memory each) but the
	// per-user limit is shared with editors, IDEs and sync clients. The shared fallback pool
	// uses at most half; other processes can still exhaust the remaining quota.
	fallbackWatchBudgetShare   = 2
	fallbackWatchBudgetDefault = 8192
	// Bound Wox bookkeeping even on hosts with very large kernel limits.
	fallbackWatchBudgetMax = 32768
)

// fallbackWatchBudget derives the inotify watch budget from the kernel limit.
func fallbackWatchBudget() int {
	data, err := os.ReadFile("/proc/sys/fs/inotify/max_user_watches")
	if err != nil {
		return fallbackWatchBudgetDefault / fallbackWatchBudgetShare
	}
	limit, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || limit <= 0 {
		return fallbackWatchBudgetDefault / fallbackWatchBudgetShare
	}
	return min(limit/fallbackWatchBudgetShare, fallbackWatchBudgetMax)
}
