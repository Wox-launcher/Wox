package filesearch

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"wox/util"
)

const lastFullIndexElapsedMsKey = "last_full_index_elapsed_ms"

// GetIndexStats returns persisted index volume, on-disk size, and last-run
// duration without sampling FTS vocab or per-root estimates.
func (e *Engine) GetIndexStats(ctx context.Context) (IndexStatsSnapshot, error) {
	if e == nil {
		return IndexStatsSnapshot{}, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if serviceStats, serviceRunning, err := getFileIndexServiceIndexStats(ctx); serviceRunning {
		return serviceStats, err
	}

	status, err := e.GetStatus(ctx)
	if err != nil {
		return IndexStatsSnapshot{}, err
	}

	e.mu.RLock()
	if e.closed || e.db == nil {
		e.mu.RUnlock()
		return IndexStatsSnapshot{}, fmt.Errorf("filesearch engine closed")
	}
	db := e.db
	e.mu.RUnlock()

	// Bug fix: ActiveRunElapsedMs only lives on the transient run snapshot. The
	// scanner clears that state as soon as a full index finishes, so Settings
	// would always show "-" unless the last completed duration is persisted.
	persistedElapsedMs, err := db.LastFullIndexElapsedMs(ctx)
	if err != nil {
		util.GetLogger().Warn(ctx, "filesearch failed to load last full index duration: "+err.Error())
	}

	stats := IndexStatsSnapshot{
		DiskBytes:     db.IndexDiskBytes(),
		RootCount:     status.RootCount,
		LastElapsedMs: resolveLastIndexElapsedMs(status.ActiveRunElapsedMs, persistedElapsedMs),
		IsIndexing:    status.IsIndexing,
	}
	fileCount, entryCount, err := db.SearchIndexCounts(ctx)
	if err != nil {
		stats.Error = err.Error()
		return stats, nil
	}
	stats.FileCount = fileCount
	stats.EntryCount = entryCount
	return stats, nil
}

func resolveLastIndexElapsedMs(activeElapsedMs, persistedElapsedMs int64) int64 {
	if activeElapsedMs > 0 {
		return activeElapsedMs
	}
	if persistedElapsedMs > 0 {
		return persistedElapsedMs
	}
	return 0
}

// SetLastFullIndexElapsedMs stores the completed full-index duration for Settings.
func (d *FileSearchDB) SetLastFullIndexElapsedMs(ctx context.Context, elapsedMs int64) error {
	if d == nil || d.db == nil || elapsedMs <= 0 {
		return nil
	}
	_, err := d.db.ExecContext(ctx, `
		INSERT INTO meta(key, value)
		VALUES(?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, lastFullIndexElapsedMsKey, strconv.FormatInt(elapsedMs, 10))
	if err != nil {
		return fmt.Errorf("set last full index duration: %w", err)
	}
	return nil
}

// LastFullIndexElapsedMs returns the persisted last full-index duration.
func (d *FileSearchDB) LastFullIndexElapsedMs(ctx context.Context) (int64, error) {
	if d == nil || d.db == nil {
		return 0, nil
	}
	var value string
	err := d.db.QueryRowContext(ctx, `SELECT value FROM meta WHERE key = ?`, lastFullIndexElapsedMsKey).Scan(&value)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("load last full index duration: %w", err)
	}
	elapsedMs, parseErr := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if parseErr != nil || elapsedMs < 0 {
		return 0, nil
	}
	return elapsedMs, nil
}
