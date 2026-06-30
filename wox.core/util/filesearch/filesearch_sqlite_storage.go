package filesearch

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
	"wox/util"
)

const fileSearchSchemaVersion = 2

const (
	searchBigramFieldName       = "name"
	searchBigramFieldPinyinFull = "pinyin_full"
)

var filesearchFTSTables = []string{
	"entries_name_fts",
	"entries_path_fts",
	"entries_pinyin_full_fts",
	"entries_initials_fts",
}

const (
	sqlitePreferredBatchRows = 2000
	entryFactColumnCount     = 14
	directoryColumnCount     = 5

	bulkFinalizeSQLiteCacheSize = -131072
	bulkFinalizeSQLiteMmapSize  = 268435456
)

type storedEntryRecord struct {
	EntryID        int64
	Path           string
	RootID         string
	ParentPath     string
	Name           string
	NormalizedName string
	NameKey        string
	NormalizedPath string
	PinyinFull     string
	PinyinInitials string
	Extension      string
	IsDir          bool
	Mtime          int64
	Size           int64
	UpdatedAt      int64
}

type sqliteIndexSnapshot struct {
	RootCount  int
	EntryCount int64
	// Bug fix: the full-index completion summary needs a real file-only count.
	// EntryCount includes directories, while streaming full-scan estimates no
	// longer count files before execution.
	FileCount              int64
	BigramRowCount         int64
	FactBytesEstimate      int64
	FTSSourceBytesEstimate int64
	BigramBytesEstimate    int64
	TotalBytesEstimate     int64
	DBMainFileBytes        int64
	DBWALFileBytes         int64
	DBSHMFileBytes         int64
	DBTotalFileBytes       int64
	NameFTSVocab           int64
	PathFTSVocab           int64
	PinyinFullFTSVocab     int64
	InitialsFTSVocab       int64
	TopRoots               []sqliteRootSnapshot
}

type sqliteRootSnapshot struct {
	RootID                 string
	Path                   string
	Docs                   int64
	BigramRows             int64
	FactBytesEstimate      int64
	FTSSourceBytesEstimate int64
	BigramBytesEstimate    int64
	TotalBytesEstimate     int64
}

type sqliteIndexDefinition struct {
	Name string
	SQL  string
}

var foregroundEntriesIndexDefinitions = []sqliteIndexDefinition{
	{Name: "idx_entries_name_key", SQL: `CREATE INDEX IF NOT EXISTS idx_entries_name_key ON entries(name_key)`},
	{Name: "idx_entries_extension", SQL: `CREATE INDEX IF NOT EXISTS idx_entries_extension ON entries(extension)`},
}

var maintenanceEntriesIndexDefinitions = []sqliteIndexDefinition{
	// collect_diff_stale/changed_old always constrain by root_id and a scope path
	// prefix together. The previous root_id-only index forced tiny subtree diffs
	// under large roots such as C:\Windows to rescan far too much of the root on
	// every job, so this composite index gives SQLite one access path that matches
	// both predicates without changing query behavior.
	{Name: "idx_entries_root_id_path", SQL: `CREATE INDEX IF NOT EXISTS idx_entries_root_id_path ON entries(root_id, path)`},
	{Name: "idx_entries_parent_path", SQL: `CREATE INDEX IF NOT EXISTS idx_entries_parent_path ON entries(parent_path)`},
	{Name: "idx_entries_is_dir", SQL: `CREATE INDEX IF NOT EXISTS idx_entries_is_dir ON entries(is_dir)`},
}

var entriesIndexDefinitions = []sqliteIndexDefinition{
	maintenanceEntriesIndexDefinitions[0],
	maintenanceEntriesIndexDefinitions[1],
	foregroundEntriesIndexDefinitions[0],
	foregroundEntriesIndexDefinitions[1],
	maintenanceEntriesIndexDefinitions[2],
}

var retiredEntriesIndexNames = []string{
	"idx_entries_root_id",
}

const (
	entryForegroundIndexesStateKey    = "entry_foreground_indexes_state"
	entryMaintenanceIndexesStateKey   = "entry_maintenance_indexes_state"
	entryMaintenanceIndexesGeneration = "entry_maintenance_indexes_generation"

	entryIndexStatePending  = "pending"
	entryIndexStateBuilding = "building"
	entryIndexStateReady    = "ready"
	entryIndexStateError    = "error"
)

type sqliteTxBeginner interface {
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
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
		dynamic_parent_root_id TEXT NOT NULL DEFAULT '',
		policy_root_path TEXT NOT NULL DEFAULT '',
		promoted_at INTEGER NOT NULL DEFAULT 0,
		last_hot_at INTEGER NOT NULL DEFAULT 0,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL
	);

	CREATE TABLE IF NOT EXISTS directories (
		path TEXT PRIMARY KEY,
		root_id TEXT NOT NULL,
		parent_path TEXT NOT NULL,
		last_scan_time INTEGER NOT NULL,
		"exists" INTEGER NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_directories_root_id ON directories(root_id);
	CREATE INDEX IF NOT EXISTS idx_directories_parent_path ON directories(parent_path);
	`

	if _, err := d.db.ExecContext(ctx, createSQL); err != nil {
		return err
	}

	alterTableSQLs := []string{
		`ALTER TABLE roots ADD COLUMN feed_type TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE roots ADD COLUMN feed_cursor TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE roots ADD COLUMN feed_state TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE roots ADD COLUMN last_reconcile_at INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE roots ADD COLUMN last_full_scan_at INTEGER NOT NULL DEFAULT 0`,
		// Dynamic roots are a hidden ownership split, not a new user-visible
		// root type. Incremental ALTERs preserve existing indexes while storing
		// enough metadata to reapply the split and inherit parent policy on restart.
		`ALTER TABLE roots ADD COLUMN dynamic_parent_root_id TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE roots ADD COLUMN policy_root_path TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE roots ADD COLUMN promoted_at INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE roots ADD COLUMN last_hot_at INTEGER NOT NULL DEFAULT 0`,
	}

	for _, alterSQL := range alterTableSQLs {
		_, alterErr := d.db.ExecContext(ctx, alterSQL)
		if alterErr != nil && !strings.Contains(alterErr.Error(), "duplicate column name") {
			return alterErr
		}
	}

	return nil
}

func (d *FileSearchDB) ensureSQLiteSearchSchema(ctx context.Context) error {
	if err := d.probeFTS5(ctx); err != nil {
		return err
	}

	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	currentVersion, err := schemaUserVersionTx(ctx, tx)
	if err != nil {
		return err
	}

	entriesRebuilt, err := migrateEntriesTableIfNeeded(ctx, tx)
	if err != nil {
		return err
	}
	if err := createEntriesIndexes(ctx, tx); err != nil {
		return err
	}
	searchArtifactsCreated, err := createSearchTables(ctx, tx)
	if err != nil {
		return err
	}

	shouldRebuildArtifacts := currentVersion < fileSearchSchemaVersion || entriesRebuilt || searchArtifactsCreated
	if shouldRebuildArtifacts {
		d.searchArtifactsNeedRebuild = true
	}
	if _, err := tx.ExecContext(ctx, fmt.Sprintf(`PRAGMA user_version = %d`, fileSearchSchemaVersion)); err != nil {
		return err
	}
	util.GetLogger().Info(ctx, fmt.Sprintf(
		"filesearch sqlite schema ready: current_version=%d target_version=%d rebuild_artifacts_queued=%v",
		currentVersion,
		fileSearchSchemaVersion,
		shouldRebuildArtifacts,
	))

	return tx.Commit()
}

// NeedsSearchArtifactRebuild reports whether schema init found stale derived search tables.
func (d *FileSearchDB) NeedsSearchArtifactRebuild() bool {
	return d != nil && d.searchArtifactsNeedRebuild
}

// RebuildSearchArtifacts refreshes derived FTS and bigram tables after DB open.
func (d *FileSearchDB) RebuildSearchArtifacts(ctx context.Context) error {
	if d == nil || d.db == nil {
		return nil
	}

	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := rebuildAllSearchArtifactsTx(ctx, tx); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	d.searchArtifactsNeedRebuild = false
	return nil
}

func (d *FileSearchDB) probeFTS5(ctx context.Context) error {
	if _, err := d.db.ExecContext(ctx, `
		CREATE VIRTUAL TABLE IF NOT EXISTS temp.filesearch_fts5_probe USING fts5(value);
	`); err != nil {
		return fmt.Errorf("filesearch requires sqlite FTS5 support; rebuild with -tags sqlite_fts5: %w", err)
	}
	if _, err := d.db.ExecContext(ctx, `DROP TABLE IF EXISTS temp.filesearch_fts5_probe`); err != nil {
		return fmt.Errorf("drop filesearch FTS5 probe table: %w", err)
	}
	return nil
}

func schemaUserVersionTx(ctx context.Context, tx *sql.Tx) (int, error) {
	row := tx.QueryRowContext(ctx, `PRAGMA user_version`)
	var version int
	if err := row.Scan(&version); err != nil {
		return 0, fmt.Errorf("read filesearch schema version: %w", err)
	}
	return version, nil
}

func migrateEntriesTableIfNeeded(ctx context.Context, tx *sql.Tx) (bool, error) {
	exists, err := tableExists(ctx, tx, "entries")
	if err != nil {
		return false, err
	}
	if !exists {
		return true, createEntriesTable(ctx, tx)
	}

	columns, err := tableColumnNames(ctx, tx, "entries")
	if err != nil {
		return false, err
	}
	if columns["entry_id"] && columns["name_key"] && columns["extension"] {
		return false, nil
	}

	// Rebuild the entries table once because the old schema used path as the
	// primary key. SQLite-first search needs a stable integer entry_id so FTS and
	// side tables can reference entries without rowid churn.
	if _, err := tx.ExecContext(ctx, `ALTER TABLE entries RENAME TO entries_legacy`); err != nil {
		return false, fmt.Errorf("rename legacy entries table: %w", err)
	}
	if err := createEntriesTable(ctx, tx); err != nil {
		return false, err
	}

	rows, err := tx.QueryContext(ctx, `
		SELECT path, root_id, parent_path, name, normalized_name, normalized_path,
		       pinyin_full, pinyin_initials, is_dir, mtime, size, updated_at
		FROM entries_legacy
		ORDER BY path ASC
	`)
	if err != nil {
		return false, fmt.Errorf("load legacy entries: %w", err)
	}
	defer rows.Close()

	insertStmt, err := tx.PrepareContext(ctx, `
		INSERT INTO entries (
			path, root_id, parent_path, name, normalized_name, name_key, normalized_path,
			pinyin_full, pinyin_initials, extension, is_dir, mtime, size, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return false, fmt.Errorf("prepare migrated entry insert: %w", err)
	}
	defer insertStmt.Close()

	for rows.Next() {
		var entry EntryRecord
		var isDir int
		if err := rows.Scan(
			&entry.Path,
			&entry.RootID,
			&entry.ParentPath,
			&entry.Name,
			&entry.NormalizedName,
			&entry.NormalizedPath,
			&entry.PinyinFull,
			&entry.PinyinInitials,
			&isDir,
			&entry.Mtime,
			&entry.Size,
			&entry.UpdatedAt,
		); err != nil {
			return false, fmt.Errorf("scan legacy entry: %w", err)
		}
		entry.IsDir = isDir == 1
		stored := buildStoredEntryRecord(entry)
		if _, err := insertStmt.ExecContext(
			ctx,
			stored.Path,
			stored.RootID,
			stored.ParentPath,
			stored.Name,
			stored.NormalizedName,
			stored.NameKey,
			stored.NormalizedPath,
			nullIfEmpty(stored.PinyinFull),
			nullIfEmpty(stored.PinyinInitials),
			stored.Extension,
			boolToInt(stored.IsDir),
			stored.Mtime,
			stored.Size,
			stored.UpdatedAt,
		); err != nil {
			return false, fmt.Errorf("insert migrated entry %q: %w", stored.Path, err)
		}
	}
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("iterate legacy entries: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `DROP TABLE entries_legacy`); err != nil {
		return false, fmt.Errorf("drop legacy entries table: %w", err)
	}
	return true, nil
}

