//go:build !darwin && !windows

package filesearch

func newPlatformChangeFeed() ChangeFeed {
	return NewFallbackChangeFeed()
}
