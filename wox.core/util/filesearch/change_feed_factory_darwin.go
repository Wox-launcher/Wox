//go:build darwin

package filesearch

func newPlatformChangeFeed() ChangeFeed {
	return NewFSEventsChangeFeed()
}
