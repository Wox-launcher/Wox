package filesearch

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
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
	progress_current, progress_total, last_error, dynamic_parent_root_id, policy_root_path, promoted_at,
	last_hot_at, created_at, updated_at
`

func NewFileSearchDB(ctx context.Context) (*FileSearchDB, error) {
	fileSearchDir := util.GetLocation().GetFileSearchDirectory()
	// Bug fix: manual full rebuilds remove the whole filesearch directory after
	// closing SQLite. Recreate the directory at DB-open time so both startup and
	// reset paths have one durable owner for the storage location.
	if err := util.GetLocation().EnsureDirectoryExist(fileSearchDir); err != nil {
		return nil, err
	}

	dbPath := filepath.Join(fileSearchDir, "directory.db")
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
		progress_current, progress_total, last_error, dynamic_parent_root_id, policy_root_path, promoted_at,
		last_hot_at, created_at, updated_at
	)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
		dynamic_parent_root_id = excluded.dynamic_parent_root_id,
		policy_root_path = excluded.policy_root_path,
		promoted_at = excluded.promoted_at,
		last_hot_at = excluded.last_hot_at,
		updated_at = excluded.updated_at
	`

	// Dynamic metadata is updated through the root upsert path because promotion
	// can collide with an existing path row. Keeping the hidden ownership fields
	// in the same conflict clause lets a user-added root cleanly take over a
	// former dynamic root without carrying stale parent policy state.
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
		root.DynamicParentRootID,
		root.PolicyRootPath,
		root.PromotedAt,
		root.LastHotAt,
		root.CreatedAt,
		root.UpdatedAt,
	)
	return err
}

func (d *FileSearchDB) UpdateRootState(ctx context.Context, root RootRecord) error {
	query := `
	UPDATE roots
	SET status = ?, feed_type = ?, feed_cursor = ?, feed_state = ?, last_reconcile_at = ?, last_full_scan_at = ?,
	    progress_current = ?, progress_total = ?, last_error = ?, dynamic_parent_root_id = ?, policy_root_path = ?,
	    promoted_at = ?, last_hot_at = ?, updated_at = ?
	WHERE id = ?
	`

	// LastHotAt changes after successful dirty flushes, while the rest of the
	// state update path is already serialized by the scanner loop. Persisting the
	// dynamic fields here avoids a separate lifecycle-only UPDATE and keeps root
	// state writes in one place.
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
		root.DynamicParentRootID,
		root.PolicyRootPath,
		root.PromotedAt,
		root.LastHotAt,
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
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM directories WHERE root_id = ?`, rootID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM roots WHERE id = ?`, rootID); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
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
	// index, not just enqueue another full scan. Keep user roots because they
	// are configuration, but drop all indexed facts and hidden dynamic roots so
	// the next preparation run rebuilds ownership and search artifacts from scratch.
	if _, err := tx.ExecContext(ctx, `DELETE FROM directories`); err != nil {
		return fmt.Errorf("reset directories: %w", err)
	}
	roots, err := d.ListRoots(ctx)
	if err != nil {
		return err
	}
	for _, root := range roots {
		if err := d.directory.detach(root.ID); err != nil {
			return err
		}
		if root.Kind != RootKindDynamic {
			if err := d.directory.attach(root); err != nil {
				return err
			}
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM roots WHERE kind = ?`, RootKindDynamic); err != nil {
		return fmt.Errorf("reset dynamic roots: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM meta WHERE key = ?`, lastFullIndexElapsedMsKey); err != nil {
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
		WHERE kind <> ?
	`, RootStatusPreparing, now, RootKindDynamic); err != nil {
		return fmt.Errorf("reset user roots: %w", err)
	}

	return tx.Commit()
}

func (d *FileSearchDB) MoveScopedRowsToRoot(ctx context.Context, fromRootID string, toRootID string, scopePath string) error {
	from, err := d.FindRootByID(ctx, fromRootID)
	if err != nil {
		return err
	}
	to, err := d.FindRootByID(ctx, toRootID)
	if err != nil {
		return err
	}
	if from == nil || to == nil {
		return fmt.Errorf("ownership root missing")
	}
	if err := d.copyColumnScope(ctx, *from, *to, scopePath); err != nil {
		return err
	}
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := moveScopedRowsToRootTx(ctx, tx, "directories", fromRootID, toRootID, scopePath); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if normalizePath(scopePath) == normalizePath(from.Path) {
		return nil
	}
	return d.directory.applyEntries(ctx, from.ID, nil, []string{scopePath}, from.FeedCursor)
}

func (d *FileSearchDB) PromoteDynamicRoot(ctx context.Context, parentRoot RootRecord, dynamicRoot RootRecord) error {
	if dynamicRoot.Kind != RootKindDynamic {
		return fmt.Errorf("invalid dynamic root kind")
	}
	if err := d.copyColumnScope(ctx, parentRoot, dynamicRoot, dynamicRoot.Path); err != nil {
		return err
	}
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := execRootUpsert(ctx, tx, dynamicRoot); err != nil {
		return err
	}
	if err := moveScopedRowsToRootTx(ctx, tx, "directories", parentRoot.ID, dynamicRoot.ID, dynamicRoot.Path); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return d.directory.applyEntries(ctx, parentRoot.ID, nil, []string{dynamicRoot.Path}, parentRoot.FeedCursor)
}

func (d *FileSearchDB) DemoteDynamicRoot(ctx context.Context, parentRoot RootRecord, dynamicRoot RootRecord) error {
	if dynamicRoot.Kind != RootKindDynamic {
		return fmt.Errorf("invalid dynamic root kind")
	}
	if err := d.copyColumnScope(ctx, dynamicRoot, parentRoot, dynamicRoot.Path); err != nil {
		return err
	}
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := moveScopedRowsToRootTx(ctx, tx, "directories", dynamicRoot.ID, parentRoot.ID, dynamicRoot.Path); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM roots WHERE id = ?`, dynamicRoot.ID); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return d.directory.detach(dynamicRoot.ID)
}

