package filesearch

import (
	"context"
	"errors"
	"os"
	"strings"
	"wox/util/filesearch/column"
)

type columnIndexSnapshot struct {
	RootCount                int
	EntryCount, FileCount    int64
	DiskBytes, MetadataBytes int64
	TopRoots                 []columnRootSnapshot
}

type columnRootSnapshot struct {
	RootID, Path    string
	Docs, DiskBytes int64
}

func (d *FileSearchDB) ensureBaseTables(ctx context.Context) error {
	createSQL := `
	CREATE TABLE IF NOT EXISTS meta (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS roots (
		id TEXT PRIMARY KEY,
		path TEXT NOT NULL UNIQUE,
		kind TEXT NOT NULL,
		status TEXT NOT NULL,
		feed_type TEXT NOT NULL DEFAULT '',
		feed_cursor TEXT NOT NULL DEFAULT '',
		feed_state TEXT NOT NULL DEFAULT '',
		last_reconcile_at INTEGER NOT NULL DEFAULT 0,
		last_full_scan_at INTEGER NOT NULL DEFAULT 0,
		progress_current INTEGER NOT NULL DEFAULT 0,
		progress_total INTEGER NOT NULL DEFAULT 0,
		last_error TEXT,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL
	);
	`

	if _, err := d.db.ExecContext(ctx, createSQL); err != nil {
		return err
	}

	return nil
}

// SearchIndexCounts reuses per-generation counters for repeated UI polling.
func (d *FileSearchDB) SearchIndexCounts(ctx context.Context) (int64, int64, error) {
	d.directory.mu.RLock()
	defer d.directory.mu.RUnlock()
	var files, entries int64
	for _, shard := range d.directory.shards {
		f, e, err := shard.store.Counts(ctx)
		if err != nil && !errors.Is(err, column.ErrNotReady) {
			return 0, 0, err
		}
		files += f
		entries += e
	}
	return files, entries, nil
}

func (d *FileSearchDB) IndexDiskBytes() int64 {
	return d.columnDiskBytes()
}

func (d *FileSearchDB) SearchIndexSnapshot(ctx context.Context) (columnIndexSnapshot, error) {
	return d.columnSnapshot(ctx)
}

func fileSizeOrZero(path string) int64 {
	if strings.TrimSpace(path) == "" {
		return 0
	}
	// Bug fix: storage diagnostics should report the indexed symlink entry size
	// instead of following a symlink target. This keeps snapshot sampling aligned
	// with full-scan and direct-delta metadata.
	info, err := os.Lstat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

func (d *FileSearchDB) BeginBulkSync() {
	d.bulkSyncMu.Lock()
	d.bulkSyncDepth++
	d.bulkSyncMu.Unlock()
}

func (d *FileSearchDB) EndBulkSync(ctx context.Context) error {
	d.bulkSyncMu.Lock()
	if d.bulkSyncDepth > 0 {
		d.bulkSyncDepth--
	}
	done := d.bulkSyncDepth == 0
	d.bulkSyncMu.Unlock()
	if done {
		d.directory.abortBuilds()
	}
	return nil
}

func (d *FileSearchDB) prepareBulkSyncFullRunRoot(ctx context.Context, rootID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return d.directory.beginBuild(rootID)
}

func escapeLikePattern(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return replacer.Replace(value)
}

func (d *FileSearchDB) persistFinalizedRoot(ctx context.Context, root RootRecord) error {
	// Finalize is the only place allowed to advance the persisted feed cursor.
	// Applying job rows before this point is safe because a crash can replay the
	// same writes, but advancing the cursor early would acknowledge unseen
	// change-feed signals and permanently skip them on recovery.
	// Bug fix: finalize must not run FTS optimize. That command is global table
	// maintenance, and CPU profiles showed doing it here made every small dirty
	// flush scan all four FTS tables. A scanner timer now runs that compaction
	// every 12 hours instead.
	_, err := d.db.ExecContext(ctx, `
		UPDATE roots
		SET status = ?, feed_type = ?, feed_cursor = ?, feed_state = ?, last_reconcile_at = ?, last_full_scan_at = ?,
		    progress_current = ?, progress_total = ?, last_error = ?, updated_at = ?
		WHERE id = ?
	`,
		string(root.Status),
		string(root.FeedType),
		root.FeedCursor,
		string(root.FeedState),
		root.LastReconcileAt,
		root.LastFullScanAt,
		root.ProgressCurrent,
		root.ProgressTotal,
		root.LastError,
		root.UpdatedAt,
		root.ID,
	)
	return err
}