func createEntriesTable(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
		CREATE TABLE entries (
			entry_id INTEGER PRIMARY KEY,
			path TEXT NOT NULL UNIQUE,
			root_id TEXT NOT NULL,
			parent_path TEXT NOT NULL,
			name TEXT NOT NULL,
			normalized_name TEXT NOT NULL,
			name_key TEXT NOT NULL DEFAULT '',
			normalized_path TEXT NOT NULL,
			pinyin_full TEXT NOT NULL DEFAULT '',
			pinyin_initials TEXT NOT NULL DEFAULT '',
			extension TEXT NOT NULL DEFAULT '',
			is_dir INTEGER NOT NULL,
			mtime INTEGER NOT NULL,
			size INTEGER NOT NULL DEFAULT 0,
			updated_at INTEGER NOT NULL
		)
	`)
	if err != nil {
		return fmt.Errorf("create entries table: %w", err)
	}
	return nil
}

func createEntriesIndexes(ctx context.Context, tx *sql.Tx) error {
	if err := dropRetiredEntriesIndexes(ctx, tx); err != nil {
		return err
	}
	for _, definition := range entriesIndexDefinitions {
		if _, err := tx.ExecContext(ctx, definition.SQL); err != nil {
			return err
		}
	}
	return nil
}

func dropRetiredEntriesIndexes(ctx context.Context, exec rootExecContext) error {
	for _, name := range retiredEntriesIndexNames {
		// Optimization: idx_entries_root_id is now redundant because
		// idx_entries_root_id_path can serve root_id equality through SQLite's
		// leftmost-prefix rule. Dropping the retired index during schema/index
		// maintenance keeps old databases from paying rebuild and write costs for
		// an access path that no longer adds query coverage.
		if _, err := exec.ExecContext(ctx, fmt.Sprintf(`DROP INDEX IF EXISTS %s`, name)); err != nil {
			return fmt.Errorf("drop retired %s: %w", name, err)
		}
	}
	return nil
}

func dropEntriesSecondaryIndexes(ctx context.Context, exec rootExecContext) error {
	if err := dropRetiredEntriesIndexes(ctx, exec); err != nil {
		return err
	}
	for _, definition := range entriesIndexDefinitions {
		if _, err := exec.ExecContext(ctx, fmt.Sprintf(`DROP INDEX IF EXISTS %s`, definition.Name)); err != nil {
			return fmt.Errorf("drop %s: %w", definition.Name, err)
		}
	}
	return nil
}

func (d *FileSearchDB) recreateEntryIndexes(ctx context.Context) error {
	return recreateEntryIndexesWithBeginner(ctx, d.db, entriesIndexDefinitions)
}

func recreateEntryIndexesWithBeginner(ctx context.Context, beginner sqliteTxBeginner, definitions []sqliteIndexDefinition) error {
	tx, err := beginner.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := dropRetiredEntriesIndexes(ctx, tx); err != nil {
		return err
	}
	for _, definition := range definitions {
		startedAt := util.GetSystemTimestamp()
		if _, err := tx.ExecContext(ctx, definition.SQL); err != nil {
			return fmt.Errorf("create %s: %w", definition.Name, err)
		}
		// Diagnostic addition: bulk finalize previously reported only the total
		// index recreation cost. Logging each index keeps the rebuild semantics
		// identical while showing whether one secondary index dominates the pause.
		logFilesearchSQLiteMaintenance(ctx, "recreate_entry_index", definition.Name, util.GetSystemTimestamp()-startedAt, 1)
	}
	return tx.Commit()
}

func createSearchTables(ctx context.Context, tx *sql.Tx) (bool, error) {
	searchArtifactsCreated := false
	tableNames := append([]string{"entries_bigram"}, filesearchFTSTables...)
	for _, tableName := range tableNames {
		exists, err := tableExists(ctx, tx, tableName)
		if err != nil {
			return false, err
		}
		if !exists {
			searchArtifactsCreated = true
		}
	}

	statements := []string{
		`CREATE TABLE IF NOT EXISTS entries_bigram (
			field TEXT NOT NULL,
			gram TEXT NOT NULL,
			entry_id INTEGER NOT NULL,
			PRIMARY KEY(field, gram, entry_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_entries_bigram_entry_id ON entries_bigram(entry_id)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS entries_name_fts USING fts5(
			normalized_name,
			content='entries',
			content_rowid='entry_id',
			tokenize='trigram',
			detail='none'
		)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS entries_path_fts USING fts5(
			normalized_path,
			content='entries',
			content_rowid='entry_id',
			tokenize='trigram',
			detail='none'
		)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS entries_pinyin_full_fts USING fts5(
			pinyin_full,
			content='entries',
			content_rowid='entry_id',
			tokenize='trigram',
			detail='none'
		)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS entries_initials_fts USING fts5(
			pinyin_initials,
			content='entries',
			content_rowid='entry_id',
			tokenize='unicode61',
			prefix='1 2 3 4 5 6 7 8'
		)`,
	}
	for _, statement := range statements {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return false, err
		}
	}
	for _, tableName := range filesearchFTSTables {
		if err := configureFTSTableTx(ctx, tx, tableName); err != nil {
			return false, err
		}
	}
	return searchArtifactsCreated, nil
}

func rebuildAllSearchArtifactsTx(ctx context.Context, tx *sql.Tx) error {
	// The SQLite-first search tables are derived data. Rebuilding them during
	// schema init keeps migrations deterministic and avoids serving stale FTS or
	// bigram rows from earlier partial schemas.
	if _, err := rebuildAllBigramsTx(ctx, tx); err != nil {
		return err
	}

	for _, tableName := range filesearchFTSTables {
		if err := rebuildFTSTableTx(ctx, tx, tableName); err != nil {
			return err
		}
	}

	return nil
}

func configureFTSTableTx(ctx context.Context, tx *sql.Tx, tableName string) error {
	commands := []string{
		fmt.Sprintf(`INSERT INTO %s(%s, rank) VALUES('automerge', 8)`, tableName, tableName),
		fmt.Sprintf(`INSERT INTO %s(%s, rank) VALUES('crisismerge', 16)`, tableName, tableName),
		fmt.Sprintf(`INSERT INTO %s(%s, rank) VALUES('usermerge', 4)`, tableName, tableName),
	}
	for _, command := range commands {
		if _, err := tx.ExecContext(ctx, command); err != nil {
			return fmt.Errorf("configure %s: %w", tableName, err)
		}
	}
	return nil
}

func tableExists(ctx context.Context, tx *sql.Tx, tableName string) (bool, error) {
	row := tx.QueryRowContext(ctx, `
		SELECT count(*)
		FROM sqlite_master
		WHERE type IN ('table', 'view') AND name = ?
	`, tableName)
	var count int
	if err := row.Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func tableColumnNames(ctx context.Context, tx *sql.Tx, tableName string) (map[string]bool, error) {
	rows, err := tx.QueryContext(ctx, fmt.Sprintf(`PRAGMA table_info(%s)`, tableName))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns := map[string]bool{}
	for rows.Next() {
		var cid int
		var name string
		var columnType string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
			return nil, err
		}
		columns[name] = true
	}
	return columns, rows.Err()
}

func buildStoredEntryRecord(entry EntryRecord) storedEntryRecord {
	normalizedName := normalizeIndexText(entry.NormalizedName)
	if normalizedName == "" {
		normalizedName = normalizeIndexText(entry.Name)
	}

	normalizedPath := normalizeEntryPathKey(entry)
	if normalizedPath == "" {
		normalizedPath = normalizeIndexText(normalizePath(entry.Path))
	}

	pinyinFull := normalizeIndexText(entry.PinyinFull)
	pinyinInitials := normalizeIndexText(entry.PinyinInitials)
	if shouldDropRedundantPinyinPayload(normalizedName, pinyinFull, pinyinInitials) {
		pinyinFull = ""
		pinyinInitials = ""
	}

	return storedEntryRecord{
		Path:           filepath.Clean(entry.Path),
		RootID:         entry.RootID,
		ParentPath:     filepath.Clean(entry.ParentPath),
		Name:           entry.Name,
		NormalizedName: normalizedName,
		NameKey:        keepLettersAndDigits(normalizedName),
		NormalizedPath: normalizedPath,
		PinyinFull:     pinyinFull,
		PinyinInitials: pinyinInitials,
		Extension:      normalizeExtension(filepath.Ext(entry.Name)),
		IsDir:          entry.IsDir,
		Mtime:          entry.Mtime,
		Size:           entry.Size,
		UpdatedAt:      entry.UpdatedAt,
	}
}

func scanStoredEntryRecord(scanner interface{ Scan(dest ...any) error }) (storedEntryRecord, error) {
	var row storedEntryRecord
	var isDir int
	if err := scanner.Scan(
		&row.EntryID,
		&row.Path,
		&row.RootID,
		&row.ParentPath,
		&row.Name,
		&row.NormalizedName,
		&row.NameKey,
		&row.NormalizedPath,
		&row.PinyinFull,
		&row.PinyinInitials,
		&row.Extension,
		&isDir,
		&row.Mtime,
		&row.Size,
		&row.UpdatedAt,
	); err != nil {
		return storedEntryRecord{}, err
	}
	row.IsDir = isDir == 1
	return row, nil
}

func (row storedEntryRecord) toEntryRecord() EntryRecord {
	return EntryRecord{
		Path:           row.Path,
		RootID:         row.RootID,
		ParentPath:     row.ParentPath,
		Name:           row.Name,
		NormalizedName: row.NormalizedName,
		NormalizedPath: row.NormalizedPath,
		PinyinFull:     row.PinyinFull,
		PinyinInitials: row.PinyinInitials,
		IsDir:          row.IsDir,
		Mtime:          row.Mtime,
		Size:           row.Size,
		UpdatedAt:      row.UpdatedAt,
	}
}

func nullIfEmpty(value string) any {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return value
}

func insertEntryBigramsTx(ctx context.Context, stmt *sql.Stmt, row storedEntryRecord) error {
	for _, gram := range uniqueNgrams(row.NormalizedName, 2) {
		if _, err := stmt.ExecContext(ctx, searchBigramFieldName, gram, row.EntryID); err != nil {
			return fmt.Errorf("insert name bigram for %q: %w", row.Path, err)
		}
	}
	for _, gram := range uniqueNgrams(row.PinyinFull, 2) {
		if _, err := stmt.ExecContext(ctx, searchBigramFieldPinyinFull, gram, row.EntryID); err != nil {
			return fmt.Errorf("insert pinyin bigram for %q: %w", row.Path, err)
		}
	}
	return nil
}

func nextPrefixUpperBound(prefix string) string {
	if prefix == "" {
		return ""
	}
	runes := []rune(prefix)
	last := len(runes) - 1
	runes[last]++
	return string(runes[:last+1])
}

func uniqueInt64(values []int64) []int64 {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[int64]struct{}, len(values))
	unique := make([]int64, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		unique = append(unique, value)
	}
	sort.Slice(unique, func(left int, right int) bool {
		return unique[left] < unique[right]
	})
	return unique
}

func escapeLikePattern(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return replacer.Replace(value)
}

func longestLiteralFromWildcard(raw string) string {
	wildcard := buildWildcardQuery(raw)
	if wildcard == nil {
		return ""
	}
	return longestString(buildWildcardLiterals(raw))
}

func trimCandidateIDs(candidateIDs []int64, limit int) []int64 {
	candidateIDs = uniqueInt64(candidateIDs)
	if limit > 0 && len(candidateIDs) > limit {
		return append([]int64(nil), candidateIDs[:limit]...)
	}
	return candidateIDs
}

func utf8LenString(value string) int {
	return utf8.RuneCountInString(value)
}

func (d *FileSearchDB) SearchIndexCounts(ctx context.Context) (int64, int64, error) {
	if d == nil || d.db == nil {
		return 0, 0, nil
	}

	// Optimization: completion summaries only need entry/file counts. The full
	// diagnostic snapshot also counts FTS vocab tables and file sizes, which made
	// the user-visible full-index path wait on logging-only work.
	var entryCount int64
	var fileCount int64
	if err := d.db.QueryRowContext(ctx, `
		SELECT
			count(*),
			COALESCE(SUM(CASE WHEN is_dir = 0 THEN 1 ELSE 0 END), 0)
		FROM entries
	`).Scan(&entryCount, &fileCount); err != nil {
		return 0, 0, err
	}
	return fileCount, entryCount, nil
}

func (d *FileSearchDB) SearchIndexSnapshot(ctx context.Context) (sqliteIndexSnapshot, error) {
	if d == nil || d.db == nil {
		return sqliteIndexSnapshot{}, nil
	}

	snapshot := sqliteIndexSnapshot{}
	if err := d.db.QueryRowContext(ctx, `SELECT count(*) FROM roots`).Scan(&snapshot.RootCount); err != nil {
		return sqliteIndexSnapshot{}, err
	}
	if err := d.db.QueryRowContext(ctx, `SELECT count(*) FROM entries`).Scan(&snapshot.EntryCount); err != nil {
		return sqliteIndexSnapshot{}, err
	}
	if err := d.db.QueryRowContext(ctx, `SELECT count(*) FROM entries WHERE is_dir = 0`).Scan(&snapshot.FileCount); err != nil {
		return sqliteIndexSnapshot{}, err
	}
	if err := d.db.QueryRowContext(ctx, `SELECT count(*) FROM entries_bigram`).Scan(&snapshot.BigramRowCount); err != nil {
		return sqliteIndexSnapshot{}, err
	}

	// The previous log used PRAGMA page_count * page_size as "db_file_bytes".
	// That only described the main database allocation and hid the WAL/shm files,
	// so the reported size did not match what users saw on disk.
	snapshot.DBMainFileBytes = fileSizeOrZero(d.dbPath)
	snapshot.DBWALFileBytes = fileSizeOrZero(d.dbPath + "-wal")
	snapshot.DBSHMFileBytes = fileSizeOrZero(d.dbPath + "-shm")
	snapshot.DBTotalFileBytes = snapshot.DBMainFileBytes + snapshot.DBWALFileBytes + snapshot.DBSHMFileBytes

	// SQLite does not expose per-index byte ownership cheaply here, so the
	// snapshot reports a stable estimate split by fact rows, FTS source text, and
	// bigram rows. This keeps the logs comparable without pretending the estimate
	// is a precise on-disk index size.
	if err := d.db.QueryRowContext(ctx, `
		SELECT
			COALESCE(SUM(
				length(CAST(path AS BLOB)) +
				length(CAST(root_id AS BLOB)) +
				length(CAST(parent_path AS BLOB)) +
				length(CAST(name AS BLOB)) +
				length(CAST(normalized_name AS BLOB)) +
				length(CAST(name_key AS BLOB)) +
				length(CAST(normalized_path AS BLOB)) +
				length(CAST(pinyin_full AS BLOB)) +
				length(CAST(pinyin_initials AS BLOB)) +
				length(CAST(extension AS BLOB)) +
				25
			), 0),
			COALESCE(SUM(
				length(CAST(normalized_name AS BLOB)) +
				length(CAST(normalized_path AS BLOB)) +
				length(CAST(pinyin_full AS BLOB)) +
				length(CAST(pinyin_initials AS BLOB))
			), 0)
		FROM entries
	`).Scan(&snapshot.FactBytesEstimate, &snapshot.FTSSourceBytesEstimate); err != nil {
		return sqliteIndexSnapshot{}, err
	}
	if err := d.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(
			length(CAST(field AS BLOB)) +
			length(CAST(gram AS BLOB)) +
			8
		), 0)
		FROM entries_bigram
	`).Scan(&snapshot.BigramBytesEstimate); err != nil {
		return sqliteIndexSnapshot{}, err
	}
	snapshot.TotalBytesEstimate = snapshot.FactBytesEstimate + snapshot.FTSSourceBytesEstimate + snapshot.BigramBytesEstimate

	nameVocab, err := countFTSVocabRows(ctx, d.db, "entries_name_fts")
	if err != nil {
		return sqliteIndexSnapshot{}, err
	}
	snapshot.NameFTSVocab = nameVocab
	pathVocab, err := countFTSVocabRows(ctx, d.db, "entries_path_fts")
	if err != nil {
		return sqliteIndexSnapshot{}, err
	}
	snapshot.PathFTSVocab = pathVocab
	pinyinFullVocab, err := countFTSVocabRows(ctx, d.db, "entries_pinyin_full_fts")
	if err != nil {
		return sqliteIndexSnapshot{}, err
	}
	snapshot.PinyinFullFTSVocab = pinyinFullVocab
	initialsVocab, err := countFTSVocabRows(ctx, d.db, "entries_initials_fts")
	if err != nil {
		return sqliteIndexSnapshot{}, err
	}
	snapshot.InitialsFTSVocab = initialsVocab

	rows, err := d.db.QueryContext(ctx, `
		SELECT
			roots.id,
			roots.path,
			COALESCE(entry_stats.docs, 0) AS docs,
			COALESCE(bigram_stats.bigram_rows, 0) AS bigram_rows,
			COALESCE(entry_stats.fact_bytes_est, 0) AS fact_bytes_est,
			COALESCE(entry_stats.fts_source_bytes_est, 0) AS fts_source_bytes_est,
			COALESCE(bigram_stats.bigram_bytes_est, 0) AS bigram_bytes_est,
			COALESCE(entry_stats.fact_bytes_est, 0) +
			COALESCE(entry_stats.fts_source_bytes_est, 0) +
			COALESCE(bigram_stats.bigram_bytes_est, 0) AS total_bytes_est
		FROM roots
		LEFT JOIN (
			SELECT
				root_id,
				COUNT(*) AS docs,
				COALESCE(SUM(
					length(CAST(path AS BLOB)) +
					length(CAST(root_id AS BLOB)) +
					length(CAST(parent_path AS BLOB)) +
					length(CAST(name AS BLOB)) +
					length(CAST(normalized_name AS BLOB)) +
					length(CAST(name_key AS BLOB)) +
					length(CAST(normalized_path AS BLOB)) +
					length(CAST(pinyin_full AS BLOB)) +
					length(CAST(pinyin_initials AS BLOB)) +
					length(CAST(extension AS BLOB)) +
					25
				), 0) AS fact_bytes_est,
				COALESCE(SUM(
					length(CAST(normalized_name AS BLOB)) +
					length(CAST(normalized_path AS BLOB)) +
					length(CAST(pinyin_full AS BLOB)) +
					length(CAST(pinyin_initials AS BLOB))
				), 0) AS fts_source_bytes_est
			FROM entries
			GROUP BY root_id
		) AS entry_stats ON entry_stats.root_id = roots.id
		LEFT JOIN (
			SELECT
				entries.root_id AS root_id,
				COUNT(*) AS bigram_rows,
				COALESCE(SUM(
					length(CAST(entries_bigram.field AS BLOB)) +
					length(CAST(entries_bigram.gram AS BLOB)) +
					8
				), 0) AS bigram_bytes_est
			FROM entries_bigram
			INNER JOIN entries ON entries.entry_id = entries_bigram.entry_id
			GROUP BY entries.root_id
		) AS bigram_stats ON bigram_stats.root_id = roots.id
		ORDER BY total_bytes_est DESC, docs DESC, roots.path ASC
		LIMIT 5
	`)
	if err != nil {
		return sqliteIndexSnapshot{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var root sqliteRootSnapshot
		if err := rows.Scan(
			&root.RootID,
			&root.Path,
			&root.Docs,
			&root.BigramRows,
			&root.FactBytesEstimate,
			&root.FTSSourceBytesEstimate,
			&root.BigramBytesEstimate,
			&root.TotalBytesEstimate,
		); err != nil {
			return sqliteIndexSnapshot{}, err
		}
		snapshot.TopRoots = append(snapshot.TopRoots, root)
	}
	if err := rows.Err(); err != nil {
		return sqliteIndexSnapshot{}, err
	}

	return snapshot, nil
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

func countFTSVocabRows(ctx context.Context, db *sql.DB, tableName string) (int64, error) {
	vocabTable := fmt.Sprintf("filesearch_%s_vocab", tableName)
	if _, err := db.ExecContext(ctx, fmt.Sprintf(`DROP TABLE IF EXISTS %s`, vocabTable)); err != nil {
		return 0, err
	}
	// fts5vocab resolves its source table in the schema that owns the virtual
	// table itself. Create and drop the helper in main so snapshot sampling does
	// not accidentally resolve temp.entries_* and fail before logging anything.
	if _, err := db.ExecContext(ctx, fmt.Sprintf(`CREATE VIRTUAL TABLE %s USING fts5vocab(%s, 'row')`, vocabTable, tableName)); err != nil {
		return 0, err
	}
	defer db.ExecContext(context.Background(), fmt.Sprintf(`DROP TABLE IF EXISTS %s`, vocabTable))

	var count int64
	if err := db.QueryRowContext(ctx, fmt.Sprintf(`SELECT count(*) FROM %s`, vocabTable)).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (d *FileSearchDB) BeginBulkSync() {
	if d == nil {
		return
	}
	d.bulkSyncMu.Lock()
	if d.bulkSyncDepth == 0 {
		// Bulk-sync hints describe one sealed full-index attempt. Reset them when
		// the outermost run starts so a new scan never reuses a previous root's
		// emptiness decision after facts have already been written.
		d.bulkSyncFullRunRoots = map[string]bulkSyncFullRunRootState{}
		d.bulkSyncEntryIndexesChecked = false
		d.bulkSyncEntryIndexesDropped = false
	}
	d.bulkSyncDepth++
	d.bulkSyncMu.Unlock()
}

func (d *FileSearchDB) EndBulkSync(ctx context.Context) error {
	if d == nil {
		return nil
	}

	d.bulkSyncMu.Lock()
	if d.bulkSyncDepth > 0 {
		d.bulkSyncDepth--
	}
	shouldFinalize := d.bulkSyncDepth == 0
	entryIndexesDropped := d.bulkSyncEntryIndexesDropped
	if shouldFinalize {
		// The full-run root hints are only valid while one bulk-sync attempt is
		// active. Clear them before the deferred rebuild so later ad-hoc writes
		// fall back to the conservative per-scope diff path.
		d.bulkSyncFullRunRoots = nil
		d.bulkSyncEntryIndexesChecked = false
		d.bulkSyncEntryIndexesDropped = false
	}
	d.bulkSyncMu.Unlock()

	if !shouldFinalize {
		return nil
	}

	return d.withBulkFinalizeConnection(ctx, func(conn *sql.Conn) error {
		if entryIndexesDropped {
			startedAt := util.GetSystemTimestamp()
			logFilesearchIndexPhase(ctx, "bulk_finalize_recreate_entry_indexes_start", "bulk", 0, map[string]any{
				"group":   "foreground",
				"indexes": len(foregroundEntriesIndexDefinitions),
			})
			if err := d.setEntryIndexState(ctx, entryForegroundIndexesStateKey, entryIndexStateBuilding); err != nil {
				return err
			}
			if err := recreateEntryIndexesWithBeginner(ctx, conn, foregroundEntriesIndexDefinitions); err != nil {
				_ = d.setEntryIndexState(ctx, entryForegroundIndexesStateKey, entryIndexStateError)
				return err
			}
			if err := d.setEntryIndexState(ctx, entryForegroundIndexesStateKey, entryIndexStateReady); err != nil {
				return err
			}
			if err := d.setEntryIndexState(ctx, entryMaintenanceIndexesStateKey, entryIndexStatePending); err != nil {
				return err
			}
			if err := d.setEntryIndexGeneration(ctx); err != nil {
				return err
			}
			elapsedMs := util.GetSystemTimestamp() - startedAt
			logFilesearchSQLiteMaintenance(ctx, "recreate_entry_indexes", "foreground", elapsedMs, len(foregroundEntriesIndexDefinitions))
			logFilesearchIndexPhase(ctx, "bulk_finalize_recreate_entry_indexes_done", "bulk", elapsedMs, map[string]any{
				"group":   "foreground",
				"indexes": len(foregroundEntriesIndexDefinitions),
			})
		}

		// Bulk mode now defers both bigram and FTS maintenance until the end of the
		// scan cycle. Root-local bigram refreshes made full runs stall in every
		// finalize job, while the final index only needs one consistent rebuild after
		// the fact table has settled.
		// Optimization: a foreground full index only needs rebuilt FTS tables to make
		// results searchable. FTS optimize only merges segments/compacts storage, so
		// keeping it out of the user-visible indexing path preserves search semantics
		// while avoiding a finalize pause on every manual rebuild.
		if err := d.rebuildBulkSearchArtifactsWithBeginner(ctx, conn, false, entryIndexesDropped); err != nil {
			return err
		}
		return nil
	})
}

func (d *FileSearchDB) withBulkFinalizeConnection(ctx context.Context, run func(conn *sql.Conn) error) error {
	conn, err := d.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	if err := configureBulkFinalizeConnection(ctx, conn); err != nil {
		return err
	}
	return run(conn)
}

func configureBulkFinalizeConnection(ctx context.Context, conn *sql.Conn) error {
	pragmas := []struct {
		name string
		sql  string
	}{
		{name: "cache_size", sql: fmt.Sprintf(`PRAGMA cache_size = %d`, bulkFinalizeSQLiteCacheSize)},
		{name: "temp_store", sql: `PRAGMA temp_store = MEMORY`},
		{name: "mmap_size", sql: fmt.Sprintf(`PRAGMA mmap_size = %d`, bulkFinalizeSQLiteMmapSize)},
	}
	for _, pragma := range pragmas {
		startedAt := util.GetSystemTimestamp()
		// Optimization: bulk finalize performs large SQLite scans and temporary
		// sort/build work in one foreground checkpoint. Applying conservative
		// connection-local PRAGMAs here improves that checkpoint without changing
		// table contents, index definitions, or search visibility semantics.
		if _, err := conn.ExecContext(ctx, pragma.sql); err != nil {
			return fmt.Errorf("set bulk finalize pragma %s: %w", pragma.name, err)
		}
		logFilesearchSQLiteMaintenance(ctx, "bulk_finalize_pragma", pragma.name, util.GetSystemTimestamp()-startedAt, 1)
	}
	return nil
}

func (d *FileSearchDB) prepareBulkSyncFullRunRoot(ctx context.Context, rootID string) error {
	if d == nil {
		return nil
	}
	if strings.TrimSpace(rootID) == "" {
		return fmt.Errorf("prepare bulk-sync full-run root requires root id")
	}

	d.bulkSyncMu.Lock()
	if d.bulkSyncDepth == 0 {
		d.bulkSyncMu.Unlock()
		return nil
	}
	if state, ok := d.bulkSyncFullRunRoots[rootID]; ok && state.prepared {
		d.bulkSyncMu.Unlock()
		return nil
	}
	d.bulkSyncMu.Unlock()

	// Scanner full runs apply each sealed leaf scope exactly once. Capturing the
	// root's initial emptiness before any write starts lets later scope applies
	// reuse one root-level answer instead of paying the same scope existence
	// probe thousands of times during a fresh full rebuild.
	freshAtStart, err := d.isRootFreshAtBulkSyncStart(ctx, rootID)
	if err != nil {
		return err
	}
	if freshAtStart {
		if err := d.maybeDropEntryIndexesForFreshBulkSync(ctx); err != nil {
			return err
		}
	}

	d.bulkSyncMu.Lock()
	defer d.bulkSyncMu.Unlock()
	if d.bulkSyncDepth == 0 {
		return nil
	}
	if d.bulkSyncFullRunRoots == nil {
		d.bulkSyncFullRunRoots = map[string]bulkSyncFullRunRootState{}
	}
	d.bulkSyncFullRunRoots[rootID] = bulkSyncFullRunRootState{
		prepared:     true,
		freshAtStart: freshAtStart,
	}
	return nil
}

func (d *FileSearchDB) isBulkSyncEnabled() bool {
	if d == nil {
		return false
	}
	d.bulkSyncMu.Lock()
	defer d.bulkSyncMu.Unlock()
	return d.bulkSyncDepth > 0
}

func (d *FileSearchDB) bulkSyncFullRunRootFresh(rootID string) bool {
	if d == nil || strings.TrimSpace(rootID) == "" {
		return false
	}
	d.bulkSyncMu.Lock()
	defer d.bulkSyncMu.Unlock()
	state, ok := d.bulkSyncFullRunRoots[rootID]
	return ok && state.prepared && state.freshAtStart
}

func (d *FileSearchDB) maybeDropEntryIndexesForFreshBulkSync(ctx context.Context) error {
	if d == nil {
		return nil
	}

	d.bulkSyncMu.Lock()
	if d.bulkSyncDepth == 0 || d.bulkSyncEntryIndexesChecked {
		d.bulkSyncMu.Unlock()
		return nil
	}
	d.bulkSyncEntryIndexesChecked = true
	d.bulkSyncMu.Unlock()

	isEmpty, err := d.entriesTableEmpty(ctx)
	if err != nil {
		return err
	}
	if !isEmpty {
		return nil
	}

	if err := d.setEntryIndexState(ctx, entryForegroundIndexesStateKey, entryIndexStatePending); err != nil {
		return err
	}
	if err := d.setEntryIndexState(ctx, entryMaintenanceIndexesStateKey, entryIndexStatePending); err != nil {
		return err
	}

	startedAt := util.GetSystemTimestamp()
	if err := dropEntriesSecondaryIndexes(ctx, d.db); err != nil {
		return err
	}
	logFilesearchSQLiteMaintenance(ctx, "drop_entry_indexes", "bulk", util.GetSystemTimestamp()-startedAt, len(entriesIndexDefinitions))

	d.bulkSyncMu.Lock()
	if d.bulkSyncDepth > 0 {
		// Optimization: when a manual rebuild starts with an empty entries table,
		// maintaining secondary lookup indexes per inserted row is pure overhead.
		// Drop only non-unique indexes; the path UNIQUE constraint remains active,
		// preserving overlap/conflict behavior while EndBulkSync recreates indexes
		// before search becomes visible again.
		d.bulkSyncEntryIndexesDropped = true
	}
	d.bulkSyncMu.Unlock()
	return nil
}

func (d *FileSearchDB) entriesTableEmpty(ctx context.Context) (bool, error) {
	if d == nil || d.db == nil {
		return true, nil
	}
	row := d.db.QueryRowContext(ctx, `SELECT 1 FROM entries LIMIT 1`)
	var value int
	if err := row.Scan(&value); err != nil {
		if err == sql.ErrNoRows {
			return true, nil
		}
		return false, err
	}
	return false, nil
}

func (d *FileSearchDB) setEntryIndexState(ctx context.Context, key string, state string) error {
	if d == nil || d.db == nil {
		return nil
	}
	_, err := d.db.ExecContext(ctx, `
		INSERT INTO meta(key, value)
		VALUES(?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, key, state)
	if err != nil {
		return fmt.Errorf("set filesearch index state %s=%s: %w", key, state, err)
	}
	return nil
}

func (d *FileSearchDB) setEntryIndexGeneration(ctx context.Context) error {
	if d == nil || d.db == nil {
		return nil
	}
	_, err := d.db.ExecContext(ctx, `
		INSERT INTO meta(key, value)
		VALUES(?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, entryMaintenanceIndexesGeneration, fmt.Sprintf("%d", util.GetSystemTimestamp()))
	if err != nil {
		return fmt.Errorf("set filesearch maintenance index generation: %w", err)
	}
	return nil
}

func (d *FileSearchDB) entryIndexState(ctx context.Context, key string) (string, error) {
	if d == nil || d.db == nil {
		return entryIndexStateReady, nil
	}
	var state string
	err := d.db.QueryRowContext(ctx, `SELECT value FROM meta WHERE key = ?`, key).Scan(&state)
	if err == sql.ErrNoRows {
		return entryIndexStateReady, nil
	}
	if err != nil {
		return "", fmt.Errorf("load filesearch index state %s: %w", key, err)
	}
	state = strings.TrimSpace(state)
	if state == "" {
		return entryIndexStateReady, nil
	}
	return state, nil
}

// EntryMaintenanceIndexesReady reports whether incremental reconcile can use
// the maintenance indexes that keep dirty-scope diffs off full table scans.
func (d *FileSearchDB) EntryMaintenanceIndexesReady(ctx context.Context) (bool, error) {
	state, err := d.entryIndexState(ctx, entryMaintenanceIndexesStateKey)
	if err != nil {
		return false, err
	}
	return state == entryIndexStateReady, nil
}

// EnsureForegroundEntryIndexes recovers the minimal query indexes after a crash
// during a fresh full scan's foreground finalize.
func (d *FileSearchDB) EnsureForegroundEntryIndexes(ctx context.Context) error {
	state, err := d.entryIndexState(ctx, entryForegroundIndexesStateKey)
	if err != nil {
		return err
	}
	if state == entryIndexStateReady {
		return nil
	}

	startedAt := util.GetSystemTimestamp()
	if err := d.setEntryIndexState(ctx, entryForegroundIndexesStateKey, entryIndexStateBuilding); err != nil {
		return err
	}
	if err := d.withBulkFinalizeConnection(ctx, func(conn *sql.Conn) error {
		return recreateEntryIndexesWithBeginner(ctx, conn, foregroundEntriesIndexDefinitions)
	}); err != nil {
		_ = d.setEntryIndexState(ctx, entryForegroundIndexesStateKey, entryIndexStateError)
		return err
	}
	if err := d.setEntryIndexState(ctx, entryForegroundIndexesStateKey, entryIndexStateReady); err != nil {
		return err
	}
	logFilesearchIndexPhase(ctx, "foreground_entry_indexes_recovered", "startup", util.GetSystemTimestamp()-startedAt, map[string]any{
		"indexes": len(foregroundEntriesIndexDefinitions),
	})
	return nil
}

// BuildMaintenanceEntryIndexes recreates deferred maintenance indexes in the
// background after search results are already foreground-ready.
func (d *FileSearchDB) BuildMaintenanceEntryIndexes(ctx context.Context) error {
	if d == nil {
		return nil
	}
	d.entryIndexMaintenanceMu.Lock()
	defer d.entryIndexMaintenanceMu.Unlock()

	ready, err := d.EntryMaintenanceIndexesReady(ctx)
	if err != nil {
		return err
	}
	if ready {
		return nil
	}

	startedAt := util.GetSystemTimestamp()
	if err := d.setEntryIndexState(ctx, entryMaintenanceIndexesStateKey, entryIndexStateBuilding); err != nil {
		return err
	}
	if err := d.withBulkFinalizeConnection(ctx, func(conn *sql.Conn) error {
		return recreateEntryIndexesWithBeginner(ctx, conn, maintenanceEntriesIndexDefinitions)
	}); err != nil {
		_ = d.setEntryIndexState(ctx, entryMaintenanceIndexesStateKey, entryIndexStateError)
		return err
	}
	if err := d.setEntryIndexState(ctx, entryMaintenanceIndexesStateKey, entryIndexStateReady); err != nil {
		return err
	}
	logFilesearchIndexPhase(ctx, "maintenance_entry_indexes_done", "background", util.GetSystemTimestamp()-startedAt, map[string]any{
		"indexes": len(maintenanceEntriesIndexDefinitions),
	})
	return nil
}

func (d *FileSearchDB) applyDirectFilesEntriesTx(ctx context.Context, tx *sql.Tx, batch SubtreeSnapshotBatch) error {
	stageStmt, err := prepareEntryStageInsertStmtTx(ctx, tx)
	if err != nil {
		return err
	}
	defer stageStmt.Close()

	stageStartedAt := util.GetSystemTimestamp()
	if err := stageEntryRecordsWithStmtTx(ctx, stageStmt, batch.Entries); err != nil {
		return err
	}
	// Direct-files jobs can spend tens of seconds inside one stream_apply. Record
	// the staging boundary so the next trace can separate temp-table population
	// from the later diff and replay phases inside the same SQLite transaction.
	logFilesearchSQLiteMaintenance(ctx, "direct_files_stage_entries", batch.ScopePath, util.GetSystemTimestamp()-stageStartedAt, len(batch.Entries))

	return d.replaceDirectFilesEntriesFromStageTx(ctx, tx, batch.RootID, batch.ScopePath)
}

func (d *FileSearchDB) replaceDirectFilesEntriesFromStageTx(ctx context.Context, tx *sql.Tx, rootID string, scopePath string) error {
	if d.bulkSyncFullRunRootFresh(rootID) {
		// Scanner full runs prepare only sealed, non-overlapping leaf scopes here.
		// When the whole root started empty, this direct-files scope cannot own any
		// older facts, so replaying the staged rows directly preserves the final
		// result while removing one scope-existence query per directory.
		return insertStagedEntriesAsNewFactsTx(ctx, tx, "direct_files_bulk_fresh_root_insert", scopePath)
	}
	if d.isBulkSyncEnabled() {
		// Full runs rebuild search artifacts once at bulk finalize. When this
		// direct-files scope has no persisted entries yet, diffing against the
		// empty baseline only burns SQL time without changing the resulting facts.
		scopeHasEntries, err := hasPersistedDirectFilesEntriesTx(ctx, tx, rootID, scopePath)
		if err != nil {
			return err
		}
		if !scopeHasEntries {
			return insertStagedEntriesAsNewFactsTx(ctx, tx, "direct_files_bulk_empty_scope_insert", scopePath)
		}
	}

	diffStartedAt := util.GetSystemTimestamp()
	staleRows, changedOldRows, changedOrNewRows, err := collectChangedDirectFilesEntrySetsTx(ctx, tx, rootID, scopePath)
	if err != nil {
		return err
	}
	// The previous stream_apply log only exposed total wall time. This direct-files
	// diff log shows whether the transaction stalls before row replay even begins.
	logFilesearchSQLiteMaintenance(ctx, "direct_files_collect_diff", scopePath, util.GetSystemTimestamp()-diffStartedAt, len(staleRows)+len(changedOldRows)+len(changedOrNewRows))
	// Full runs already rebuild derived search structures after facts settle. The
	// old direct-files path still replayed FTS/bigram rows per file inside bulk
	// sync, which duplicated the later finalize work and made large roots much
	// slower without changing the final index shape.
	return applyChangedEntrySetsTx(ctx, tx, scopePath, staleRows, changedOldRows, changedOrNewRows, !d.isBulkSyncEnabled(), nil)
}

func collectChangedDirectFilesEntrySetsTx(ctx context.Context, tx *sql.Tx, rootID string, scopePath string) ([]storedEntryRecord, []storedEntryRecord, []storedEntryRecord, error) {
	directScopePredicate := "(e.path = ? OR (e.parent_path = ? AND e.is_dir = 0))"
	directStagePredicate := "(s.path = ? OR (s.parent_path = ? AND s.is_dir = 0))"
	diffPredicate := buildEntryDifferencePredicate("e", "s")

	staleRows, err := selectStoredEntriesTx(ctx, tx, fmt.Sprintf(`
		SELECT e.entry_id, e.path, e.root_id, e.parent_path, e.name, e.normalized_name, e.name_key, e.normalized_path,
		       e.pinyin_full, e.pinyin_initials, e.extension, e.is_dir, e.mtime, e.size, e.updated_at
		FROM entries e
		LEFT JOIN filesearch_stage_entries s ON s.path = e.path
		WHERE e.root_id = ? AND %s AND s.path IS NULL
		ORDER BY e.path ASC
	`, directScopePredicate), rootID, scopePath, scopePath)
	if err != nil {
		return nil, nil, nil, err
	}

	changedOldRows, err := selectStoredEntriesTx(ctx, tx, fmt.Sprintf(`
		SELECT e.entry_id, e.path, e.root_id, e.parent_path, e.name, e.normalized_name, e.name_key, e.normalized_path,
		       e.pinyin_full, e.pinyin_initials, e.extension, e.is_dir, e.mtime, e.size, e.updated_at
		FROM entries e
		INNER JOIN filesearch_stage_entries s ON s.path = e.path
		WHERE e.root_id = ? AND %s AND (%s)
		ORDER BY e.path ASC
	`, directScopePredicate, diffPredicate), rootID, scopePath, scopePath)
	if err != nil {
		return nil, nil, nil, err
	}

	changedOrNewRows, err := selectStoredEntriesTx(ctx, tx, fmt.Sprintf(`
		SELECT CAST(COALESCE(e.entry_id, 0) AS INTEGER) AS entry_id,
		       s.path, s.root_id, s.parent_path, s.name, s.normalized_name, s.name_key, s.normalized_path,
		       s.pinyin_full, s.pinyin_initials, s.extension, s.is_dir, s.mtime, s.size, s.updated_at
		FROM filesearch_stage_entries s
		LEFT JOIN entries e ON e.path = s.path
		WHERE %s AND (e.entry_id IS NULL OR (%s))
		ORDER BY s.path ASC
	`, directStagePredicate, diffPredicate), scopePath, scopePath)
	if err != nil {
		return nil, nil, nil, err
	}

	return staleRows, changedOldRows, changedOrNewRows, nil
}

func stageEntryRecordsTx(ctx context.Context, tx *sql.Tx, entries []EntryRecord) error {
	stmt, err := prepareEntryStageInsertStmtTx(ctx, tx)
	if err != nil {
		return err
	}
	defer stmt.Close()

	return stageEntryRecordsWithStmtTx(ctx, stmt, entries)
}

func prepareEntryStageInsertStmtTx(ctx context.Context, tx *sql.Tx) (*sql.Stmt, error) {
	if _, err := tx.ExecContext(ctx, `
			CREATE TEMP TABLE IF NOT EXISTS filesearch_stage_entries (
			path TEXT PRIMARY KEY,
			root_id TEXT NOT NULL,
			parent_path TEXT NOT NULL,
			name TEXT NOT NULL,
			normalized_name TEXT NOT NULL,
			name_key TEXT NOT NULL,
			normalized_path TEXT NOT NULL,
			pinyin_full TEXT NOT NULL,
			pinyin_initials TEXT NOT NULL,
			extension TEXT NOT NULL,
			is_dir INTEGER NOT NULL,
			mtime INTEGER NOT NULL,
			size INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		)
		`); err != nil {
		return nil, fmt.Errorf("create stage entries table: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM filesearch_stage_entries`); err != nil {
		return nil, fmt.Errorf("clear stage entries table: %w", err)
	}

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO filesearch_stage_entries (
			path, root_id, parent_path, name, normalized_name, name_key, normalized_path,
			pinyin_full, pinyin_initials, extension, is_dir, mtime, size, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return nil, fmt.Errorf("prepare stage entry insert: %w", err)
	}
	return stmt, nil
}

func stageEntryRecordsWithStmtTx(ctx context.Context, stmt *sql.Stmt, entries []EntryRecord) error {
	for _, entry := range entries {
		row := buildStoredEntryRecord(entry)
		if _, err := stmt.ExecContext(
			ctx,
			row.Path,
			row.RootID,
			row.ParentPath,
			row.Name,
			row.NormalizedName,
			row.NameKey,
			row.NormalizedPath,
			row.PinyinFull,
			row.PinyinInitials,
			row.Extension,
			boolToInt(row.IsDir),
			row.Mtime,
			row.Size,
			row.UpdatedAt,
		); err != nil {
			return fmt.Errorf("stage entry %q: %w", row.Path, err)
		}
	}
	return nil
}

func (d *FileSearchDB) replaceRootEntriesTx(ctx context.Context, tx *sql.Tx, root RootRecord, entries []EntryRecord, onProgress func(current int64, total int64)) error {
	if err := stageEntryRecordsTx(ctx, tx, entries); err != nil {
		return err
	}

	staleRows, changedOldRows, changedOrNewRows, err := collectChangedEntrySetsTx(ctx, tx, root.ID, root.Path)
	if err != nil {
		return err
	}

	// The old toolbar progress was driven by the size of the in-memory snapshot,
	// which said "writing 100%" before SQLite had applied the expensive delta.
	// Report progress from the actual changed-set replay so large roots expose
	// meaningful write progress while FTS/bigram updates are running.
	if err := applyChangedEntrySetsTx(ctx, tx, root.Path, staleRows, changedOldRows, changedOrNewRows, !d.isBulkSyncEnabled(), onProgress); err != nil {
		return err
	}
	// Bulk sync now leaves derived bigram rebuilds to EndBulkSync(). Rebuilding
	// here used to duplicate the later full-index maintenance and made large
	// roots pay the finalize cost multiple times without changing the final data.

	return nil
}

func (d *FileSearchDB) replaceSubtreeEntriesTx(ctx context.Context, tx *sql.Tx, batch SubtreeSnapshotBatch) error {
	if d.bulkSyncFullRunRootFresh(batch.RootID) {
		// Scanner full runs prepare only sealed, non-overlapping subtree scopes
		// here. Reusing the root-level "fresh at bulk-sync start" fact avoids one
		// scope probe per subtree while keeping the persisted entry set identical.
		return insertEntriesAsNewFactsTx(ctx, tx, "subtree_bulk_fresh_root_insert", batch.ScopePath, batch.Entries)
	}
	if d.isBulkSyncEnabled() {
		// Full runs rebuild search artifacts once at bulk finalize. When this
		// subtree scope has no persisted rows yet, the old fast path still staged
		// every entry and then copied the same rows into facts. Probe first so new
		// scopes can insert facts directly without the extra temp-table writes.
		scopeHasEntries, err := hasPersistedSubtreeEntriesTx(ctx, tx, batch.RootID, batch.ScopePath)
		if err != nil {
			return err
		}
		if !scopeHasEntries {
			return insertEntriesAsNewFactsTx(ctx, tx, "subtree_bulk_empty_scope_insert", batch.ScopePath, batch.Entries)
		}
	}

	if err := stageEntryRecordsTx(ctx, tx, batch.Entries); err != nil {
		return err
	}

	staleRows, changedOldRows, changedOrNewRows, err := collectChangedEntrySetsTx(ctx, tx, batch.RootID, batch.ScopePath)
	if err != nil {
		return err
	}

	// Full runs already defer FTS maintenance to one rebuild at bulk finalize.
	// The previous subtree path still maintained derived search artifacts row by
	// row during bulk sync, so large scopes paid the per-entry replay cost and
	// then paid the global rebuild anyway.
	return applyChangedEntrySetsTx(ctx, tx, batch.ScopePath, staleRows, changedOldRows, changedOrNewRows, !d.isBulkSyncEnabled(), nil)
}

func (d *FileSearchDB) replaceSubtreeEntriesFromStageTx(ctx context.Context, tx *sql.Tx, rootID string, scopePath string) error {
	staleRows, changedOldRows, changedOrNewRows, err := collectChangedEntrySetsTx(ctx, tx, rootID, scopePath)
	if err != nil {
		return err
	}

	// Streaming subtree jobs stage rows incrementally, then reuse the same scoped
	// diff/replay contract as materialized subtree batches. This keeps delete and
	// rename correctness for non-fresh roots while removing the duplicate pre-walk.
	return applyChangedEntrySetsTx(ctx, tx, scopePath, staleRows, changedOldRows, changedOrNewRows, !d.isBulkSyncEnabled(), nil)
}

func hasPersistedDirectFilesEntriesTx(ctx context.Context, tx *sql.Tx, rootID string, scopePath string) (bool, error) {
	row := tx.QueryRowContext(ctx, `
		SELECT 1
		FROM entries e
		WHERE e.root_id = ?
		  AND (e.path = ? OR (e.parent_path = ? AND e.is_dir = 0))
		LIMIT 1
	`, rootID, scopePath, scopePath)

	var exists int
	if err := row.Scan(&exists); err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func hasPersistedSubtreeEntriesTx(ctx context.Context, tx *sql.Tx, rootID string, scopePath string) (bool, error) {
	scopeQuery, scopeArgs := buildEntryScopeQuery(scopePath, "e.path")
	args := append([]any{rootID}, scopeArgs...)
	row := tx.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT 1
		FROM entries e
		WHERE e.root_id = ? AND %s
		LIMIT 1
	`, scopeQuery), args...)

	var exists int
	if err := row.Scan(&exists); err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (d *FileSearchDB) isRootFreshAtBulkSyncStart(ctx context.Context, rootID string) (bool, error) {
	row := d.db.QueryRowContext(ctx, `
		SELECT CASE
			WHEN EXISTS (SELECT 1 FROM entries WHERE root_id = ? LIMIT 1) THEN 0
			WHEN EXISTS (SELECT 1 FROM directories WHERE root_id = ? LIMIT 1) THEN 0
			ELSE 1
		END
	`, rootID, rootID)

	var fresh int
	if err := row.Scan(&fresh); err != nil {
		return false, fmt.Errorf("load bulk-sync full-run root baseline %q: %w", rootID, err)
	}
	return fresh == 1, nil
}

func insertStagedEntriesAsNewFactsTx(ctx context.Context, tx *sql.Tx, operation string, scopePath string) error {
	startedAt := util.GetSystemTimestamp()
	result, err := tx.ExecContext(ctx, `
		INSERT INTO entries (
			path, root_id, parent_path, name, normalized_name, name_key, normalized_path,
			pinyin_full, pinyin_initials, extension, is_dir, mtime, size, updated_at
		)
		SELECT
			path, root_id, parent_path, name, normalized_name, name_key, normalized_path,
			pinyin_full, pinyin_initials, extension, is_dir, mtime, size, updated_at
		FROM filesearch_stage_entries
		ORDER BY path ASC
	`)
	if err != nil {
		return fmt.Errorf("insert staged entries for scope %q: %w", scopePath, err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	logFilesearchSQLiteMaintenance(ctx, operation, scopePath, util.GetSystemTimestamp()-startedAt, int(rowsAffected))
	return nil
}

func insertEntriesAsNewFactsTx(ctx context.Context, tx *sql.Tx, operation string, scopePath string, entries []EntryRecord) error {
	startedAt := util.GetSystemTimestamp()
	if len(entries) == 0 {
		logFilesearchSQLiteMaintenance(ctx, operation, scopePath, util.GetSystemTimestamp()-startedAt, 0)
		return nil
	}

	// The staged fast path inserted facts with ORDER BY path ASC. Preserve that
	// deterministic order when bypassing the temp table so bulk full scans keep
	// the same entry_id assignment and derived search rows as before.
	orderedEntries := append([]EntryRecord(nil), entries...)
	sort.Slice(orderedEntries, func(left int, right int) bool {
		return orderedEntries[left].Path < orderedEntries[right].Path
	})

	if err := insertEntryFactsNoReturningBatchTx(ctx, tx, orderedEntries); err != nil {
		return fmt.Errorf("insert direct entries for scope %q: %w", scopePath, err)
	}

	logFilesearchSQLiteMaintenance(ctx, operation, scopePath, util.GetSystemTimestamp()-startedAt, len(orderedEntries))
	return nil
}

func upsertEntryFactsTx(ctx context.Context, tx *sql.Tx, row storedEntryRecord) (storedEntryRecord, error) {
	mutator, err := newEntryFactMutatorTx(ctx, tx)
	if err != nil {
		return storedEntryRecord{}, err
	}
	defer mutator.Close()
	return upsertEntryFactsWithMutatorTx(ctx, mutator, row)
}

func upsertEntryFactsWithMutatorTx(ctx context.Context, mutator *entryFactMutatorTx, row storedEntryRecord) (storedEntryRecord, error) {
	scanner := mutator.upsertStmt.QueryRowContext(ctx,
		row.Path,
		row.RootID,
		row.ParentPath,
		row.Name,
		row.NormalizedName,
		row.NameKey,
		row.NormalizedPath,
		row.PinyinFull,
		row.PinyinInitials,
		row.Extension,
		boolToInt(row.IsDir),
		row.Mtime,
		row.Size,
		row.UpdatedAt,
	)

	current, err := scanStoredEntryRecord(scanner)
	if err != nil {
		return storedEntryRecord{}, fmt.Errorf("upsert entry %q: %w", row.Path, err)
	}
	return current, nil
}

func deleteDirectDeltaEntryByPathTx(ctx context.Context, tx *sql.Tx, artifactSync *entrySearchArtifactSyncTx, factMutator *entryFactMutatorTx, path string) (bool, error) {
	existing, ok, err := selectStoredEntryByPathTx(ctx, tx, path)
	if err != nil || !ok {
		return false, err
	}
	if err := deleteEntrySearchArtifactsWithSyncTx(ctx, artifactSync, existing); err != nil {
		return false, err
	}
	if err := deleteEntryBigramsTx(ctx, tx, existing.EntryID); err != nil {
		return false, err
	}
	if _, err := factMutator.deleteStmt.ExecContext(ctx, existing.EntryID); err != nil {
		return false, fmt.Errorf("delete direct-delta entry %q: %w", path, err)
	}
	return true, nil
}

func upsertDirectDeltaEntryTx(ctx context.Context, tx *sql.Tx, artifactSync *entrySearchArtifactSyncTx, factMutator *entryFactMutatorTx, entry EntryRecord) (bool, error) {
	next := buildStoredEntryRecord(entry)
	existing, ok, err := selectStoredEntryByPathTx(ctx, tx, next.Path)
	if err != nil {
		return false, err
	}
	if ok && !entrySearchContentChanged(existing, next) {
		// Bug fix: exact file deltas must not rebuild FTS for unchanged content.
		// updated_at only records that this path was observed by the latest
		// watcher pass, so a no-op delta updates that marker without touching
		// derived search artifacts.
		if _, err := tx.ExecContext(ctx, `UPDATE entries SET updated_at = ? WHERE entry_id = ?`, next.UpdatedAt, existing.EntryID); err != nil {
			return false, fmt.Errorf("update direct-delta timestamp %q: %w", next.Path, err)
		}
		return false, nil
	}
	if ok {
		if err := deleteEntrySearchArtifactsWithSyncTx(ctx, artifactSync, existing); err != nil {
			return false, err
		}
		if err := deleteEntryBigramsTx(ctx, tx, existing.EntryID); err != nil {
			return false, err
		}
	}

	current, err := upsertEntryFactsWithMutatorTx(ctx, factMutator, next)
	if err != nil {
		return false, err
	}
	if err := insertEntrySearchArtifactsWithSyncTx(ctx, artifactSync, current); err != nil {
		return false, err
	}
	return true, nil
}

func entrySearchContentChanged(left storedEntryRecord, right storedEntryRecord) bool {
	return left.RootID != right.RootID ||
		left.ParentPath != right.ParentPath ||
		left.Name != right.Name ||
		left.NormalizedName != right.NormalizedName ||
		left.NameKey != right.NameKey ||
		left.NormalizedPath != right.NormalizedPath ||
		left.PinyinFull != right.PinyinFull ||
		left.PinyinInitials != right.PinyinInitials ||
		left.Extension != right.Extension ||
		left.IsDir != right.IsDir ||
		left.Mtime != right.Mtime ||
		left.Size != right.Size
}

func deleteEntryBigramsTx(ctx context.Context, tx *sql.Tx, entryID int64) error {
	if entryID == 0 {
		return nil
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM entries_bigram WHERE entry_id = ?`, entryID); err != nil {
		return fmt.Errorf("delete direct-delta bigrams for entry %d: %w", entryID, err)
	}
	return nil
}

func collectChangedEntrySetsTx(ctx context.Context, tx *sql.Tx, rootID string, scopePath string) ([]storedEntryRecord, []storedEntryRecord, []storedEntryRecord, error) {
	scopeQuery, scopeArgs := buildEntryScopeQuery(scopePath, "e.path")
	// Compare the staged snapshot against persisted facts inside SQLite so no-op
	// subtree refreshes do not have to materialize every root row back into Go
	// just to discover that nothing changed.
	staleRowsStartedAt := util.GetSystemTimestamp()
	staleRows, err := selectStoredEntriesTx(ctx, tx, fmt.Sprintf(`
		SELECT e.entry_id, e.path, e.root_id, e.parent_path, e.name, e.normalized_name, e.name_key, e.normalized_path,
		       e.pinyin_full, e.pinyin_initials, e.extension, e.is_dir, e.mtime, e.size, e.updated_at
		FROM entries e
		LEFT JOIN filesearch_stage_entries s ON s.path = e.path
		WHERE e.root_id = ? AND %s AND s.path IS NULL
		ORDER BY e.path ASC
		`, scopeQuery), append([]any{rootID}, scopeArgs...)...)
	if err != nil {
		return nil, nil, nil, err
	}
	// The previous diagnostics only exposed the combined changed-set apply time. These
	// query-level timings identify whether subtree jobs are stalling before replay,
	// especially for tiny scopes where SQL diff collection can dominate the wall time.
	logFilesearchSQLiteMaintenance(ctx, "collect_diff_stale", scopePath, util.GetSystemTimestamp()-staleRowsStartedAt, len(staleRows))

	diffPredicate := buildEntryDifferencePredicate("e", "s")
	changedOldRowsStartedAt := util.GetSystemTimestamp()
	changedOldRows, err := selectStoredEntriesTx(ctx, tx, fmt.Sprintf(`
		SELECT e.entry_id, e.path, e.root_id, e.parent_path, e.name, e.normalized_name, e.name_key, e.normalized_path,
		       e.pinyin_full, e.pinyin_initials, e.extension, e.is_dir, e.mtime, e.size, e.updated_at
		FROM entries e
		INNER JOIN filesearch_stage_entries s ON s.path = e.path
		WHERE e.root_id = ? AND %s AND (%s)
		ORDER BY e.path ASC
		`, scopeQuery, diffPredicate), append([]any{rootID}, scopeArgs...)...)
	if err != nil {
		return nil, nil, nil, err
	}
	logFilesearchSQLiteMaintenance(ctx, "collect_diff_changed_old", scopePath, util.GetSystemTimestamp()-changedOldRowsStartedAt, len(changedOldRows))

	changedOrNewRowsStartedAt := util.GetSystemTimestamp()
	changedOrNewRows, err := selectStoredEntriesTx(ctx, tx, fmt.Sprintf(`
		SELECT CAST(COALESCE(e.entry_id, 0) AS INTEGER) AS entry_id,
		       s.path, s.root_id, s.parent_path, s.name, s.normalized_name, s.name_key, s.normalized_path,
		       s.pinyin_full, s.pinyin_initials, s.extension, s.is_dir, s.mtime, s.size, s.updated_at
		FROM filesearch_stage_entries s
		LEFT JOIN entries e ON e.path = s.path
		WHERE e.entry_id IS NULL OR (%s)
		ORDER BY s.path ASC
		`, diffPredicate), nil...)
	if err != nil {
		return nil, nil, nil, err
	}
	logFilesearchSQLiteMaintenance(ctx, "collect_diff_changed_or_new", scopePath, util.GetSystemTimestamp()-changedOrNewRowsStartedAt, len(changedOrNewRows))

	return staleRows, changedOldRows, changedOrNewRows, nil
}

func applyChangedEntrySetsTx(ctx context.Context, tx *sql.Tx, scopePath string, staleRows []storedEntryRecord, changedOldRows []storedEntryRecord, changedOrNewRows []storedEntryRecord, syncSearchArtifacts bool, onProgress func(current int64, total int64)) error {
	var artifactSync *entrySearchArtifactSyncTx
	var factMutator *entryFactMutatorTx
	var err error
	if syncSearchArtifacts {
		// Large subtree refreshes spend most of their time replaying the derived
		// search indexes. Reusing prepared statements within the transaction keeps
		// the SQLite-first path from paying a prepare/close round-trip for every row.
		artifactSync, err = newEntrySearchArtifactSyncTx(ctx, tx)
		if err != nil {
			return err
		}
		defer artifactSync.Close()
	}
	if len(staleRows) > 0 || len(changedOrNewRows) > 0 {
		// Bulk sync no longer needs each upserted row echoed back because FTS
		// maintenance is deferred. Preparing only the statement shape that the
		// current path needs removes an avoidable RETURNING/scan round-trip.
		factMutator, err = newEntryFactMutatorTx(ctx, tx)
		if err != nil {
			return err
		}
		defer factMutator.Close()
	}

	totalChangedRows := int64(len(staleRows) + len(changedOrNewRows))
	completedRows := int64(0)
	lastReportedCurrent := int64(-1)
	lastReportedAt := time.Now()
	reportProgress := func(force bool) {
		if onProgress == nil || totalChangedRows <= 0 {
			return
		}
		if !force && completedRows != totalChangedRows && completedRows%progressBatchSize != 0 && time.Since(lastReportedAt) < progressUpdateGap {
			return
		}
		if completedRows == lastReportedCurrent {
			return
		}
		onProgress(completedRows, totalChangedRows)
		lastReportedCurrent = completedRows
		lastReportedAt = time.Now()
	}
	reportProgress(true)

	deleteArtifactsStartedAt := util.GetSystemTimestamp()
	if syncSearchArtifacts {
		for _, existing := range staleRows {
			if err := deleteEntrySearchArtifactsWithSyncTx(ctx, artifactSync, existing); err != nil {
				return err
			}
		}
		for _, existing := range changedOldRows {
			if err := deleteEntrySearchArtifactsWithSyncTx(ctx, artifactSync, existing); err != nil {
				return err
			}
		}
	}
	// Direct-files hot scopes such as WinSxS can spend most of their write time
	// replaying derived search artifacts. Logging each replay block clarifies
	// whether the stall is in FTS/bigram cleanup or the fact-table mutations.
	if syncSearchArtifacts {
		logFilesearchSQLiteMaintenance(ctx, "changed_set_delete_artifacts", scopePath, util.GetSystemTimestamp()-deleteArtifactsStartedAt, len(staleRows)+len(changedOldRows))
	}

	deleteFactsStartedAt := util.GetSystemTimestamp()
	for _, existing := range staleRows {
		if _, err := factMutator.deleteStmt.ExecContext(ctx, existing.EntryID); err != nil {
			return fmt.Errorf("delete stale entry %q: %w", existing.Path, err)
		}
		completedRows++
		reportProgress(false)
	}
	logFilesearchSQLiteMaintenance(ctx, "changed_set_delete_facts", scopePath, util.GetSystemTimestamp()-deleteFactsStartedAt, len(staleRows))

	upsertFactsStartedAt := util.GetSystemTimestamp()
	for _, staged := range changedOrNewRows {
		current, err := upsertEntryFactsWithMutatorTx(ctx, factMutator, staged)
		if err != nil {
			return err
		}
		if syncSearchArtifacts {
			if err := insertEntrySearchArtifactsWithSyncTx(ctx, artifactSync, current); err != nil {
				return err
			}
		}
		completedRows++
		reportProgress(false)
	}
	logFilesearchSQLiteMaintenance(ctx, "changed_set_upsert_facts", scopePath, util.GetSystemTimestamp()-upsertFactsStartedAt, len(changedOrNewRows))

	reportProgress(true)

	return nil
}

type entryFactMutatorTx struct {
	statements []*sql.Stmt
	upsertStmt *sql.Stmt
	deleteStmt *sql.Stmt
}

func newEntryFactMutatorTx(ctx context.Context, tx *sql.Tx) (*entryFactMutatorTx, error) {
	mutator := &entryFactMutatorTx{}

	prepare := func(query string) (*sql.Stmt, error) {
		stmt, err := tx.PrepareContext(ctx, query)
		if err != nil {
			return nil, err
		}
		mutator.statements = append(mutator.statements, stmt)
		return stmt, nil
	}

	var err error
	if mutator.upsertStmt, err = prepare(`
		INSERT INTO entries (
			path, root_id, parent_path, name, normalized_name, name_key, normalized_path,
			pinyin_full, pinyin_initials, extension, is_dir, mtime, size, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET
			root_id = excluded.root_id,
			parent_path = excluded.parent_path,
			name = excluded.name,
			normalized_name = excluded.normalized_name,
			name_key = excluded.name_key,
			normalized_path = excluded.normalized_path,
			pinyin_full = excluded.pinyin_full,
			pinyin_initials = excluded.pinyin_initials,
			extension = excluded.extension,
			is_dir = excluded.is_dir,
			mtime = excluded.mtime,
			size = excluded.size,
			updated_at = excluded.updated_at
		RETURNING entry_id, path, root_id, parent_path, name, normalized_name, name_key, normalized_path,
		          pinyin_full, pinyin_initials, extension, is_dir, mtime, size, updated_at
	`); err != nil {
		return nil, fmt.Errorf("prepare entry upsert: %w", err)
	}
	if mutator.deleteStmt, err = prepare(`DELETE FROM entries WHERE entry_id = ?`); err != nil {
		return nil, fmt.Errorf("prepare entry delete: %w", err)
	}

	return mutator, nil
}

func prepareEntryFactUpsertNoReturningStmtTx(ctx context.Context, tx *sql.Tx) (*sql.Stmt, error) {
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO entries (
			path, root_id, parent_path, name, normalized_name, name_key, normalized_path,
			pinyin_full, pinyin_initials, extension, is_dir, mtime, size, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET
			root_id = excluded.root_id,
			parent_path = excluded.parent_path,
			name = excluded.name,
			normalized_name = excluded.normalized_name,
			name_key = excluded.name_key,
			normalized_path = excluded.normalized_path,
			pinyin_full = excluded.pinyin_full,
			pinyin_initials = excluded.pinyin_initials,
			extension = excluded.extension,
			is_dir = excluded.is_dir,
			mtime = excluded.mtime,
			size = excluded.size,
			updated_at = excluded.updated_at
	`)
	if err != nil {
		return nil, fmt.Errorf("prepare entry upsert without returning: %w", err)
	}
	return stmt, nil
}

func sqliteBatchRows(totalRows int, columnCount int) int {
	if totalRows <= 0 {
		return 0
	}
	if columnCount <= 0 {
		return totalRows
	}
	// Optimization: bulk full-index writes should reduce sqlite3_step calls but
	// stay within the bundled SQLite variable limit. The streaming scanner emits
	// 2048-record chunks, so 2000 rows preserves bounded statements while avoiding
	// four smaller SQL statements for the common full batch.
	if totalRows < sqlitePreferredBatchRows {
		return totalRows
	}
	return sqlitePreferredBatchRows
}

func upsertEntryFactsNoReturningWithStmtTx(ctx context.Context, stmt *sql.Stmt, entries []EntryRecord) error {
	for _, entry := range entries {
		row := buildStoredEntryRecord(entry)
		if _, err := stmt.ExecContext(
			ctx,
			row.Path,
			row.RootID,
			row.ParentPath,
			row.Name,
			row.NormalizedName,
			row.NameKey,
			row.NormalizedPath,
			row.PinyinFull,
			row.PinyinInitials,
			row.Extension,
			boolToInt(row.IsDir),
			row.Mtime,
			row.Size,
			row.UpdatedAt,
		); err != nil {
			return fmt.Errorf("upsert entry without returning %q: %w", row.Path, err)
		}
	}
	return nil
}

func upsertEntryFactsNoReturningBatchTx(ctx context.Context, tx *sql.Tx, entries []EntryRecord) error {
	if len(entries) == 0 {
		return nil
	}

	// Optimization: fresh full-index roots have no stale facts, so they can write
	// rows directly in multi-value chunks. Building stored rows once per callback
	// avoids repeating normalization when the chunked SQL statements are created.
	rows := make([]storedEntryRecord, 0, len(entries))
	for _, entry := range entries {
		rows = append(rows, buildStoredEntryRecord(entry))
	}

	for start := 0; start < len(rows); {
		chunkSize := sqliteBatchRows(len(rows)-start, entryFactColumnCount)
		end := start + chunkSize
		if err := writeStoredEntryFactRowsNoReturningBatchTx(ctx, tx, rows[start:end], false); err != nil {
			return err
		}
		start = end
	}
	return nil
}

// insertEntryFactsNoReturningBatchTx skips conflict handling for fresh full-index
// roots, where the sealed plan owns non-overlapping entry paths.
func insertEntryFactsNoReturningBatchTx(ctx context.Context, tx *sql.Tx, entries []EntryRecord) error {
	if len(entries) == 0 {
		return nil
	}

	rows := make([]storedEntryRecord, 0, len(entries))
	for _, entry := range entries {
		rows = append(rows, buildStoredEntryRecord(entry))
	}

	for start := 0; start < len(rows); {
		chunkSize := sqliteBatchRows(len(rows)-start, entryFactColumnCount)
		end := start + chunkSize
		if err := writeStoredEntryFactRowsNoReturningBatchTx(ctx, tx, rows[start:end], true); err != nil {
			return err
		}
		start = end
	}
	return nil
}

// writeStoredEntryFactRowsNoReturningBatchTx writes one chunk of entry facts and
// can optionally omit ON CONFLICT for fresh bulk-loads.
func writeStoredEntryFactRowsNoReturningBatchTx(ctx context.Context, tx *sql.Tx, rows []storedEntryRecord, plainInsert bool) error {
	if len(rows) == 0 {
		return nil
	}

	var builder strings.Builder
	builder.WriteString(`
		INSERT INTO entries (
			path, root_id, parent_path, name, normalized_name, name_key, normalized_path,
			pinyin_full, pinyin_initials, extension, is_dir, mtime, size, updated_at
		) VALUES `)

	args := make([]any, 0, len(rows)*entryFactColumnCount)
	for index, row := range rows {
		if index > 0 {
			builder.WriteString(",")
		}
		builder.WriteString("(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
		args = append(args,
			row.Path,
			row.RootID,
			row.ParentPath,
			row.Name,
			row.NormalizedName,
			row.NameKey,
			row.NormalizedPath,
			row.PinyinFull,
			row.PinyinInitials,
			row.Extension,
			boolToInt(row.IsDir),
			row.Mtime,
			row.Size,
			row.UpdatedAt,
		)
	}

	if !plainInsert {
		builder.WriteString(`
			ON CONFLICT(path) DO UPDATE SET
				root_id = excluded.root_id,
				parent_path = excluded.parent_path,
				name = excluded.name,
				normalized_name = excluded.normalized_name,
				name_key = excluded.name_key,
				normalized_path = excluded.normalized_path,
				pinyin_full = excluded.pinyin_full,
				pinyin_initials = excluded.pinyin_initials,
				extension = excluded.extension,
				is_dir = excluded.is_dir,
				mtime = excluded.mtime,
				size = excluded.size,
				updated_at = excluded.updated_at
		`)
	}
	if _, err := tx.ExecContext(ctx, builder.String(), args...); err != nil {
		operation := "upsert"
		if plainInsert {
			operation = "insert"
		}
		return fmt.Errorf("batch %s %d entry facts: %w", operation, len(rows), err)
	}
	return nil
}

func (m *entryFactMutatorTx) Close() {
	if m == nil {
		return
	}
	for _, stmt := range m.statements {
		stmt.Close()
	}
}

type entrySearchArtifactSyncTx struct {
	statements []*sql.Stmt

	deleteNameFTSStmt       *sql.Stmt
	deletePathFTSStmt       *sql.Stmt
	deletePinyinFullFTSStmt *sql.Stmt
	deleteInitialsFTSStmt   *sql.Stmt

	insertNameFTSStmt       *sql.Stmt
	insertPathFTSStmt       *sql.Stmt
	insertPinyinFullFTSStmt *sql.Stmt
	insertInitialsFTSStmt   *sql.Stmt
}

func newEntrySearchArtifactSyncTx(ctx context.Context, tx *sql.Tx) (*entrySearchArtifactSyncTx, error) {
	syncer := &entrySearchArtifactSyncTx{}

	prepare := func(query string) (*sql.Stmt, error) {
		stmt, err := tx.PrepareContext(ctx, query)
		if err != nil {
			return nil, err
		}
		syncer.statements = append(syncer.statements, stmt)
		return stmt, nil
	}

	var err error
	if syncer.deleteNameFTSStmt, err = prepare(`INSERT INTO entries_name_fts(entries_name_fts, rowid, normalized_name) VALUES('delete', ?, ?)`); err != nil {
		return nil, fmt.Errorf("prepare name fts delete: %w", err)
	}
	if syncer.deletePathFTSStmt, err = prepare(`INSERT INTO entries_path_fts(entries_path_fts, rowid, normalized_path) VALUES('delete', ?, ?)`); err != nil {
		return nil, fmt.Errorf("prepare path fts delete: %w", err)
	}
	if syncer.deletePinyinFullFTSStmt, err = prepare(`INSERT INTO entries_pinyin_full_fts(entries_pinyin_full_fts, rowid, pinyin_full) VALUES('delete', ?, ?)`); err != nil {
		return nil, fmt.Errorf("prepare pinyin full fts delete: %w", err)
	}
	if syncer.deleteInitialsFTSStmt, err = prepare(`INSERT INTO entries_initials_fts(entries_initials_fts, rowid, pinyin_initials) VALUES('delete', ?, ?)`); err != nil {
		return nil, fmt.Errorf("prepare initials fts delete: %w", err)
	}

	if syncer.insertNameFTSStmt, err = prepare(`INSERT INTO entries_name_fts(rowid, normalized_name) VALUES(?, ?)`); err != nil {
		return nil, fmt.Errorf("prepare name fts insert: %w", err)
	}
	if syncer.insertPathFTSStmt, err = prepare(`INSERT INTO entries_path_fts(rowid, normalized_path) VALUES(?, ?)`); err != nil {
		return nil, fmt.Errorf("prepare path fts insert: %w", err)
	}
	if syncer.insertPinyinFullFTSStmt, err = prepare(`INSERT INTO entries_pinyin_full_fts(rowid, pinyin_full) VALUES(?, ?)`); err != nil {
		return nil, fmt.Errorf("prepare pinyin full fts insert: %w", err)
	}
	if syncer.insertInitialsFTSStmt, err = prepare(`INSERT INTO entries_initials_fts(rowid, pinyin_initials) VALUES(?, ?)`); err != nil {
		return nil, fmt.Errorf("prepare initials fts insert: %w", err)
	}

	return syncer, nil
}

func (s *entrySearchArtifactSyncTx) Close() {
	if s == nil {
		return
	}
	for _, stmt := range s.statements {
		stmt.Close()
	}
}

func selectStoredEntriesTx(ctx context.Context, tx *sql.Tx, query string, args ...any) ([]storedEntryRecord, error) {
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var loaded []storedEntryRecord
	for rows.Next() {
		row, err := scanStoredEntryRecord(rows)
		if err != nil {
			return nil, err
		}
		loaded = append(loaded, row)
	}
	return loaded, rows.Err()
}

func selectStoredEntryByPathTx(ctx context.Context, tx *sql.Tx, path string) (storedEntryRecord, bool, error) {
	rows, err := selectStoredEntriesTx(ctx, tx, `
		SELECT entry_id, path, root_id, parent_path, name, normalized_name, name_key, normalized_path,
		       pinyin_full, pinyin_initials, extension, is_dir, mtime, size, updated_at
		FROM entries
		WHERE path = ?
	`, path)
	if err != nil {
		return storedEntryRecord{}, false, err
	}
	if len(rows) == 0 {
		return storedEntryRecord{}, false, nil
	}
	return rows[0], true, nil
}

func buildScopedPathQuery(scopePath string, column string) (string, []any) {
	cleanScope := filepath.Clean(scopePath)
	scopePrefix := cleanScope + string(filepath.Separator)
	// The previous LIKE-based subtree predicate was correct but SQLite could still
	// spend most of an incremental restore scanning a large root before comparing
	// staged rows. Use a byte-ordered range that matches the existing
	// (root_id, path) index so subtree diffs seek directly to the scoped prefix
	// while the equality branch keeps the scope directory entry itself included.
	return fmt.Sprintf("(%s = ? OR (%s >= ? AND %s < ?))", column, column, column), []any{
		cleanScope,
		scopePrefix,
		nextPathPrefixUpperBound(scopePrefix),
	}
}

func buildEntryScopeQuery(scopePath string, column string) (string, []any) {
	return buildScopedPathQuery(scopePath, column)
}

func nextPathPrefixUpperBound(prefix string) string {
	if prefix == "" {
		return prefix
	}

	bytes := []byte(prefix)
	for index := len(bytes) - 1; index >= 0; index-- {
		if bytes[index] == 0xff {
			continue
		}
		bytes[index]++
		return string(bytes[:index+1])
	}

	return prefix + "\x00"
}

func buildEntryDifferencePredicate(existingAlias string, stagedAlias string) string {
	left := strings.TrimSpace(existingAlias)
	right := strings.TrimSpace(stagedAlias)
	// Bug fix: updated_at is a write marker, not file identity. Including it made
	// every subtree reconcile rewrite unchanged rows because each scan gets a new
	// timestamp even when mtime, size, names, and search keys are identical.
	return fmt.Sprintf(
		"%s.root_id <> %s.root_id OR %s.parent_path <> %s.parent_path OR %s.name <> %s.name OR %s.normalized_name <> %s.normalized_name OR %s.name_key <> %s.name_key OR %s.normalized_path <> %s.normalized_path OR %s.pinyin_full <> %s.pinyin_full OR %s.pinyin_initials <> %s.pinyin_initials OR %s.extension <> %s.extension OR %s.is_dir <> %s.is_dir OR %s.mtime <> %s.mtime OR %s.size <> %s.size",
		left, right,
		left, right,
		left, right,
		left, right,
		left, right,
		left, right,
		left, right,
		left, right,
		left, right,
		left, right,
		left, right,
		left, right,
	)
}

func refreshRootBigramsTx(ctx context.Context, tx *sql.Tx, rootID string) error {
	startedAt := util.GetSystemTimestamp()
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM entries_bigram
		WHERE entry_id IN (SELECT entry_id FROM entries WHERE root_id = ?)
	`, rootID); err != nil {
		return fmt.Errorf("clear root bigrams for %s: %w", rootID, err)
	}
	// Two-character search no longer uses substring bigrams, so keeping the
	// table empty avoids rebuild work without affecting the remaining query
	// paths. The log stays in place so traces still show that the step is cheap.
	logFilesearchSQLiteMaintenance(ctx, "refresh_root_bigrams", rootID, util.GetSystemTimestamp()-startedAt, 0)
	return nil
}

func deleteEntrySearchArtifactsTx(ctx context.Context, tx *sql.Tx, row storedEntryRecord) error {
	syncer, err := newEntrySearchArtifactSyncTx(ctx, tx)
	if err != nil {
		return err
	}
	defer syncer.Close()
	return deleteEntrySearchArtifactsWithSyncTx(ctx, syncer, row)
}

func deleteEntrySearchArtifactsWithSyncTx(ctx context.Context, syncer *entrySearchArtifactSyncTx, row storedEntryRecord) error {
	if row.EntryID == 0 {
		return nil
	}
	if err := deleteEntryFTSWithSyncTx(ctx, syncer, row); err != nil {
		return err
	}
	return nil
}

func insertEntrySearchArtifactsTx(ctx context.Context, tx *sql.Tx, row storedEntryRecord) error {
	syncer, err := newEntrySearchArtifactSyncTx(ctx, tx)
	if err != nil {
		return err
	}
	defer syncer.Close()
	return insertEntrySearchArtifactsWithSyncTx(ctx, syncer, row)
}

func insertEntrySearchArtifactsWithSyncTx(ctx context.Context, syncer *entrySearchArtifactSyncTx, row storedEntryRecord) error {
	if row.EntryID == 0 {
		return nil
	}
	if err := insertEntryFTSWithSyncTx(ctx, syncer, row); err != nil {
		return err
	}
	return nil
}

func deleteEntryFTSTx(ctx context.Context, tx *sql.Tx, row storedEntryRecord) error {
	syncer, err := newEntrySearchArtifactSyncTx(ctx, tx)
	if err != nil {
		return err
	}
	defer syncer.Close()
	return deleteEntryFTSWithSyncTx(ctx, syncer, row)
}

func deleteEntryFTSWithSyncTx(ctx context.Context, syncer *entrySearchArtifactSyncTx, row storedEntryRecord) error {
	commands := []struct {
		name  string
		stmt  *sql.Stmt
		value string
	}{
		{name: "entries_name_fts", stmt: syncer.deleteNameFTSStmt, value: row.NormalizedName},
		{name: "entries_pinyin_full_fts", stmt: syncer.deletePinyinFullFTSStmt, value: row.PinyinFull},
		{name: "entries_initials_fts", stmt: syncer.deleteInitialsFTSStmt, value: row.PinyinInitials},
	}
	if row.IsDir {
		commands = append(commands, struct {
			name  string
			stmt  *sql.Stmt
			value string
		}{name: "entries_path_fts", stmt: syncer.deletePathFTSStmt, value: row.NormalizedPath})
	}
	for _, command := range commands {
		if strings.TrimSpace(command.value) == "" {
			continue
		}
		if _, err := command.stmt.ExecContext(ctx, row.EntryID, command.value); err != nil {
			return fmt.Errorf("delete %s row for %q: %w", command.name, row.Path, err)
		}
	}
	return nil
}

func insertEntryFTSTx(ctx context.Context, tx *sql.Tx, row storedEntryRecord) error {
	syncer, err := newEntrySearchArtifactSyncTx(ctx, tx)
	if err != nil {
		return err
	}
	defer syncer.Close()
	return insertEntryFTSWithSyncTx(ctx, syncer, row)
}

func insertEntryFTSWithSyncTx(ctx context.Context, syncer *entrySearchArtifactSyncTx, row storedEntryRecord) error {
	commands := []struct {
		name  string
		stmt  *sql.Stmt
		value string
	}{
		{name: "entries_name_fts", stmt: syncer.insertNameFTSStmt, value: row.NormalizedName},
		{name: "entries_pinyin_full_fts", stmt: syncer.insertPinyinFullFTSStmt, value: row.PinyinFull},
		{name: "entries_initials_fts", stmt: syncer.insertInitialsFTSStmt, value: row.PinyinInitials},
	}
	if row.IsDir {
		commands = append(commands, struct {
			name  string
			stmt  *sql.Stmt
			value string
		}{name: "entries_path_fts", stmt: syncer.insertPathFTSStmt, value: row.NormalizedPath})
	}
	for _, command := range commands {
		if strings.TrimSpace(command.value) == "" {
			continue
		}
		if _, err := command.stmt.ExecContext(ctx, row.EntryID, command.value); err != nil {
			return fmt.Errorf("insert %s row for %q: %w", command.name, row.Path, err)
		}
	}
	return nil
}

func (d *FileSearchDB) finalizeRootRunTx(ctx context.Context, tx *sql.Tx, root RootRecord) error {
	// Finalize is the only place allowed to advance the persisted feed cursor.
	// Applying job rows before this point is safe because a crash can replay the
	// same writes, but advancing the cursor early would acknowledge unseen
	// change-feed signals and permanently skip them on recovery.
	// Bug fix: finalize must not run FTS optimize. That command is global table
	// maintenance, and CPU profiles showed doing it here made every small dirty
	// flush scan all four FTS tables. A scanner timer now runs that compaction
	// every 12 hours instead.
	_, err := tx.ExecContext(ctx, `
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

func (d *FileSearchDB) OptimizeFTSTables(ctx context.Context) error {
	if d == nil || d.db == nil {
		return nil
	}

	startedAt := util.GetSystemTimestamp()
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := optimizeFTSTablesTx(ctx, tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}

	// Optimization: scheduled FTS maintenance is intentionally separated from
	// incremental indexing latency. Keep one timing log at the storage boundary so
	// future CPU profiles can still correlate the 12-hour compaction cost.
	logFilesearchSQLiteMaintenance(ctx, "optimize_fts", "scheduled", util.GetSystemTimestamp()-startedAt, len(filesearchFTSTables))
	return nil
}

func optimizeFTSTablesTx(ctx context.Context, tx *sql.Tx) error {
	for _, tableName := range filesearchFTSTables {
		if _, err := tx.ExecContext(ctx, fmt.Sprintf(`INSERT INTO %s(%s) VALUES('optimize')`, tableName, tableName)); err != nil {
			return fmt.Errorf("optimize %s: %w", tableName, err)
		}
	}
	return nil
}

func (d *FileSearchDB) checkpointWALAfterFinalize(ctx context.Context) {
	if d == nil || d.db == nil || d.isBulkSyncEnabled() {
		return
	}

	// Finalize is the SQLite-first maintenance boundary because it runs after a
	// batch of job writes has committed. A checkpoint miss should not undo the
	// committed facts/cursor state, so this hook is best-effort and only trims
	// WAL growth instead of reviving the old whole-index rebuild path.
	if _, err := d.db.ExecContext(ctx, `PRAGMA wal_checkpoint(PASSIVE)`); err != nil {
		util.GetLogger().Warn(ctx, "filesearch finalize wal checkpoint failed: "+err.Error())
	}
}

func (d *FileSearchDB) rebuildFTSTables(ctx context.Context, optimize bool) error {
	startedAt := util.GetSystemTimestamp()
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	rebuildElapsedMs, optimizeElapsedMs, err := rebuildFTSTablesTimedTx(ctx, tx, optimize)
	if err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	// Full runs deliberately defer FTS maintenance, so log the rebuild cost at
	// the storage boundary instead of attributing the whole pause to generic
	// "finalizing" time higher in the stack.
	logFilesearchSQLiteMaintenance(ctx, "rebuild_fts", fmt.Sprintf("optimize=%t", optimize), rebuildElapsedMs, len(filesearchFTSTables))
	if optimize {
		logFilesearchSQLiteMaintenance(ctx, "optimize_fts", "standalone", optimizeElapsedMs, len(filesearchFTSTables))
	}
	logFilesearchSQLiteMaintenance(ctx, "rebuild_fts_total", fmt.Sprintf("optimize=%t", optimize), util.GetSystemTimestamp()-startedAt, len(filesearchFTSTables))
	return nil
}

func (d *FileSearchDB) rebuildBulkSearchArtifacts(ctx context.Context, optimize bool, freshEmptyIndex bool) error {
	return d.rebuildBulkSearchArtifactsWithBeginner(ctx, d.db, optimize, freshEmptyIndex)
}

func (d *FileSearchDB) rebuildBulkSearchArtifactsWithBeginner(ctx context.Context, beginner sqliteTxBeginner, optimize bool, freshEmptyIndex bool) error {
	totalStartedAt := util.GetSystemTimestamp()
	tx, err := beginner.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	bigramStartedAt := util.GetSystemTimestamp()
	bigramRows, err := rebuildAllBigramsTx(ctx, tx)
	if err != nil {
		return err
	}
	bigramElapsedMs := util.GetSystemTimestamp() - bigramStartedAt
	logFilesearchIndexPhase(ctx, "bulk_finalize_rebuild_bigrams", "bulk", bigramElapsedMs, map[string]any{
		"rows": bigramRows,
	})

	rebuildElapsedMs := int64(0)
	optimizeElapsedMs := int64(0)
	if freshEmptyIndex {
		// Optimization: a manual rebuild that started from an empty entries table
		// also has empty FTS tables. Populate those tables directly from the final
		// facts instead of issuing FTS5's generic rebuild command, which scans the
		// content table for every FTS table and cannot skip empty pinyin payloads.
		if rebuildElapsedMs, err = populateFreshFTSTablesTx(ctx, tx); err != nil {
			return err
		}
	} else {
		rebuildElapsedMs, optimizeElapsedMs, err = rebuildFTSTablesTimedTx(ctx, tx, optimize)
		if err != nil {
			return err
		}
	}
	commitStartedAt := util.GetSystemTimestamp()
	if err := tx.Commit(); err != nil {
		return err
	}
	commitElapsedMs := util.GetSystemTimestamp() - commitStartedAt
	logFilesearchIndexPhase(ctx, "bulk_finalize_commit", "bulk", commitElapsedMs, map[string]any{
		"fresh_empty_index": freshEmptyIndex,
	})

	// Bulk finalization now owns the single source of truth for derived search
	// artifacts. Split the logs so the next trace can tell whether time is going
	// into bigram replay or the later FTS rebuild/optimize phase.
	logFilesearchSQLiteMaintenance(ctx, "rebuild_bigrams", "bulk", bigramElapsedMs, bigramRows)
	if freshEmptyIndex {
		logFilesearchSQLiteMaintenance(ctx, "populate_fts", "bulk_fresh", rebuildElapsedMs, len(filesearchFTSTables))
	} else {
		logFilesearchSQLiteMaintenance(ctx, "rebuild_fts", fmt.Sprintf("optimize=%t", optimize), rebuildElapsedMs, len(filesearchFTSTables))
	}
	if optimize {
		logFilesearchSQLiteMaintenance(ctx, "optimize_fts", "bulk", optimizeElapsedMs, len(filesearchFTSTables))
	}
	logFilesearchIndexPhase(ctx, "bulk_finalize_artifacts_done", "bulk", util.GetSystemTimestamp()-totalStartedAt, map[string]any{
		"fresh_empty_index": freshEmptyIndex,
		"optimize":          optimize,
	})
	return nil
}

func populateFreshFTSTablesTx(ctx context.Context, tx *sql.Tx) (int64, error) {
	startedAt := util.GetSystemTimestamp()
	statements := []struct {
		table  string
		column string
		where  string
	}{
		{table: "entries_name_fts", column: "normalized_name", where: "normalized_name <> ''"},
		{table: "entries_path_fts", column: "normalized_path", where: "is_dir = 1 AND normalized_path <> ''"},
		{table: "entries_pinyin_full_fts", column: "pinyin_full", where: "pinyin_full <> ''"},
		{table: "entries_initials_fts", column: "pinyin_initials", where: "pinyin_initials <> ''"},
	}
	for _, statement := range statements {
		statementStartedAt := util.GetSystemTimestamp()
		logFilesearchIndexPhase(ctx, "bulk_finalize_populate_fts_start", statement.table, 0, map[string]any{
			"column": statement.column,
		})
		result, err := tx.ExecContext(ctx, fmt.Sprintf(`
				INSERT INTO %s(rowid, %s)
				SELECT entry_id, %s
				FROM entries
				WHERE %s
			`, statement.table, statement.column, statement.column, statement.where))
		if err != nil {
			return 0, fmt.Errorf("populate %s: %w", statement.table, err)
		}
		rowsAffected, _ := result.RowsAffected()
		statementElapsedMs := util.GetSystemTimestamp() - statementStartedAt
		// Diagnostic addition: populate_fts used to hide which derived token table
		// was expensive. Keep FTS semantics unchanged, but expose each table so the
		// real-index artifact can decide whether pinyin/initials need a later design.
		logFilesearchSQLiteMaintenance(ctx, "populate_fts_"+statement.table, statement.table, statementElapsedMs, 1)
		logFilesearchIndexPhase(ctx, "bulk_finalize_populate_fts_done", statement.table, statementElapsedMs, map[string]any{
			"rows": rowsAffected,
		})
	}
	return util.GetSystemTimestamp() - startedAt, nil
}

func rebuildFTSTablesTx(ctx context.Context, tx *sql.Tx, optimize bool) error {
	_, _, err := rebuildFTSTablesTimedTx(ctx, tx, optimize)
	return err
}

func rebuildFTSTablesTimedTx(ctx context.Context, tx *sql.Tx, optimize bool) (int64, int64, error) {
	rebuildStartedAt := util.GetSystemTimestamp()
	for _, tableName := range filesearchFTSTables {
		tableStartedAt := util.GetSystemTimestamp()
		logFilesearchIndexPhase(ctx, "bulk_finalize_rebuild_fts_start", tableName, 0, map[string]any{
			"table": tableName,
		})
		if err := rebuildFTSTableTx(ctx, tx, tableName); err != nil {
			return 0, 0, err
		}
		logFilesearchIndexPhase(ctx, "bulk_finalize_rebuild_fts_done", tableName, util.GetSystemTimestamp()-tableStartedAt, map[string]any{
			"table": tableName,
		})
	}
	rebuildElapsedMs := util.GetSystemTimestamp() - rebuildStartedAt
	optimizeElapsedMs := int64(0)
	if optimize {
		optimizeStartedAt := util.GetSystemTimestamp()
		for _, tableName := range filesearchFTSTables {
			tableStartedAt := util.GetSystemTimestamp()
			logFilesearchIndexPhase(ctx, "bulk_finalize_optimize_fts_start", tableName, 0, map[string]any{
				"table": tableName,
			})
			if _, err := tx.ExecContext(ctx, fmt.Sprintf(`INSERT INTO %s(%s) VALUES('optimize')`, tableName, tableName)); err != nil {
				return rebuildElapsedMs, 0, fmt.Errorf("optimize %s: %w", tableName, err)
			}
			logFilesearchIndexPhase(ctx, "bulk_finalize_optimize_fts_done", tableName, util.GetSystemTimestamp()-tableStartedAt, map[string]any{
				"table": tableName,
			})
		}
		optimizeElapsedMs = util.GetSystemTimestamp() - optimizeStartedAt
	}
	return rebuildElapsedMs, optimizeElapsedMs, nil
}

// rebuildFTSTableTx rebuilds one FTS table while keeping path FTS limited to
// directory entries instead of every full file path.
func rebuildFTSTableTx(ctx context.Context, tx *sql.Tx, tableName string) error {
	if tableName != "entries_path_fts" {
		if _, err := tx.ExecContext(ctx, fmt.Sprintf(`INSERT INTO %s(%s) VALUES('rebuild')`, tableName, tableName)); err != nil {
			return fmt.Errorf("rebuild %s: %w", tableName, err)
		}
		return nil
	}

	// Path recall only needs directory paths. Keeping entries_path_fts sparse
	// avoids trigram-indexing every file's full path during bulk finalize.
	if _, err := tx.ExecContext(ctx, `INSERT INTO entries_path_fts(entries_path_fts) VALUES('delete-all')`); err != nil {
		return fmt.Errorf("clear entries_path_fts: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO entries_path_fts(rowid, normalized_path)
		SELECT entry_id, normalized_path
		FROM entries
		WHERE is_dir = 1 AND normalized_path <> ''
	`); err != nil {
		return fmt.Errorf("rebuild entries_path_fts: %w", err)
	}
	return nil
}

func rebuildAllBigramsTx(ctx context.Context, tx *sql.Tx) (int, error) {
	if _, err := tx.ExecContext(ctx, `DELETE FROM entries_bigram`); err != nil {
		return 0, fmt.Errorf("clear entries_bigram: %w", err)
	}
	// Two-character search now uses a narrower prefix-only path, so the bigram
	// side table is intentionally kept empty. Clearing it here removes the
	// expensive rebuild step while keeping the on-disk schema compatible.
	return 0, nil
}