func moveScopedRowsToRootTx(ctx context.Context, tx *sql.Tx, table string, fromRootID string, toRootID string, scopePath string) error {
	scopeClause, scopeArgs := buildScopedPathQuery(scopePath, "path")
	args := append([]any{toRootID, fromRootID}, scopeArgs...)
	_, err := tx.ExecContext(ctx, fmt.Sprintf(`
		UPDATE %s
		SET root_id = ?
		WHERE root_id = ?
		  AND %s
	`, table, scopeClause), args...)
	return err
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

func (d *FileSearchDB) ApplyDirectFilesJobStream(ctx context.Context, root RootRecord, job Job, snapshot *SnapshotBuilder, onProgress func(JobApplyStats)) (JobApplyStats, error) {
	return d.applyColumnJob(ctx, root, job, snapshot, onProgress, true)
}

func (d *FileSearchDB) ApplySubtreeJobStream(ctx context.Context, root RootRecord, job Job, snapshot *SnapshotBuilder, onProgress func(JobApplyStats)) (JobApplyStats, error) {
	return d.applyColumnJob(ctx, root, job, snapshot, onProgress, false)
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
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM directories WHERE root_id = ? AND "exists" = 0`, root.ID); err != nil {
		return err
	}
	if err := d.finalizeRootRunTx(ctx, tx, root); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
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

func prepareDirectoryUpsertStmtTx(ctx context.Context, tx *sql.Tx) (*sql.Stmt, error) {
	return tx.PrepareContext(ctx, `
		INSERT INTO directories (path, root_id, parent_path, last_scan_time, "exists")
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET
			root_id = excluded.root_id,
			parent_path = excluded.parent_path,
			last_scan_time = excluded.last_scan_time,
			"exists" = excluded."exists"
	`)
}

func upsertDirectoryRecordsBatchTx(ctx context.Context, tx *sql.Tx, directories []DirectoryRecord) error {
	for start := 0; start < len(directories); {
		chunkSize := sqliteBatchRows(len(directories)-start, directoryColumnCount)
		end := start + chunkSize
		if err := writeDirectoryRecordRowsBatchTx(ctx, tx, directories[start:end], false); err != nil {
			return err
		}
		start = end
	}
	return nil
}

// insertDirectoryRecordsBatchTx skips conflict handling for fresh full-index
// roots, where the sealed plan owns non-overlapping directory paths.
func insertDirectoryRecordsBatchTx(ctx context.Context, tx *sql.Tx, directories []DirectoryRecord) error {
	for start := 0; start < len(directories); {
		chunkSize := sqliteBatchRows(len(directories)-start, directoryColumnCount)
		end := start + chunkSize
		if err := writeDirectoryRecordRowsBatchTx(ctx, tx, directories[start:end], true); err != nil {
			return err
		}
		start = end
	}
	return nil
}

// writeDirectoryRecordRowsBatchTx writes one chunk of directory facts and can
// optionally omit ON CONFLICT for fresh bulk-loads.
func writeDirectoryRecordRowsBatchTx(ctx context.Context, tx *sql.Tx, directories []DirectoryRecord, plainInsert bool) error {
	if len(directories) == 0 {
		return nil
	}

	var builder strings.Builder
	builder.WriteString(`
		INSERT INTO directories (path, root_id, parent_path, last_scan_time, "exists")
		VALUES `)

	args := make([]any, 0, len(directories)*directoryColumnCount)
	for index, directory := range directories {
		if index > 0 {
			builder.WriteString(",")
		}
		builder.WriteString("(?, ?, ?, ?, ?)")
		args = append(args,
			directory.Path,
			directory.RootID,
			directory.ParentPath,
			directory.LastScanTime,
			boolToInt(directory.Exists),
		)
	}

	if !plainInsert {
		// Optimization: fresh full-index streams can contain thousands of directories.
		// Writing them in chunks keeps the same path-conflict behavior while removing
		// one sqlite3_step round trip per directory.
		builder.WriteString(`
			ON CONFLICT(path) DO UPDATE SET
				root_id = excluded.root_id,
				parent_path = excluded.parent_path,
				last_scan_time = excluded.last_scan_time,
				"exists" = excluded."exists"
		`)
	}
	if _, err := tx.ExecContext(ctx, builder.String(), args...); err != nil {
		operation := "upsert"
		if plainInsert {
			operation = "insert"
		}
		return fmt.Errorf("batch %s %d directories: %w", operation, len(directories), err)
	}
	return nil
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

func subtreeBatchScanTime(batch SubtreeSnapshotBatch) int64 {
	scanTime := int64(0)
	for _, directory := range batch.Directories {
		if directory.LastScanTime > scanTime {
			scanTime = directory.LastScanTime
		}
	}
	return scanTime
}

func tombstoneScopedDirectories(tx *sql.Tx, ctx context.Context, rootID string, scopePath string, scanTime int64) error {
	scopeClause, scopeArgs := buildScopedPathQuery(scopePath, "path")
	// Full rescans were paying root-wide readback and one UPDATE per directory
	// before any subtree rows were restored. Reusing the same scoped path
	// predicate inside SQLite keeps the exact ownership boundary while removing
	// the high fixed cost of filtering and replaying tombstones in Go.
	args := append([]any{scanTime, rootID}, scopeArgs...)
	_, err := tx.ExecContext(ctx, fmt.Sprintf(`
		UPDATE directories
		SET "exists" = 0, last_scan_time = ?
		WHERE root_id = ?
		  AND "exists" = 1
		  AND %s
	`, scopeClause), args...)
	return err
}

func (d *FileSearchDB) DeleteDirectoryTombstones(ctx context.Context, rootID string) error {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		DELETE FROM directories
		WHERE root_id = ?
		  AND "exists" = 0
	`, rootID); err != nil {
		return err
	}

	return tx.Commit()
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

func lockRootForSubtreeSnapshot(ctx context.Context, tx *sql.Tx, rootID string) (*RootRecord, error) {
	result, err := tx.ExecContext(ctx, `
		UPDATE roots
		SET updated_at = updated_at
		WHERE id = ?
	`, rootID)
	if err != nil {
		return nil, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if rowsAffected == 0 {
		return nil, fmt.Errorf("root %q not found", rootID)
	}

	row := tx.QueryRowContext(ctx, `
		SELECT `+rootRecordSelectColumns+`
		FROM roots
		WHERE id = ?
	`, rootID)

	root, err := scanRootRecord(row)
	if err != nil {
		return nil, err
	}

	return &root, nil
}

func deleteScopedRows(tx *sql.Tx, ctx context.Context, table string, rootID string, scopePath string) error {
	rows, err := tx.QueryContext(ctx, fmt.Sprintf(`SELECT path FROM %s WHERE root_id = ?`, table), rootID)
	if err != nil {
		return err
	}
	defer rows.Close()

	var paths []string
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return err
		}
		if pathWithinScope(scopePath, path) {
			paths = append(paths, path)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, path := range paths {
		if _, err := tx.ExecContext(ctx, fmt.Sprintf(`DELETE FROM %s WHERE root_id = ? AND path = ?`, table), rootID, path); err != nil {
			return err
		}
	}

	return nil
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

func (d *FileSearchDB) ListDirectoriesByRoot(ctx context.Context, rootID string) ([]DirectoryRecord, error) {
	rows, err := d.db.QueryContext(ctx, `
		SELECT path, root_id, parent_path, last_scan_time, "exists"
		FROM directories
		WHERE root_id = ?
		ORDER BY path ASC
	`, rootID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var directories []DirectoryRecord
	for rows.Next() {
		var directory DirectoryRecord
		var exists int
		if err := rows.Scan(
			&directory.Path,
			&directory.RootID,
			&directory.ParentPath,
			&directory.LastScanTime,
			&exists,
		); err != nil {
			return nil, err
		}
		directory.Exists = exists == 1
		directories = append(directories, directory)
	}

	return directories, rows.Err()
}

func (d *FileSearchDB) CountDirectoriesByRoot(ctx context.Context, rootID string) (int, error) {
	row := d.db.QueryRowContext(ctx, `
		SELECT count(*)
		FROM directories
		WHERE root_id = ? AND "exists" = 1
	`, rootID)

	var count int
	if err := row.Scan(&count); err != nil {
		return 0, err
	}

	return count, nil
}

func (d *FileSearchDB) ListEntries(ctx context.Context) ([]EntryRecord, error) {
	return d.columnEntries(ctx, "")
}

func (d *FileSearchDB) ListEntriesByRoot(ctx context.Context, rootID string) ([]EntryRecord, error) {
	return d.columnEntries(ctx, rootID)
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
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
		&root.DynamicParentRootID,
		&root.PolicyRootPath,
		&root.PromotedAt,
		&root.LastHotAt,
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
