package filesearch

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"
	"wox/util"

	_ "github.com/mattn/go-sqlite3"
)

// ContentSearchDB owns the optional full-text content index. It is stored in a
// separate SQLite database so large content FTS writes cannot bloat or lock the
// filename search database.
type ContentSearchDB struct {
	db     *sql.DB
	dbPath string
}

// NewContentSearchDB opens the standalone content search database.
func NewContentSearchDB(ctx context.Context) (*ContentSearchDB, error) {
	fileSearchDir := util.GetLocation().GetFileSearchDirectory()
	if err := util.GetLocation().EnsureDirectoryExist(fileSearchDir); err != nil {
		return nil, err
	}

	dbPath := contentSearchDBPath()
	dsn := dbPath + "?" +
		"_journal_mode=WAL&" +
		"_synchronous=NORMAL&" +
		"_cache_size=2000&" +
		"_foreign_keys=true&" +
		"_busy_timeout=5000"

	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open content search database: %w", err)
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)
	db.SetConnMaxLifetime(time.Hour)

	contentDB := &ContentSearchDB{db: db, dbPath: dbPath}
	if err := contentDB.initTables(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return contentDB, nil
}

func contentSearchDBPath() string {
	return filepath.Join(util.GetLocation().GetFileSearchDirectory(), "contentsearch.db")
}

func contentSearchDBFiles() []string {
	dbPath := contentSearchDBPath()
	return []string{dbPath, dbPath + "-wal", dbPath + "-shm"}
}

// removeContentSearchDBFiles removes the standalone content DB and SQLite sidecar files.
func removeContentSearchDBFiles() error {
	for _, path := range contentSearchDBFiles() {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove content search database file %s: %w", path, err)
		}
	}
	return nil
}

// Close closes the standalone content search database.
func (d *ContentSearchDB) Close() error {
	if d == nil || d.db == nil {
		return nil
	}
	return d.db.Close()
}

func (d *ContentSearchDB) initTables(ctx context.Context) error {
	if err := d.probeFTS5(ctx); err != nil {
		return err
	}
	return d.ensureTables(ctx)
}

func (d *ContentSearchDB) probeFTS5(ctx context.Context) error {
	if _, err := d.db.ExecContext(ctx, `
		CREATE VIRTUAL TABLE IF NOT EXISTS temp.contentsearch_fts5_probe USING fts5(value);
	`); err != nil {
		return fmt.Errorf("content search requires sqlite FTS5 support; rebuild with -tags sqlite_fts5: %w", err)
	}
	if _, err := d.db.ExecContext(ctx, `DROP TABLE IF EXISTS temp.contentsearch_fts5_probe`); err != nil {
		return fmt.Errorf("drop content search FTS5 probe table: %w", err)
	}
	return nil
}

func (d *ContentSearchDB) ensureTables(ctx context.Context) error {
	if _, err := d.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS meta (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL
	)`); err != nil {
		return fmt.Errorf("create content search meta table: %w", err)
	}

	resetSchema, err := d.contentSchemaNeedsReset(ctx)
	if err != nil {
		return err
	}
	if resetSchema {
		if err := d.resetContentSchema(ctx); err != nil {
			return err
		}
	}

	statements := []string{
		`CREATE TABLE IF NOT EXISTS content_entries (
			rowid INTEGER PRIMARY KEY AUTOINCREMENT,
			path TEXT NOT NULL UNIQUE,
			mtime INTEGER NOT NULL,
			size INTEGER NOT NULL,
			content_hash INTEGER NOT NULL DEFAULT 0,
			extension TEXT NOT NULL DEFAULT '',
			indexed_text_bytes INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE INDEX IF NOT EXISTS idx_content_entries_path ON content_entries(path)`,
		`CREATE INDEX IF NOT EXISTS idx_content_entries_extension ON content_entries(extension)`,
		// contentless FTS5: content='' means FTS5 doesn't store or look up
		// original text. detail='full' keeps token offsets so quoted phrase
		// queries can be resolved by FTS5 without storing full file contents.
		`CREATE VIRTUAL TABLE IF NOT EXISTS entries_content_fts USING fts5(
			content,
			content='',
			tokenize='unicode61',
			detail='full'
		)`,
	}
	for _, stmt := range statements {
		if _, err := d.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("create content search table: %w", err)
		}
	}
	if _, err := d.db.ExecContext(ctx, `
		INSERT OR REPLACE INTO meta (key, value) VALUES (?, ?)
	`, contentIndexSchemaVersionKey, currentContentIndexSchemaVersion); err != nil {
		return fmt.Errorf("store content search schema version: %w", err)
	}
	return nil
}

func (d *ContentSearchDB) contentSchemaNeedsReset(ctx context.Context) (bool, error) {
	var version string
	err := d.db.QueryRowContext(ctx, `SELECT value FROM meta WHERE key = ?`, contentIndexSchemaVersionKey).Scan(&version)
	if err == sql.ErrNoRows {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("read content search schema version: %w", err)
	}
	return version != currentContentIndexSchemaVersion, nil
}

func (d *ContentSearchDB) resetContentSchema(ctx context.Context) error {
	statements := []string{
		`DROP TABLE IF EXISTS entries_content_fts`,
		`DROP TABLE IF EXISTS content_entries`,
	}
	for _, stmt := range statements {
		if _, err := d.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("reset content search schema: %w", err)
		}
	}
	if _, err := d.db.ExecContext(ctx, `DELETE FROM meta WHERE key = ?`, contentCrawlStateKey); err != nil {
		return fmt.Errorf("clear content crawl state after schema reset: %w", err)
	}
	return nil
}
