//go:build !windows

package filesearch

import "context"

func tryServiceNameSearch(context.Context, SearchQuery, int) ([]SearchResult, bool, error) {
	return nil, false, nil
}

func pauseFileIndexService(context.Context) (bool, error) { return false, nil }

func resumeFileIndexService(context.Context) error { return nil }

func getFileIndexServiceStatus(context.Context) (StatusSnapshot, bool, error) {
	return StatusSnapshot{}, false, nil
}

func getFileIndexServiceIndexStats(context.Context) (IndexStatsSnapshot, bool, error) {
	return IndexStatsSnapshot{}, false, nil
}
