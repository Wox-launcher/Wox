package filesearch

import (
	"context"
	"database/sql"
	"fmt"
	"hash/fnv"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ContentSearchResult is one hit from the content index.
type ContentSearchResult struct {
	Path  string
	Score int64
}

// ContentStats is a snapshot of content index statistics for display.
type ContentStats struct {
	DocCount         int
	IndexedTextBytes int64
	CrawlComplete    bool
}

// contentCrawlStateKey is the meta key for content crawl state.
const contentCrawlStateKey = "content_crawl_state"

// IndexContent indexes or updates a file's content in the content index.
// It tokenizes the text, computes a content hash for change detection, and
// inserts into content_entries + entries_content_fts. If the path already
// exists with the same hash, it skips (no change). Returns true if the index
// was actually updated. Retries on "database is locked" up to 3 times.
func (d *FileSearchDB) IndexContent(ctx context.Context, path string, mtime, size int64, extension, text string) (bool, error) {
	if d == nil || d.db == nil {
		return false, fmt.Errorf("filesearch db not open")
	}

	hash := fnv32aContent(text)
	tokenized := TokenizeForContentIndex(text)
	indexedBytes := int64(len(text))

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		updated, err := d.indexContentOnce(ctx, path, mtime, size, extension, hash, tokenized, indexedBytes)
		if err == nil {
			return updated, nil
		}
		lastErr = err
		// Retry only on "database is locked" or "SQLITE_BUSY".
		if !strings.Contains(err.Error(), "locked") && !strings.Contains(err.Error(), "busy") {
			return false, err
		}
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-time.After(time.Duration(100*(attempt+1)) * time.Millisecond):
		}
	}
	return false, lastErr
}

