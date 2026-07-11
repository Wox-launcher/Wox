package filesearch

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"wox/util"

	_ "github.com/mattn/go-sqlite3"
)

type FileSearchDB struct {
	db     *sql.DB
	dbPath string
	// searchArtifactsNeedRebuild is set during schema init when derived search
	// tables must be rebuilt outside the startup-critical DB open path.
	searchArtifactsNeedRebuild bool
	// Bulk sync mode defers expensive FTS maintenance until the full scan cycle
	// finishes. The previous all-at-once in-memory index build avoided per-entry
	// write amplification, so the SQLite-first path needs an explicit bulk gate
	// to keep full rescans from thrashing the FTS tables.
	bulkSyncMu    sync.Mutex
	bulkSyncDepth int
	// Full-run leaf scopes are sealed, non-overlapping ownership boundaries.
	// Remembering which roots started empty lets later subtree/direct-files
	// applies skip repeated "does this scope already exist?" probes without
	// widening delete ownership or changing query-time semantics.
	bulkSyncFullRunRoots        map[string]bulkSyncFullRunRootState
	bulkSyncEntryIndexesChecked bool
	bulkSyncEntryIndexesDropped bool
	entryIndexMaintenanceMu     sync.Mutex
}

const rootRecordSelectColumns = `
	id, path, kind, status, feed_type, feed_cursor, feed_state, last_reconcile_at, last_full_scan_at,
	progress_current, progress_total, last_error, dynamic_parent_root_id, policy_root_path, promoted_at,
	last_hot_at, created_at, updated_at
`

type bulkSyncFullRunRootState struct {
	prepared     bool
	freshAtStart bool
}

func NewFileSearchDB(ctx context.Context) (*FileSearchDB, error) {
	fileSearchDir := util.GetLocation().GetFileSearchDirectory()
	// Bug fix: manual full rebuilds remove the whole filesearch directory after
	// closing SQLite. Recreate the directory at DB-open time so both startup and
	// reset paths have one durable owner for the storage location.
	if err := util.GetLocation().EnsureDirectoryExist(fileSearchDir); err != nil {
		return nil, err
	}

	dbPath := filepath.Join(fileSearchDir, "filesearch.db")
	dsn := dbPath + "?" +
		"_journal_mode=WAL&" +
		"_synchronous=NORMAL&" +
		"_cache_size=2000&" +
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
	db.SetMaxIdleConns(4)
	db.SetConnMaxLifetime(time.Hour)

	fileSearchDB := &FileSearchDB{db: db, dbPath: dbPath}
	if err := fileSearchDB.initTables(ctx); err != nil {
		db.Close()
		return nil, err
	}

	return fileSearchDB, nil
}

func (d *FileSearchDB) Close() error {
	if d == nil || d.db == nil {
		return nil
	}
	return d.db.Close()
}

func (d *FileSearchDB) initTables(ctx context.Context) error {
	if err := d.ensureBaseTables(ctx); err != nil {
		return err
	}
	if err := d.ensureSQLiteSearchSchema(ctx); err != nil {
		return err
	}
	return nil
}

