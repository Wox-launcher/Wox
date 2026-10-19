package filesearch

func newConsumerChangeFeed() ChangeFeed { return newFSEventsChangeFeed(256) }