// indexContentOnce performs a single IndexContent attempt without retry.
func (d *FileSearchDB) indexContentOnce(ctx context.Context, path string, mtime, size int64, extension string, hash uint32, tokenized string, indexedBytes int64) (bool, error) {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Check if this path already exists with the same content hash (inside the tx).
	var existingRowid int64
	var existingHash int64
	err = tx.QueryRowContext(ctx, `SELECT rowid, content_hash FROM content_entries WHERE path = ?`, path).Scan(&existingRowid, &existingHash)
	if err == nil && uint32(existingHash) == hash {
		return false, nil // no content change, skip
	}

	// If path exists with different content, delete old FTS row first.
	if err == nil && existingRowid > 0 {
		if _, err := tx.ExecContext(ctx, `INSERT INTO entries_content_fts(entries_content_fts, rowid) VALUES('delete', ?)`, existingRowid); err != nil {
			return false, fmt.Errorf("delete old content fts: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM content_entries WHERE rowid = ?`, existingRowid); err != nil {
			return false, fmt.Errorf("delete old content entry: %w", err)
		}
	}

	// Insert new content_entries row.
	res, err := tx.ExecContext(ctx,
		`INSERT INTO content_entries (path, mtime, size, content_hash, extension, indexed_text_bytes) VALUES (?, ?, ?, ?, ?, ?)`,
		path, mtime, size, int64(hash), extension, indexedBytes)
	if err != nil {
		return false, fmt.Errorf("insert content entry: %w", err)
	}
	newRowid, err := res.LastInsertId()
	if err != nil {
		return false, fmt.Errorf("get last insert id: %w", err)
	}

	// Insert FTS row with tokenized content.
	if tokenized != "" {
		if _, err := tx.ExecContext(ctx, `INSERT INTO entries_content_fts(rowid, content) VALUES (?, ?)`, newRowid, tokenized); err != nil {
			return false, fmt.Errorf("insert content fts: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit: %w", err)
	}
	return true, nil
}

// DeleteContent removes a file's content from the index.
func (d *FileSearchDB) DeleteContent(ctx context.Context, path string) error {
	if d == nil || d.db == nil {
		return fmt.Errorf("filesearch db not open")
	}

	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	var rowid int64
	err = tx.QueryRowContext(ctx, `SELECT rowid FROM content_entries WHERE path = ?`, path).Scan(&rowid)
	if err == sql.ErrNoRows {
		return nil // not indexed, nothing to delete
	}
	if err != nil {
		return fmt.Errorf("query content entry: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO entries_content_fts(entries_content_fts, rowid) VALUES('delete', ?)`, rowid); err != nil {
		return fmt.Errorf("delete content fts: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM content_entries WHERE rowid = ?`, rowid); err != nil {
		return fmt.Errorf("delete content entry: %w", err)
	}

	return tx.Commit()
}

// ResetContentIndex deletes all content index data. Called when the user
// changes the extension whitelist or disables content search.
func (d *FileSearchDB) ResetContentIndex(ctx context.Context) error {
	if d == nil || d.db == nil {
		return fmt.Errorf("filesearch db not open")
	}

	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// FTS5 contentless table: 'delete-all' clears all rows.
	if _, err := tx.ExecContext(ctx, `INSERT INTO entries_content_fts(entries_content_fts) VALUES('delete-all')`); err != nil {
		return fmt.Errorf("clear content fts: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM content_entries`); err != nil {
		return fmt.Errorf("clear content entries: %w", err)
	}
	// Reset crawl state.
	if _, err := tx.ExecContext(ctx, `DELETE FROM meta WHERE key = ?`, contentCrawlStateKey); err != nil {
		return fmt.Errorf("clear crawl state: %w", err)
	}

	return tx.Commit()
}

// SearchContent searches the content index for files matching all query terms.
// Returns results sorted by FTS5 relevance (rank), limited to `limit`.
// Uses the same SQLite page cache as name/path search — equally fast.
func (d *FileSearchDB) SearchContent(ctx context.Context, query string, limit int) ([]ContentSearchResult, error) {
	if d == nil || d.db == nil {
		return nil, fmt.Errorf("filesearch db not open")
	}
	if limit <= 0 {
		limit = 20
	}

	// Tokenize the query the same way content is tokenized.
	tokenized := TokenizeForContentIndex(query)
	if tokenized == "" {
		return nil, nil
	}

	// FTS5 MATCH with space-separated terms is implicit AND by default.
	// Quote the tokenized string to avoid FTS5 syntax injection.
	matchExpr := quoteFTS5Match(tokenized)

	rows, err := d.db.QueryContext(ctx, `
		SELECT ce.path, f.rank
		FROM entries_content_fts f
		JOIN content_entries ce ON ce.rowid = f.rowid
		WHERE f.entries_content_fts MATCH ?
		ORDER BY f.rank
		LIMIT ?
	`, matchExpr, limit)
	if err != nil {
		return nil, fmt.Errorf("content search query: %w", err)
	}
	defer rows.Close()

	var results []ContentSearchResult
	for rows.Next() {
		var path string
		var rank float64
		if err := rows.Scan(&path, &rank); err != nil {
			return nil, err
		}
		// FTS5 rank is lower = better (negative values common). Convert to
		// a positive score for consistent sorting with name/path results.
		score := int64(-rank * 1000)
		if score < 0 {
			score = 0
		}
		results = append(results, ContentSearchResult{Path: path, Score: score})
	}
	return results, rows.Err()
}

// ContentStats returns statistics about the content index for display.
func (d *FileSearchDB) ContentStats(ctx context.Context) (ContentStats, error) {
	if d == nil || d.db == nil {
		return ContentStats{}, fmt.Errorf("filesearch db not open")
	}

	var stats ContentStats
	err := d.db.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(SUM(indexed_text_bytes), 0) FROM content_entries`).Scan(&stats.DocCount, &stats.IndexedTextBytes)
	if err != nil {
		return ContentStats{}, fmt.Errorf("content stats query: %w", err)
	}

	crawlState, _ := d.GetContentCrawlState(ctx)
	stats.CrawlComplete = crawlState == "complete"
	return stats, nil
}

// SetContentCrawlState stores the content crawl state in the meta table.
func (d *FileSearchDB) SetContentCrawlState(ctx context.Context, state string) error {
	if d == nil || d.db == nil {
		return fmt.Errorf("filesearch db not open")
	}
	_, err := d.db.ExecContext(ctx,
		`INSERT INTO meta(key, value) VALUES(?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		contentCrawlStateKey, state)
	return err
}

// GetContentCrawlState reads the content crawl state from the meta table.
func (d *FileSearchDB) GetContentCrawlState(ctx context.Context) (string, error) {
	if d == nil || d.db == nil {
		return "", fmt.Errorf("filesearch db not open")
	}
	var state string
	err := d.db.QueryRowContext(ctx, `SELECT value FROM meta WHERE key = ?`, contentCrawlStateKey).Scan(&state)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return state, err
}

// GetContentEntryHash returns the content hash for a path, or 0 if not indexed.
// Used for change detection during incremental updates.
func (d *FileSearchDB) GetContentEntryHash(ctx context.Context, path string) (uint32, error) {
	if d == nil || d.db == nil {
		return 0, fmt.Errorf("filesearch db not open")
	}
	var hash int64
	err := d.db.QueryRowContext(ctx, `SELECT content_hash FROM content_entries WHERE path = ?`, path).Scan(&hash)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return uint32(hash), err
}

// ListContentEntryPaths returns all indexed content paths. Used by the crawler
// to detect stale entries (paths in DB but no longer on disk).
func (d *FileSearchDB) ListContentEntryPaths(ctx context.Context) ([]string, error) {
	if d == nil || d.db == nil {
		return nil, fmt.Errorf("filesearch db not open")
	}
	rows, err := d.db.QueryContext(ctx, `SELECT path FROM content_entries ORDER BY path ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var paths []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		paths = append(paths, p)
	}
	return paths, rows.Err()
}

// ListContentEntryPathsUnderScope returns all indexed content paths that fall
// under the given directory scope (the scope path itself plus any path with it
// as a parent prefix). Used by the content hook to reconcile content entries
// after a scanner scope replacement.
func (d *FileSearchDB) ListContentEntryPathsUnderScope(ctx context.Context, scopePath string) ([]string, error) {
	if d == nil || d.db == nil {
		return nil, fmt.Errorf("filesearch db not open")
	}
	// Match the scope path exactly and any path beneath it (scopePath + "/" or "\").
	// SQLite LIKE with escaped separator handles both separators because paths
	// are stored with os-specific separators via filepath.Clean.
	likePattern := scopePath + string(filepath.Separator) + "%"
	rows, err := d.db.QueryContext(ctx,
		`SELECT path FROM content_entries WHERE path = ? OR path LIKE ? ORDER BY path ASC`,
		scopePath, likePattern)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var paths []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		paths = append(paths, p)
	}
	return paths, rows.Err()
}

// EntryPathExists reports whether a path is currently present in the name
// index entries table. Used by the content hook to decide whether a content
// entry should be deleted during scope reconciliation.
func (d *FileSearchDB) EntryPathExists(ctx context.Context, path string) (bool, error) {
	if d == nil || d.db == nil {
		return false, fmt.Errorf("filesearch db not open")
	}
	var exists int
	err := d.db.QueryRowContext(ctx, `SELECT 1 FROM entries WHERE path = ? LIMIT 1`, path).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// quoteFTS5Match wraps a tokenized query string in double quotes for FTS5
// MATCH, so tokens are treated as literal terms (no FTS5 syntax interpretation).
// Each token is individually quoted and joined with implicit AND (space).
func quoteFTS5Match(tokenized string) string {
	tokens := strings.Fields(tokenized)
	if len(tokens) == 0 {
		return `""`
	}
	// Sort tokens for consistent query regardless of input order. FTS5 AND
	// is order-independent.
	sort.Strings(tokens)
	quoted := make([]string, len(tokens))
	for i, t := range tokens {
		// Double-quote each token to avoid FTS5 operator interpretation.
		quoted[i] = `"` + strings.ReplaceAll(t, `"`, `""`) + `"`
	}
	return strings.Join(quoted, " ")
}

// fnv32aContent computes FNV-32a hash of text for content change detection.
func fnv32aContent(text string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(text))
	return h.Sum32()
}
