//go:build windows

package filesearch

import (
	"context"
	"path/filepath"
	"strings"

	"wox/util"
	"wox/util/filesearchservice"
)

// tryServiceNameSearch uses the service for ordinary filename queries while
// preserving SQLite's richer path, quoted-phrase, and wildcard grammar.
func tryServiceNameSearch(ctx context.Context, query SearchQuery, limit int) ([]SearchResult, bool, error) {
	if !filesearchservice.IsRunning() || strings.ContainsAny(query.Raw, `*/\"`) {
		return nil, false, nil
	}
	entries, err := filesearchservice.Search(ctx, query.Raw, limit, !query.DisablePinyin)
	if err != nil {
		return nil, true, err
	}
	results := make([]SearchResult, 0, len(entries))
	for _, entry := range entries {
		results = append(results, SearchResult{
			Path:       entry.Path,
			Name:       filepath.Base(entry.Path),
			ParentPath: filepath.Dir(entry.Path),
			IsDir:      entry.IsDir,
			Score:      entry.Score,
		})
	}
	return results, true, nil
}

func pauseFileIndexService(ctx context.Context) (bool, error) {
	if !filesearchservice.IsRunning() {
		return false, nil
	}
	if err := filesearchservice.Pause(ctx); err != nil {
		return false, err
	}
	return true, nil
}

func resumeFileIndexService(ctx context.Context) error {
	if !filesearchservice.IsRunning() {
		return nil
	}
	return filesearchservice.Resume(ctx, filepath.Join(util.GetLocation().GetFileSearchDirectory(), filesearchservice.IndexDirectory))
}

func getFileIndexServiceStatus(ctx context.Context) (StatusSnapshot, bool, error) {
	if !filesearchservice.IsRunning() {
		return StatusSnapshot{}, false, nil
	}
	stats, err := filesearchservice.GetIndexStats(ctx)
	if err != nil {
		return StatusSnapshot{}, true, err
	}
	return statusSnapshotFromServiceIndexStats(stats), true, nil
}

func getFileIndexServiceIndexStats(ctx context.Context) (IndexStatsSnapshot, bool, error) {
	if !filesearchservice.IsRunning() {
		return IndexStatsSnapshot{}, false, nil
	}
	stats, err := filesearchservice.GetIndexStats(ctx)
	if err != nil {
		return IndexStatsSnapshot{}, true, err
	}
	elapsedMs := stats.LastElapsedMs
	if stats.IsIndexing {
		elapsedMs = stats.ElapsedMs
	}
	return IndexStatsSnapshot{
		FileCount:     stats.FileCount,
		EntryCount:    stats.EntryCount,
		DiskBytes:     stats.IndexBytes,
		RootCount:     serviceVolumeCount(stats),
		LastElapsedMs: elapsedMs,
		IsIndexing:    stats.IsIndexing,
		Error:         stats.Error,
	}, true, nil
}

func statusSnapshotFromServiceIndexStats(stats filesearchservice.IndexStats) StatusSnapshot {
	status := StatusSnapshot{
		RootCount:             serviceVolumeCount(stats),
		ActiveRootIndex:       stats.VolumeIndex,
		ActiveRootTotal:       stats.VolumeTotal,
		ActiveRootPath:        stats.CurrentVolume,
		ActiveDiscoveredCount: stats.EntryCount,
		ActiveRunFileCount:    stats.FileCount,
		ActiveRunEntryCount:   stats.EntryCount,
		ActiveRunElapsedMs:    stats.ElapsedMs,
		LastError:             stats.Error,
		IsIndexing:            stats.IsIndexing,
		IsInitialIndexing:     stats.IsIndexing && stats.EntryCount == 0,
	}
	if stats.IsIndexing {
		status.ActiveRootStatus = RootStatusScanning
		status.ActiveRunStatus = RunStatusExecuting
		status.ActiveRunKind = RunKindFull
		status.ActiveStage = RunStageExecuting
	}
	if stats.Error != "" {
		status.ErrorRootCount = 1
		status.ErrorRootPath = stats.CurrentVolume
		if !stats.IsIndexing {
			status.ActiveRootStatus = RootStatusError
		}
	}
	return status
}

func serviceVolumeCount(stats filesearchservice.IndexStats) int {
	if stats.VolumeTotal > 0 {
		return stats.VolumeTotal
	}
	return stats.VolumeCount
}
