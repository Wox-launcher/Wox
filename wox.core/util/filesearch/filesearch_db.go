package filesearch

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"time"
	"wox/util"
	"wox/util/sqlitememory"

	_ "github.com/mattn/go-sqlite3"
)

type FileSearchDB struct {
	db        *sql.DB
	dbPath    string
	directory *directoryIndex
	// Bulk depth owns unpublished full-run builders until their finalize jobs.
	bulkSyncMu    sync.Mutex
	bulkSyncDepth int
}

const rootRecordSelectColumns = `
	id, path, kind, status, feed_type, feed_cursor, feed_state, last_reconcile_at, last_full_scan_at,
	progress_current, progress_total, last_error, created_at, updated_at
`

func NewFileSearchDB(ctx context.Context) (*FileSearchDB, error) {
	fileSearchDir := util.GetLocation().GetFileSearchDirectory()
	// Bug fix: manual full rebuilds remove the whole filesearch directory after
	// closing SQLite. Recreate the directory at DB-open time so both startup and
	// reset paths have one durable owner for the storage location.
	if err := util.GetLocation().EnsureDirectoryExist(fileSearchDir); err != nil {
		return nil, err
	}

	dbPath := filepath.Join(fileSearchDir, "metadata.db")
	dsn := dbPath + "?" +
		"_journal_mode=WAL&" +
		"_synchronous=NORMAL&" +
		"_cache_size=-4000&" +
		"_foreign_keys=true&" +
		"_busy_timeout=5000"

	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open filesearch database: %w", err)
	}

	// File indexing uses long write transactions. Allow a few extra read
	// connections so queries and status polling can keep using the last
	// committed snapshot instead of blocking behind the writer.
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(time.Hour)

	fileSearchDB := &FileSearchDB{db: db, dbPath: dbPath}
	if err := fileSearchDB.initTables(ctx); err != nil {
		db.Close()
		return nil, err
	}
	if err := fileSearchDB.restoreDirectory(ctx); err != nil {
		if fileSearchDB.directory != nil {
			fileSearchDB.directory.close()
		}
		db.Close()
		return nil, err
	}
	sqlitememory.Register(db, "filesearch")

	return fileSearchDB, nil
}

func (d *FileSearchDB) Close() error {
	if d == nil || d.db == nil {
		return nil
	}
	sqlitememory.Unregister(d.db)
	return errors.Join(d.directory.close(), d.db.Close())
}

func (d *FileSearchDB) initTables(ctx context.Context) error {
	return d.ensureBaseTables(ctx)
}

func (d *FileSearchDB) UpsertRoot(ctx context.Context, root RootRecord) error {
	if err := execRootUpsert(ctx, d.db, root); err != nil {
		return err
	}
	persisted, err := d.FindRootByPath(ctx, root.Path)
	if err != nil {
		return err
	}
	return d.directory.attach(*persisted)
}

type rootExecContext interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func execRootUpsert(ctx context.Context, exec rootExecContext, root RootRecord) error {
	query := `
	INSERT INTO roots (
		id, path, kind, status, feed_type, feed_cursor, feed_state, last_reconcile_at, last_full_scan_at,
		progress_current, progress_total, last_error, created_at, updated_at
	)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(path) DO UPDATE SET
		kind = excluded.kind,
		status = excluded.status,
		feed_type = excluded.feed_type,
		feed_cursor = excluded.feed_cursor,
		feed_state = excluded.feed_state,
		last_reconcile_at = excluded.last_reconcile_at,
		last_full_scan_at = excluded.last_full_scan_at,
		progress_current = excluded.progress_current,
		progress_total = excluded.progress_total,
		last_error = excluded.last_error,
		updated_at = excluded.updated_at
	`

	_, err := exec.ExecContext(
		ctx,
		query,
		root.ID,
		root.Path,
		string(root.Kind),
		string(root.Status),
		string(root.FeedType),
		root.FeedCursor,
		string(root.FeedState),
		root.LastReconcileAt,
		root.LastFullScanAt,
		root.ProgressCurrent,
		root.ProgressTotal,
		root.LastError,
		root.CreatedAt,
		root.UpdatedAt,
	)
	return err
}

