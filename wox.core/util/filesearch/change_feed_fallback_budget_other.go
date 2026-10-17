//go:build !linux && !windows

package filesearch

// fallbackWatchBudget is conservative for kqueue backends, where every watched
// directory and each of its files consumes an open file descriptor.
func fallbackWatchBudget() int {
	return 128
}
