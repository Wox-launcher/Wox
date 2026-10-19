//go:build !windows && !darwin

package filesearch

// newConsumerChangeFeed keeps small consumers from reserving scanner-sized queues.
func newConsumerChangeFeed() ChangeFeed { return newFallbackChangeFeed(256) }