func (d *FileSearchDB) UpdateRootState(ctx context.Context, root RootRecord) error {
	query := `
	UPDATE roots
	SET status = ?, feed_type = ?, feed_cursor = ?, feed_state = ?, last_reconcile_at = ?, last_full_scan_at = ?,
	    progress_current = ?, progress_total = ?, last_error = ?, updated_at = ?
	WHERE id = ?
	`

	_, err := d.db.ExecContext(
		ctx,
		query,
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

func (d *FileSearchDB) ListRoots(ctx context.Context) ([]RootRecord, error) {
	rows, err := d.db.QueryContext(ctx, `
		SELECT `+rootRecordSelectColumns+`
		FROM roots
		ORDER BY path ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roots []RootRecord
	for rows.Next() {
		root, err := scanRootRecord(rows)
		if err != nil {
			return nil, err
		}
		roots = append(roots, root)
	}

	return roots, rows.Err()
}

func (d *FileSearchDB) DeleteRoot(ctx context.Context, rootID string) error {
	if _, err := d.db.ExecContext(ctx, `DELETE FROM roots WHERE id = ?`, rootID); err != nil {
		return err
	}
	return d.directory.detach(rootID)
}

func (d *FileSearchDB) ResetIndex(ctx context.Context) error {
	if d == nil || d.db == nil {
		return nil
	}

	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Feature addition: manual "Index Files" must start from a clean search
	// index, not just enqueue another full scan. Keep roots because they are
	// configuration, but drop every column shard so the next preparation run
	// rebuilds search artifacts from scratch.
	roots, err := d.ListRoots(ctx)
	if err != nil {
		return err
	}
	for _, root := range roots {
		if err := d.directory.detach(root.ID); err != nil {
			return err
		}
		if err := d.directory.attach(root); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM meta WHERE key = ? OR key GLOB ?`, lastFullIndexElapsedMsKey, nativeFullIndexElapsedMsPrefix+"*"); err != nil {
		return fmt.Errorf("reset last full index duration: %w", err)
	}
	now := util.GetSystemTimestamp()
	if _, err := tx.ExecContext(ctx, `
		UPDATE roots
		SET status = ?,
		    feed_cursor = '',
		    feed_state = '',
		    last_reconcile_at = 0,
		    last_full_scan_at = 0,
		    progress_current = 0,
		    progress_total = 0,
		    last_error = NULL,
		    updated_at = ?
	`, RootStatusPreparing, now); err != nil {
		return fmt.Errorf("reset roots: %w", err)
	}

	return tx.Commit()
}

func (d *FileSearchDB) FindRootByPath(ctx context.Context, rootPath string) (*RootRecord, error) {
	row := d.db.QueryRowContext(ctx, `
		SELECT `+rootRecordSelectColumns+`
		FROM roots
		WHERE path = ?
	`, rootPath)

	root, err := scanRootRecord(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return &root, nil
}

func (d *FileSearchDB) ReplaceRootSnapshot(
	ctx context.Context,
	root RootRecord,
	directories []DirectoryRecord,
	entries []EntryRecord,
	onProgress func(ReplaceEntriesProgress),
) error {
	batch := SubtreeSnapshotBatch{RootID: root.ID, ScopePath: root.Path, Directories: directories, Entries: entries}
	if err := validateSubtreeSnapshotBatch(batch); err != nil {
		return err
	}
	if onProgress != nil {
		onProgress(ReplaceEntriesProgress{Stage: ReplaceEntriesStagePreparing})
	}
	err := d.writeColumnScope(ctx, root.ID, root.Path, false, func(yield func(SubtreeSnapshotBatch) error) error { return yield(batch) })
	if err == nil && onProgress != nil {
		onProgress(ReplaceEntriesProgress{Stage: ReplaceEntriesStageFinalizing, Current: int64(len(entries)), Total: int64(len(entries))})
	}
	return err
}

func (d *FileSearchDB) ReplaceRootEntries(ctx context.Context, root RootRecord, entries []EntryRecord, onProgress func(ReplaceEntriesProgress)) error {
	return d.ReplaceRootSnapshot(ctx, root, nil, entries, onProgress)
}

func (d *FileSearchDB) ApplyDirectFilesJob(ctx context.Context, job Job, batch SubtreeSnapshotBatch) error {
	if job.Kind != JobKindDirectFiles {
		return fmt.Errorf("invalid direct files job")
	}
	if err := validateJobSnapshotBatch(job, batch); err != nil {
		return err
	}
	return d.writeColumnScope(ctx, batch.RootID, batch.ScopePath, true, func(yield func(SubtreeSnapshotBatch) error) error { return yield(batch) })
}

// ApplyJobStream streams one direct-files, subtree, or directory job into
// the column store.
func (d *FileSearchDB) ApplyJobStream(ctx context.Context, root RootRecord, job Job, snapshot *SnapshotBuilder, onProgress func(JobApplyStats)) (JobApplyStats, error) {
	return d.applyColumnJob(ctx, root, job, snapshot, onProgress)
}

func (d *FileSearchDB) ApplySubtreeJob(ctx context.Context, job Job, batch SubtreeSnapshotBatch) error {
	if job.Kind != JobKindSubtree {
		return fmt.Errorf("apply subtree job requires kind %q, got %q", JobKindSubtree, job.Kind)
	}
	if err := validateJobSnapshotBatch(job, batch); err != nil {
		return err
	}

	// Subtree jobs still own a complete recursive scope, so the existing scoped
	// replace helper remains correct and keeps this job-oriented wrapper small.
	return d.ReplaceSubtreeSnapshot(ctx, batch)
}

func (d *FileSearchDB) ApplyDirectDeltaJob(ctx context.Context, root RootRecord, job Job, policy *policyState) error {
	return d.applyColumnDelta(ctx, root, job, policy)
}

func isDeleteOnlyDelta(kind ChangeSemanticKind) bool {
	// Only explicit Remove is handled as delete-without-stat. Rename is excluded
	// because FSEvents sends the renamed flag on both the old path (gone) and the
	// new path (now exists). Treating Rename as delete-only would remove the new
	// path from the index every time a file is renamed. The stat path below
	// handles both cases correctly: old path → IsNotExist → delete, new path →
	// exists → upsert.
	return kind == ChangeSemanticKindRemove
}

func (d *FileSearchDB) FinalizeRootRun(ctx context.Context, root RootRecord) error {
	if err := d.directory.finishBuild(ctx, root); err != nil {
		return err
	}
	if err := d.persistFinalizedRoot(ctx, root); err != nil {
		return err
	}
	if err := d.compactColumns(ctx); err != nil {
		util.GetLogger().Warn(ctx, "filesearch deferred compaction: "+err.Error())
	}
	return nil
}

func (d *FileSearchDB) ReplaceSubtreeSnapshot(ctx context.Context, batch SubtreeSnapshotBatch) error {
	return d.ReplaceSubtreeSnapshots(ctx, []SubtreeSnapshotBatch{batch})
}

func (d *FileSearchDB) ReplaceSubtreeSnapshots(ctx context.Context, batches []SubtreeSnapshotBatch) error {
	if len(batches) == 0 {
		return nil
	}
	rootID := batches[0].RootID
	scopes := make([]string, 0, len(batches))
	for _, batch := range batches {
		if batch.RootID != rootID {
			return fmt.Errorf("subtree batch spans multiple root generations")
		}
		if err := validateSubtreeSnapshotBatch(batch); err != nil {
			return err
		}
		scopes = append(scopes, batch.ScopePath)
	}
	return d.writeColumnScopes(ctx, rootID, scopes, false, func(yield func(SubtreeSnapshotBatch) error) error {
		for _, batch := range batches {
			if err := yield(batch); err != nil {
				return err
			}
		}
		return nil
	})
}

func validateJobSnapshotBatch(job Job, batch SubtreeSnapshotBatch) error {
	if err := validateSubtreeSnapshotBatch(batch); err != nil {
		return err
	}
	if job.RootID == "" {
		return fmt.Errorf("job root id is required")
	}
	if batch.RootID != job.RootID {
		return fmt.Errorf("job root id %q does not match batch root id %q", job.RootID, batch.RootID)
	}
	if filepath.Clean(batch.ScopePath) != filepath.Clean(job.ScopePath) {
		return fmt.Errorf("job scope path %q does not match batch scope path %q", job.ScopePath, batch.ScopePath)
	}
	return nil
}

func validateSubtreeSnapshotBatch(batch SubtreeSnapshotBatch) error {
	if batch.RootID == "" {
		return fmt.Errorf("subtree snapshot root id is required")
	}

	cleanScope := filepath.Clean(batch.ScopePath)
	if batch.ScopePath == "" || cleanScope == "." || !filepath.IsAbs(cleanScope) {
		return fmt.Errorf("subtree snapshot scope path %q is invalid", batch.ScopePath)
	}

	for _, directory := range batch.Directories {
		if directory.RootID != batch.RootID {
			return fmt.Errorf("directory %q belongs to root %q, want %q", directory.Path, directory.RootID, batch.RootID)
		}
		if filepath.Clean(directory.ParentPath) != filepath.Dir(filepath.Clean(directory.Path)) {
			return fmt.Errorf("directory %q has parent %q, want %q", directory.Path, directory.ParentPath, filepath.Dir(filepath.Clean(directory.Path)))
		}
		if !pathWithinScope(batch.ScopePath, directory.Path) {
			return fmt.Errorf("directory %q is outside subtree scope %q", directory.Path, batch.ScopePath)
		}
	}

	for _, entry := range batch.Entries {
		if entry.RootID != batch.RootID {
			return fmt.Errorf("entry %q belongs to root %q, want %q", entry.Path, entry.RootID, batch.RootID)
		}
		if filepath.Clean(entry.ParentPath) != filepath.Dir(filepath.Clean(entry.Path)) {
			return fmt.Errorf("entry %q has parent %q, want %q", entry.Path, entry.ParentPath, filepath.Dir(filepath.Clean(entry.Path)))
		}
		if !pathWithinScope(batch.ScopePath, entry.Path) {
			return fmt.Errorf("entry %q is outside subtree scope %q", entry.Path, batch.ScopePath)
		}
	}

	return nil
}

func (d *FileSearchDB) FindRootByID(ctx context.Context, rootID string) (*RootRecord, error) {
	row := d.db.QueryRowContext(ctx, `
		SELECT `+rootRecordSelectColumns+`
		FROM roots
		WHERE id = ?
	`, rootID)

	root, err := scanRootRecord(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return &root, nil
}

func pathWithinScope(scopePath, candidatePath string) bool {
	cleanScope := filepath.Clean(scopePath)
	cleanCandidate := filepath.Clean(candidatePath)

	rel, err := filepath.Rel(cleanScope, cleanCandidate)
	if err != nil {
		return false
	}

	if rel == "." {
		return true
	}

	parentPrefix := ".." + string(filepath.Separator)
	if rel == ".." || len(rel) >= len(parentPrefix) && rel[:len(parentPrefix)] == parentPrefix {
		return false
	}

	return true
}

func (d *FileSearchDB) ListEntries(ctx context.Context) ([]EntryRecord, error) {
	return d.columnEntries(ctx, "")
}

func (d *FileSearchDB) ListEntriesByRoot(ctx context.Context, rootID string) ([]EntryRecord, error) {
	return d.columnEntries(ctx, rootID)
}

type rootScanner interface {
	Scan(dest ...any) error
}

func scanRootRecord(scanner rootScanner) (RootRecord, error) {
	var root RootRecord
	var kind string
	var status string
	var feedType string
	var feedState string
	if err := scanner.Scan(
		&root.ID,
		&root.Path,
		&kind,
		&status,
		&feedType,
		&root.FeedCursor,
		&feedState,
		&root.LastReconcileAt,
		&root.LastFullScanAt,
		&root.ProgressCurrent,
		&root.ProgressTotal,
		&root.LastError,
		&root.CreatedAt,
		&root.UpdatedAt,
	); err != nil {
		return RootRecord{}, err
	}

	root.Kind = RootKind(kind)
	root.Status = RootStatus(status)
	root.FeedType = RootFeedType(feedType)
	root.FeedState = RootFeedState(feedState)
	return root, nil
}