func (d *FileSearchDB) UpsertRoot(ctx context.Context, root RootRecord) error {
	return execRootUpsert(ctx, d.db, root)
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

	// Delete search artifacts from the persisted facts so root removal no longer
	// depends on the in-memory snapshot helpers that were removed by the SQLite-first path.
	rows, err := selectStoredEntriesTx(ctx, tx, `
		SELECT entry_id, path, root_id, parent_path, name, normalized_name, name_key, normalized_path,
		       pinyin_full, pinyin_initials, extension, is_dir, mtime, size, updated_at
		FROM entries
		WHERE root_id = ?
		ORDER BY path ASC
	`, rootID)
	if err != nil {
		return err
	}
	artifactSync, err := newEntrySearchArtifactSyncTx(ctx, tx)
	if err != nil {
		return err
	}
	defer artifactSync.Close()
	for _, row := range rows {
		if err := deleteEntrySearchArtifactsWithSyncTx(ctx, artifactSync, row); err != nil {
			return err
		}
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM directories WHERE root_id = ?`, rootID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM entries WHERE root_id = ?`, rootID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM roots WHERE id = ?`, rootID); err != nil {
		return err
	}

	return tx.Commit()
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
	if _, err := tx.ExecContext(ctx, `DELETE FROM entries`); err != nil {
		return fmt.Errorf("reset entries: %w", err)
	}
	if err := rebuildAllSearchArtifactsTx(ctx, tx); err != nil {
		return fmt.Errorf("reset search artifacts: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM roots WHERE kind = ?`, RootKindDynamic); err != nil {
		return fmt.Errorf("reset dynamic roots: %w", err)
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
	if strings.TrimSpace(fromRootID) == "" || strings.TrimSpace(toRootID) == "" {
		return fmt.Errorf("move scoped rows requires both source and target root ids")
	}
	if strings.TrimSpace(scopePath) == "" {
		return fmt.Errorf("move scoped rows requires a scope path")
	}

	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Promotion and demotion only change ownership facts; entry_id and derived
	// search artifacts stay valid because query semantics are path based. Using
	// the existing scoped SQL predicate keeps this transaction bounded to the
	// dynamic subtree without reading every row back into Go.
	if err := moveScopedRowsToRootTx(ctx, tx, "directories", fromRootID, toRootID, scopePath); err != nil {
		return err
	}
	if err := moveScopedRowsToRootTx(ctx, tx, "entries", fromRootID, toRootID, scopePath); err != nil {
		return err
	}

	return tx.Commit()
}

func (d *FileSearchDB) PromoteDynamicRoot(ctx context.Context, parentRoot RootRecord, dynamicRoot RootRecord) error {
	if dynamicRoot.Kind != RootKindDynamic {
		return fmt.Errorf("promote dynamic root requires kind %q, got %q", RootKindDynamic, dynamicRoot.Kind)
	}
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Promotion must create the hidden root and move scoped ownership together.
	// If those writes were split, a parent reconcile between them could observe
	// duplicate ownership and use entries.path's unique upsert to take the path
	// back before the dynamic root ever gets a clean snapshot.
	if err := execRootUpsert(ctx, tx, dynamicRoot); err != nil {
		return err
	}
	if err := moveScopedRowsToRootTx(ctx, tx, "directories", parentRoot.ID, dynamicRoot.ID, dynamicRoot.Path); err != nil {
		return err
	}
	if err := moveScopedRowsToRootTx(ctx, tx, "entries", parentRoot.ID, dynamicRoot.ID, dynamicRoot.Path); err != nil {
		return err
	}

	return tx.Commit()
}

func (d *FileSearchDB) DemoteDynamicRoot(ctx context.Context, parentRoot RootRecord, dynamicRoot RootRecord) error {
	if dynamicRoot.Kind != RootKindDynamic {
		return fmt.Errorf("demote dynamic root requires kind %q, got %q", RootKindDynamic, dynamicRoot.Kind)
	}
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Demotion is the inverse ownership move plus a DELETE of the hidden root.
	// Keeping it transactional avoids a stale dynamic root row that would keep
	// excluding the path from the parent even after rows had moved back.
	if err := moveScopedRowsToRootTx(ctx, tx, "directories", dynamicRoot.ID, parentRoot.ID, dynamicRoot.Path); err != nil {
		return err
	}
	if err := moveScopedRowsToRootTx(ctx, tx, "entries", dynamicRoot.ID, parentRoot.ID, dynamicRoot.Path); err != nil {
		return err
	}

	rows, err := selectStoredEntriesTx(ctx, tx, `
		SELECT entry_id, path, root_id, parent_path, name, normalized_name, name_key, normalized_path,
		       pinyin_full, pinyin_initials, extension, is_dir, mtime, size, updated_at
		FROM entries
		WHERE root_id = ?
		ORDER BY path ASC
	`, dynamicRoot.ID)
	if err != nil {
		return err
	}
	artifactSync, err := newEntrySearchArtifactSyncTx(ctx, tx)
	if err != nil {
		return err
	}
	defer artifactSync.Close()
	for _, row := range rows {
		if err := deleteEntrySearchArtifactsWithSyncTx(ctx, artifactSync, row); err != nil {
			return err
		}
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM directories WHERE root_id = ?`, dynamicRoot.ID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM entries WHERE root_id = ?`, dynamicRoot.ID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM roots WHERE id = ?`, dynamicRoot.ID); err != nil {
		return err
	}

	return tx.Commit()
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
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := validateSubtreeSnapshotBatch(SubtreeSnapshotBatch{
		RootID:      root.ID,
		ScopePath:   root.Path,
		Directories: directories,
		Entries:     entries,
	}); err != nil {
		return err
	}

	reportProgress := func(progress ReplaceEntriesProgress) {
		if onProgress == nil {
			return
		}
		onProgress(progress)
	}

	reportProgress(ReplaceEntriesProgress{Stage: ReplaceEntriesStagePreparing})

	if _, err := tx.ExecContext(ctx, `DELETE FROM directories WHERE root_id = ?`, root.ID); err != nil {
		return err
	}

	directoryStmt, err := tx.PrepareContext(ctx, `
		INSERT INTO directories (path, root_id, parent_path, last_scan_time, "exists")
		VALUES (?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer directoryStmt.Close()

	for _, directory := range directories {
		if _, err := directoryStmt.ExecContext(
			ctx,
			directory.Path,
			directory.RootID,
			directory.ParentPath,
			directory.LastScanTime,
			boolToInt(directory.Exists),
		); err != nil {
			return err
		}
	}

	totalEntries := int64(len(entries))
	// Root replacement used to delete every row and reinsert it, which changed
	// rowids and broke any external index keyed by the persisted entry identity.
	// The SQLite search tables now depend on stable entry_id values, so root
	// snapshots must upsert facts and delete only the stale paths.
	if totalEntries == 0 {
		reportProgress(ReplaceEntriesProgress{
			Stage:   ReplaceEntriesStageWriting,
			Current: 1,
			Total:   1,
		})
	} else {
		reportProgress(ReplaceEntriesProgress{
			Stage: ReplaceEntriesStageWriting,
			Total: totalEntries,
		})
	}
	if err := d.replaceRootEntriesTx(ctx, tx, root, entries, func(current int64, total int64) {
		reportProgress(ReplaceEntriesProgress{
			Stage:   ReplaceEntriesStageWriting,
			Current: current,
			Total:   total,
		})
	}); err != nil {
		return err
	}
	reportProgress(ReplaceEntriesProgress{Stage: ReplaceEntriesStageFinalizing})

	if err := tx.Commit(); err != nil {
		return err
	}

	return nil
}

func (d *FileSearchDB) ReplaceRootEntries(ctx context.Context, root RootRecord, entries []EntryRecord, onProgress func(ReplaceEntriesProgress)) error {
	return d.ReplaceRootSnapshot(ctx, root, nil, entries, onProgress)
}

func (d *FileSearchDB) ApplyDirectFilesJob(ctx context.Context, job Job, batch SubtreeSnapshotBatch) error {
	if job.Kind != JobKindDirectFiles {
		return fmt.Errorf("apply direct-files job requires kind %q, got %q", JobKindDirectFiles, job.Kind)
	}
	if err := validateJobSnapshotBatch(job, batch); err != nil {
		return err
	}

	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	root, err := lockRootForSubtreeSnapshot(ctx, tx, batch.RootID)
	if err != nil {
		return err
	}
	if !pathWithinScope(root.Path, batch.ScopePath) {
		return fmt.Errorf("direct-files job scope path %q is outside root path %q", batch.ScopePath, root.Path)
	}

	directoryStmt, err := prepareDirectoryUpsertStmtTx(ctx, tx)
	if err != nil {
		return err
	}
	defer directoryStmt.Close()

	for _, directory := range batch.Directories {
		if _, err := directoryStmt.ExecContext(
			ctx,
			directory.Path,
			directory.RootID,
			directory.ParentPath,
			directory.LastScanTime,
			boolToInt(directory.Exists),
		); err != nil {
			return err
		}
	}

	// Bounded direct-file jobs used to share the root-wide replace path, which
	// deleted sibling chunks and subtree scopes before their own jobs ran. The
	// job-oriented path now upserts only the rows owned by this job so replay is
	// safe and unrelated sibling scopes remain intact until their jobs apply.
	if err := d.applyDirectFilesEntriesTx(ctx, tx, batch); err != nil {
		return err
	}

	return tx.Commit()
}

func (d *FileSearchDB) ApplyDirectFilesJobStream(ctx context.Context, root RootRecord, job Job, snapshot *SnapshotBuilder, onProgress func(JobApplyStats)) (JobApplyStats, error) {
	if job.Kind != JobKindDirectFiles {
		return JobApplyStats{}, fmt.Errorf("apply direct-files stream requires kind %q, got %q", JobKindDirectFiles, job.Kind)
	}
	if snapshot == nil {
		return JobApplyStats{}, fmt.Errorf("direct-files stream requires a snapshot builder")
	}

	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return JobApplyStats{}, err
	}
	defer tx.Rollback()

	lockedRoot, err := lockRootForSubtreeSnapshot(ctx, tx, root.ID)
	if err != nil {
		return JobApplyStats{}, err
	}
	if !pathWithinScope(lockedRoot.Path, job.ScopePath) {
		return JobApplyStats{}, fmt.Errorf("direct-files job scope path %q is outside root path %q", job.ScopePath, lockedRoot.Path)
	}

	if d.bulkSyncFullRunRootFresh(root.ID) {
		// A fresh full-index direct-files scope has no stale facts to diff, so it
		// can stream facts directly and leave FTS maintenance to EndBulkSync.
		stats := JobApplyStats{}
		streamStartedAt := util.GetSystemTimestamp()
		directoryWriteElapsedMs := int64(0)
		entryWriteElapsedMs := int64(0)
		insertedDirectories := 0
		insertedEntries := 0
		if err := snapshot.StreamDirectFilesJobBatches(ctx, *lockedRoot, job, func(batch SubtreeSnapshotBatch) error {
			if err := validateJobSnapshotBatch(job, batch); err != nil {
				return err
			}

			directoryStartedAt := util.GetSystemTimestamp()
			if err := insertDirectoryRecordsBatchTx(ctx, tx, batch.Directories); err != nil {
				return err
			}
			directoryWriteElapsedMs += util.GetSystemTimestamp() - directoryStartedAt

			entryStartedAt := util.GetSystemTimestamp()
			if err := insertEntryFactsNoReturningBatchTx(ctx, tx, batch.Entries); err != nil {
				return err
			}
			entryWriteElapsedMs += util.GetSystemTimestamp() - entryStartedAt
			insertedDirectories += len(batch.Directories)
			insertedEntries += len(batch.Entries)
			stats.add(jobApplyStatsFromBatch(batch))
			if onProgress != nil {
				onProgress(stats)
			}
			return nil
		}); err != nil {
			return JobApplyStats{}, err
		}
		streamElapsedMs := util.GetSystemTimestamp() - streamStartedAt

		commitStartedAt := util.GetSystemTimestamp()
		if err := tx.Commit(); err != nil {
			return JobApplyStats{}, err
		}
		commitElapsedMs := util.GetSystemTimestamp() - commitStartedAt
		scanBuildElapsedMs := streamElapsedMs - directoryWriteElapsedMs - entryWriteElapsedMs
		if scanBuildElapsedMs < 0 {
			scanBuildElapsedMs = 0
		}
		logFilesearchIndexPhase(ctx, "direct_files_stream_fresh", job.ScopePath, streamElapsedMs+commitElapsedMs, map[string]any{
			"commit_ms":     commitElapsedMs,
			"directories":   insertedDirectories,
			"directory_ms":  directoryWriteElapsedMs,
			"entries":       insertedEntries,
			"entry_ms":      entryWriteElapsedMs,
			"scan_build_ms": scanBuildElapsedMs,
		})
		return stats, nil
	}

	directoryStmt, err := prepareDirectoryUpsertStmtTx(ctx, tx)
	if err != nil {
		return JobApplyStats{}, err
	}
	defer directoryStmt.Close()

	stageStmt, err := prepareEntryStageInsertStmtTx(ctx, tx)
	if err != nil {
		return JobApplyStats{}, err
	}
	defer stageStmt.Close()

	stats := JobApplyStats{}
	streamStartedAt := util.GetSystemTimestamp()
	directoryWriteElapsedMs := int64(0)
	stageWriteElapsedMs := int64(0)
	insertedDirectories := 0
	stagedEntries := 0
	// Direct-files jobs now own the whole directory scope. Streaming each batch
	// into the temporary stage table keeps SQLite writes bounded without losing
	// the single-scope stale prune that chunked jobs could not express safely.
	if err := snapshot.StreamDirectFilesJobBatches(ctx, *lockedRoot, job, func(batch SubtreeSnapshotBatch) error {
		if err := validateJobSnapshotBatch(job, batch); err != nil {
			return err
		}
		directoryStartedAt := util.GetSystemTimestamp()
		for _, directory := range batch.Directories {
			if _, err := directoryStmt.ExecContext(
				ctx,
				directory.Path,
				directory.RootID,
				directory.ParentPath,
				directory.LastScanTime,
				boolToInt(directory.Exists),
			); err != nil {
				return err
			}
		}
		directoryWriteElapsedMs += util.GetSystemTimestamp() - directoryStartedAt
		stageStartedAt := util.GetSystemTimestamp()
		if err := stageEntryRecordsWithStmtTx(ctx, stageStmt, batch.Entries); err != nil {
			return err
		}
		stageWriteElapsedMs += util.GetSystemTimestamp() - stageStartedAt
		insertedDirectories += len(batch.Directories)
		stagedEntries += len(batch.Entries)
		stats.add(jobApplyStatsFromBatch(batch))
		if onProgress != nil {
			// Streaming toolbar counts are emitted only after the batch has been
			// staged successfully, so the UI never gets ahead of accepted work.
			onProgress(stats)
		}
		return nil
	}); err != nil {
		return JobApplyStats{}, err
	}
	streamElapsedMs := util.GetSystemTimestamp() - streamStartedAt

	replaceStartedAt := util.GetSystemTimestamp()
	if err := d.replaceDirectFilesEntriesFromStageTx(ctx, tx, job.RootID, job.ScopePath); err != nil {
		return JobApplyStats{}, err
	}
	replaceElapsedMs := util.GetSystemTimestamp() - replaceStartedAt

	commitStartedAt := util.GetSystemTimestamp()
	if err := tx.Commit(); err != nil {
		return JobApplyStats{}, err
	}
	commitElapsedMs := util.GetSystemTimestamp() - commitStartedAt
	scanBuildElapsedMs := streamElapsedMs - directoryWriteElapsedMs - stageWriteElapsedMs
	if scanBuildElapsedMs < 0 {
		scanBuildElapsedMs = 0
	}
	logFilesearchIndexPhase(ctx, "direct_files_stream", job.ScopePath, streamElapsedMs+replaceElapsedMs+commitElapsedMs, map[string]any{
		"commit_ms":        commitElapsedMs,
		"directories":      insertedDirectories,
		"directory_ms":     directoryWriteElapsedMs,
		"entries":          stagedEntries,
		"replace_ms":       replaceElapsedMs,
		"scan_build_ms":    scanBuildElapsedMs,
		"stage_entries_ms": stageWriteElapsedMs,
	})
	return stats, nil
}

func (d *FileSearchDB) ApplySubtreeJobStream(ctx context.Context, root RootRecord, job Job, snapshot *SnapshotBuilder, onProgress func(JobApplyStats)) (JobApplyStats, error) {
	if job.Kind != JobKindSubtree {
		return JobApplyStats{}, fmt.Errorf("apply subtree stream requires kind %q, got %q", JobKindSubtree, job.Kind)
	}
	if snapshot == nil {
		return JobApplyStats{}, fmt.Errorf("subtree stream requires a snapshot builder")
	}

	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return JobApplyStats{}, err
	}
	defer tx.Rollback()

	lockedRoot, err := lockRootForSubtreeSnapshot(ctx, tx, root.ID)
	if err != nil {
		return JobApplyStats{}, err
	}
	if !pathWithinScope(lockedRoot.Path, job.ScopePath) {
		return JobApplyStats{}, fmt.Errorf("subtree job scope path %q is outside root path %q", job.ScopePath, lockedRoot.Path)
	}

	if d.bulkSyncFullRunRootFresh(root.ID) {
		// Optimization: a fresh full-index root has no stale facts to diff or
		// prune. Stream batches straight into the fact tables and let EndBulkSync
		// rebuild FTS/bigram artifacts once, removing both the duplicate preparation
		// walk and the temp-stage copy for first-time large roots. The hot path now
		// uses chunked multi-row writes so a workstation-sized tree does not pay one
		// SQLite round trip per indexed file.
		streamStartedAt := util.GetSystemTimestamp()
		directoryWriteElapsedMs := int64(0)
		entryWriteElapsedMs := int64(0)
		insertedDirectories := 0
		insertedEntries := 0
		stats := JobApplyStats{}
		if err := snapshot.StreamSubtreeJobBatches(ctx, *lockedRoot, job, func(batch SubtreeSnapshotBatch) error {
			if err := validateJobSnapshotBatch(job, batch); err != nil {
				return err
			}

			directoryStartedAt := util.GetSystemTimestamp()
			if err := insertDirectoryRecordsBatchTx(ctx, tx, batch.Directories); err != nil {
				return err
			}
			directoryWriteElapsedMs += util.GetSystemTimestamp() - directoryStartedAt

			entryStartedAt := util.GetSystemTimestamp()
			if err := insertEntryFactsNoReturningBatchTx(ctx, tx, batch.Entries); err != nil {
				return err
			}
			entryWriteElapsedMs += util.GetSystemTimestamp() - entryStartedAt
			insertedDirectories += len(batch.Directories)
			insertedEntries += len(batch.Entries)
			stats.add(jobApplyStatsFromBatch(batch))
			if onProgress != nil {
				// Fresh full-index streams write facts directly, so batch progress
				// can be reported immediately after each successful write.
				onProgress(stats)
			}
			return nil
		}); err != nil {
			return JobApplyStats{}, err
		}
		streamElapsedMs := util.GetSystemTimestamp() - streamStartedAt
		scanBuildElapsedMs := streamElapsedMs - directoryWriteElapsedMs - entryWriteElapsedMs
		if scanBuildElapsedMs < 0 {
			scanBuildElapsedMs = 0
		}
		logFilesearchSQLiteMaintenance(ctx, "subtree_stream_scan_build", job.ScopePath, scanBuildElapsedMs, insertedDirectories+insertedEntries)
		logFilesearchSQLiteMaintenance(ctx, "subtree_stream_fresh_directories", job.ScopePath, directoryWriteElapsedMs, insertedDirectories)
		logFilesearchSQLiteMaintenance(ctx, "subtree_stream_fresh_entries", job.ScopePath, entryWriteElapsedMs, insertedEntries)

		commitStartedAt := util.GetSystemTimestamp()
		if err := tx.Commit(); err != nil {
			return JobApplyStats{}, err
		}
		commitElapsedMs := util.GetSystemTimestamp() - commitStartedAt
		logFilesearchSQLiteMaintenance(ctx, "subtree_stream_fresh_commit", job.ScopePath, commitElapsedMs, 1)
		logFilesearchSQLiteMaintenance(ctx, "subtree_stream_fresh_insert", job.ScopePath, streamElapsedMs+commitElapsedMs, insertedEntries)
		logFilesearchIndexPhase(ctx, "subtree_stream_fresh", job.ScopePath, streamElapsedMs+commitElapsedMs, map[string]any{
			"commit_ms":     commitElapsedMs,
			"directories":   insertedDirectories,
			"directory_ms":  directoryWriteElapsedMs,
			"entries":       insertedEntries,
			"entry_ms":      entryWriteElapsedMs,
			"scan_build_ms": scanBuildElapsedMs,
		})
		return stats, nil
	}

	directoryStmt, err := prepareDirectoryUpsertStmtTx(ctx, tx)
	if err != nil {
		return JobApplyStats{}, err
	}
	defer directoryStmt.Close()

	stageStmt, err := prepareEntryStageInsertStmtTx(ctx, tx)
	if err != nil {
		return JobApplyStats{}, err
	}
	defer stageStmt.Close()

	streamStartedAt := util.GetSystemTimestamp()
	maxScanTime := int64(0)
	stagedEntries := 0
	stagedDirectories := 0
	directoryWriteElapsedMs := int64(0)
	stageWriteElapsedMs := int64(0)
	stats := JobApplyStats{}
	if err := snapshot.StreamSubtreeJobBatches(ctx, *lockedRoot, job, func(batch SubtreeSnapshotBatch) error {
		if err := validateJobSnapshotBatch(job, batch); err != nil {
			return err
		}
		if scanTime := subtreeBatchScanTime(batch); scanTime > maxScanTime {
			maxScanTime = scanTime
		}
		directoryStartedAt := util.GetSystemTimestamp()
		for _, directory := range batch.Directories {
			if _, err := directoryStmt.ExecContext(
				ctx,
				directory.Path,
				directory.RootID,
				directory.ParentPath,
				directory.LastScanTime,
				boolToInt(directory.Exists),
			); err != nil {
				return err
			}
		}
		directoryWriteElapsedMs += util.GetSystemTimestamp() - directoryStartedAt
		stageStartedAt := util.GetSystemTimestamp()
		if err := stageEntryRecordsWithStmtTx(ctx, stageStmt, batch.Entries); err != nil {
			return err
		}
		stageWriteElapsedMs += util.GetSystemTimestamp() - stageStartedAt
		stagedDirectories += len(batch.Directories)
		stagedEntries += len(batch.Entries)
		stats.add(jobApplyStatsFromBatch(batch))
		if onProgress != nil {
			// Non-fresh streams first stage rows and then replay the scoped diff.
			// Reporting staged progress still reflects real scan/write work while
			// keeping the final committed count correction at job completion.
			onProgress(stats)
		}
		return nil
	}); err != nil {
		return JobApplyStats{}, err
	}
	streamElapsedMs := util.GetSystemTimestamp() - streamStartedAt
	scanBuildElapsedMs := streamElapsedMs - directoryWriteElapsedMs - stageWriteElapsedMs
	if scanBuildElapsedMs < 0 {
		scanBuildElapsedMs = 0
	}
	logFilesearchSQLiteMaintenance(ctx, "subtree_stream_stage_entries", job.ScopePath, 0, stagedEntries)

	tombstoneStartedAt := util.GetSystemTimestamp()
	if err := tombstoneScopedDirectories(tx, ctx, job.RootID, job.ScopePath, maxScanTime); err != nil {
		return JobApplyStats{}, err
	}
	logFilesearchSQLiteMaintenance(ctx, "subtree_stream_tombstone_directories", job.ScopePath, util.GetSystemTimestamp()-tombstoneStartedAt, 1)

	replaceStartedAt := util.GetSystemTimestamp()
	if err := d.replaceSubtreeEntriesFromStageTx(ctx, tx, job.RootID, job.ScopePath); err != nil {
		return JobApplyStats{}, err
	}
	replaceElapsedMs := util.GetSystemTimestamp() - replaceStartedAt
	logFilesearchSQLiteMaintenance(ctx, "subtree_stream_replace_entries", job.ScopePath, replaceElapsedMs, stagedEntries)

	commitStartedAt := util.GetSystemTimestamp()
	if err := tx.Commit(); err != nil {
		return JobApplyStats{}, err
	}
	commitElapsedMs := util.GetSystemTimestamp() - commitStartedAt
	logFilesearchIndexPhase(ctx, "subtree_stream", job.ScopePath, streamElapsedMs+replaceElapsedMs+commitElapsedMs, map[string]any{
		"commit_ms":        commitElapsedMs,
		"directories":      stagedDirectories,
		"directory_ms":     directoryWriteElapsedMs,
		"entries":          stagedEntries,
		"replace_ms":       replaceElapsedMs,
		"scan_build_ms":    scanBuildElapsedMs,
		"stage_entries_ms": stageWriteElapsedMs,
	})
	return stats, nil
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
	if job.Kind != JobKindDirectDelta {
		return fmt.Errorf("apply direct-delta job requires kind %q, got %q", JobKindDirectDelta, job.Kind)
	}
	if policy == nil {
		policy = newPolicyState(Policy{})
	}

	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	lockedRoot, err := lockRootForSubtreeSnapshot(ctx, tx, root.ID)
	if err != nil {
		return err
	}

	artifactSync, err := newEntrySearchArtifactSyncTx(ctx, tx)
	if err != nil {
		return err
	}
	defer artifactSync.Close()

	factMutator, err := newEntryFactMutatorTx(ctx, tx)
	if err != nil {
		return err
	}
	defer factMutator.Close()

	// Feature addition: known file watcher events apply through exact path
	// mutations. The previous dirty path flow widened every file write to the
	// parent directory, which made ordinary saves rescan and diff all siblings.
	for _, delta := range job.DirectDeltas {
		if err := d.applyDirectPathDeltaTx(ctx, tx, *lockedRoot, policy, artifactSync, factMutator, delta); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (d *FileSearchDB) applyDirectPathDeltaTx(ctx context.Context, tx *sql.Tx, root RootRecord, policy *policyState, artifactSync *entrySearchArtifactSyncTx, factMutator *entryFactMutatorTx, delta PathDelta) error {
	startedAt := util.GetSystemTimestamp()
	cleanPath := filepath.Clean(delta.Path)
	if cleanPath == "" || cleanPath == "." {
		return nil
	}
	if !pathWithinScope(root.Path, cleanPath) {
		return fmt.Errorf("direct-delta path %q is outside root path %q", cleanPath, root.Path)
	}

	if isDeleteOnlyDelta(delta.SemanticKind) {
		deleted, err := deleteDirectDeltaEntryByPathTx(ctx, tx, artifactSync, factMutator, cleanPath)
		if err != nil {
			return err
		}
		logFilesearchSQLiteMaintenance(ctx, "direct_delta_delete", cleanPath, util.GetSystemTimestamp()-startedAt, boolToInt(deleted))
		return nil
	}

	// Bug fix: direct deltas should update the symlink entry itself instead of
	// following a symlink-to-directory and deleting it as a skipped directory.
	// Full scans use DirEntry.Info, which reports symlink metadata, so Lstat keeps
	// incremental updates on the same indexing contract.
	info, err := os.Lstat(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			deleted, deleteErr := deleteDirectDeltaEntryByPathTx(ctx, tx, artifactSync, factMutator, cleanPath)
			if deleteErr != nil {
				return deleteErr
			}
			logFilesearchSQLiteMaintenance(ctx, "direct_delta_missing_delete", cleanPath, util.GetSystemTimestamp()-startedAt, boolToInt(deleted))
			return nil
		}
		return err
	}
	if info.IsDir() {
		deleted, err := deleteDirectDeltaEntryByPathTx(ctx, tx, artifactSync, factMutator, cleanPath)
		if err != nil {
			return err
		}
		logFilesearchSQLiteMaintenance(ctx, "direct_delta_directory_skip", cleanPath, util.GetSystemTimestamp()-startedAt, boolToInt(deleted))
		return nil
	}

	policyContext := policy.newTraversalContext(root, filepath.Dir(cleanPath))
	if !policyContext.ShouldIndexPath(cleanPath, false) {
		deleted, err := deleteDirectDeltaEntryByPathTx(ctx, tx, artifactSync, factMutator, cleanPath)
		if err != nil {
			return err
		}
		logFilesearchSQLiteMaintenance(ctx, "direct_delta_policy_delete", cleanPath, util.GetSystemTimestamp()-startedAt, boolToInt(deleted))
		return nil
	}

	entry := newEntryRecord(root, cleanPath, info)
	changed, err := upsertDirectDeltaEntryTx(ctx, tx, artifactSync, factMutator, entry)
	if err != nil {
		return err
	}
	operation := "direct_delta_upsert"
	if !changed {
		operation = "direct_delta_unchanged"
	}
	logFilesearchSQLiteMaintenance(ctx, operation, cleanPath, util.GetSystemTimestamp()-startedAt, boolToInt(changed))
	return nil
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
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := lockRootForSubtreeSnapshot(ctx, tx, root.ID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM directories
		WHERE root_id = ?
		  AND "exists" = 0
	`, root.ID); err != nil {
		return err
	}
	if err := d.finalizeRootRunTx(ctx, tx, root); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	d.checkpointWALAfterFinalize(ctx)
	return nil
}

func (d *FileSearchDB) ReplaceSubtreeSnapshot(ctx context.Context, batch SubtreeSnapshotBatch) error {
	return d.ReplaceSubtreeSnapshots(ctx, []SubtreeSnapshotBatch{batch})
}

func (d *FileSearchDB) ReplaceSubtreeSnapshots(ctx context.Context, batches []SubtreeSnapshotBatch) error {
	if len(batches) == 0 {
		return nil
	}

	startedAt := util.GetSystemTimestamp()
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	directoryStmt, err := prepareDirectoryUpsertStmtTx(ctx, tx)
	if err != nil {
		return err
	}
	defer directoryStmt.Close()
	for _, batch := range batches {
		lockStartedAt := util.GetSystemTimestamp()
		root, err := lockRootForSubtreeSnapshot(ctx, tx, batch.RootID)
		if err != nil {
			return err
		}
		// Small subtree jobs were still taking ~0.8s even when the changed-set replay
		// itself was cheap. Logging the root lock separately shows whether the fixed
		// cost starts with transaction-level contention before any diff work runs.
		logFilesearchSQLiteMaintenance(ctx, "subtree_lock_root", batch.ScopePath, util.GetSystemTimestamp()-lockStartedAt, 1)
		if !pathWithinScope(root.Path, batch.ScopePath) {
			return fmt.Errorf("subtree snapshot scope path %q is outside root path %q", batch.ScopePath, root.Path)
		}

		if err := validateSubtreeSnapshotBatch(batch); err != nil {
			return err
		}

		tombstoneStartedAt := util.GetSystemTimestamp()
		if d.bulkSyncFullRunRootFresh(batch.RootID) {
			// Prepared full-run roots only use this shortcut when the root had no
			// persisted directories before execution started. In that case there is
			// nothing stale to tombstone inside any later leaf scope, so skipping the
			// scoped UPDATE removes one fixed SQL write per subtree without changing
			// which directories survive the run.
			logFilesearchSQLiteMaintenance(ctx, "subtree_tombstone_directories", batch.ScopePath, 0, 0)
		} else {
			if err := tombstoneScopedDirectories(tx, ctx, batch.RootID, batch.ScopePath, subtreeBatchScanTime(batch)); err != nil {
				return err
			}
			// The previous logs only covered entry-table maintenance, so directory tombstones
			// could hide inside the "apply_snapshot" wall time without attribution.
			logFilesearchSQLiteMaintenance(ctx, "subtree_tombstone_directories", batch.ScopePath, util.GetSystemTimestamp()-tombstoneStartedAt, len(batch.Directories))
		}

		upsertDirectoriesStartedAt := util.GetSystemTimestamp()
		for _, directory := range batch.Directories {
			if _, err := directoryStmt.ExecContext(
				ctx,
				directory.Path,
				directory.RootID,
				directory.ParentPath,
				directory.LastScanTime,
				boolToInt(directory.Exists),
			); err != nil {
				return err
			}
		}
		// Tiny WinSxS subtree scopes often carry very few entries, so even directory
		// upserts need their own timing to tell whether the fixed overhead is in
		// directory bookkeeping or later entry diff/commit work.
		logFilesearchSQLiteMaintenance(ctx, "subtree_upsert_directories", batch.ScopePath, util.GetSystemTimestamp()-upsertDirectoriesStartedAt, len(batch.Directories))

		// Subtree refreshes can update, delete, and rename within the scope. The
		// explicit delete-old -> upsert -> insert-new order keeps FTS and bigram
		// rows aligned with the fact table so incremental reconcile does not leave
		// stale matches behind.
		replaceEntriesStartedAt := util.GetSystemTimestamp()
		if err := d.replaceSubtreeEntriesTx(ctx, tx, batch); err != nil {
			return err
		}
		// The changed-set replay now has inner logs, but the subtree wrapper still owns
		// all entry-side work for a batch. Keeping a parent phase here makes it obvious
		// whether the missing time is inside diff collection or outside the entry path.
		logFilesearchSQLiteMaintenance(ctx, "subtree_replace_entries", batch.ScopePath, util.GetSystemTimestamp()-replaceEntriesStartedAt, len(batch.Entries))
	}

	commitStartedAt := util.GetSystemTimestamp()
	if err := tx.Commit(); err != nil {
		return err
	}
	// Recent traces showed many tiny subtree batches paying a near-constant ~0.8s.
	// Logging commit separately distinguishes SQLite flush/lock cost from the batch
	// body so we can tell whether transaction finalization dominates the slowdown.
	logFilesearchSQLiteMaintenance(ctx, "subtree_commit", fmt.Sprintf("batches=%d", len(batches)), util.GetSystemTimestamp()-commitStartedAt, len(batches))
	logFilesearchSQLiteMaintenance(ctx, "subtree_apply_total", fmt.Sprintf("batches=%d", len(batches)), util.GetSystemTimestamp()-startedAt, len(batches))
	return nil
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
	rows, err := d.db.QueryContext(ctx, `
		SELECT entry_id, path, root_id, parent_path, name, normalized_name, name_key, normalized_path,
		       pinyin_full, pinyin_initials, extension, is_dir, mtime, size, updated_at
		FROM entries
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []EntryRecord
	for rows.Next() {
		row, err := scanStoredEntryRecord(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, row.toEntryRecord())
	}

	return entries, rows.Err()
}

func (d *FileSearchDB) ListEntriesByRoot(ctx context.Context, rootID string) ([]EntryRecord, error) {
	rows, err := d.db.QueryContext(ctx, `
		SELECT entry_id, path, root_id, parent_path, name, normalized_name, name_key, normalized_path,
		       pinyin_full, pinyin_initials, extension, is_dir, mtime, size, updated_at
		FROM entries
		WHERE root_id = ?
	`, rootID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []EntryRecord
	for rows.Next() {
		row, err := scanStoredEntryRecord(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, row.toEntryRecord())
	}

	return entries, rows.Err()
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
