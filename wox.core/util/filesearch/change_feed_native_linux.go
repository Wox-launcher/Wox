package filesearch

var sharedFallbackWatches = newFallbackWatchPool(fallbackWatchBudget())

func newFallbackNativeWatcher() (fallbackNativeWatcher, error) {
	return sharedFallbackWatches.subscribe()
}
