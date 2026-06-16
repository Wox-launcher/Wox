//go:build windows

package filesearch

func newPlatformChangeFeed() ChangeFeed {
	return NewWindowsChangeFeed()
}
