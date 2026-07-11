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

// ContentEntryMetadata is the persisted file identity used to skip unchanged content during a full crawl.
type ContentEntryMetadata struct {
	Mtime     int64
	Size      int64
	Extension string
}

// ContentIndexCandidate is one authoritative file candidate read from the name index.
type ContentIndexCandidate struct {
	EntryID   int64
	Path      string
	Mtime     int64
	Size      int64
	Extension string
}

type contentIndexDocument struct {
	Path      string
	Mtime     int64
	Size      int64
	Extension string
	Text      string
}

type preparedContentIndexDocument struct {
	contentIndexDocument
	Hash         uint32
	Tokenized    string
	IndexedBytes int64
}

// contentCrawlStateKey is the meta key for content crawl state.
const contentCrawlStateKey = "content_crawl_state"

const (
	contentIndexSchemaVersionKey     = "content_index_schema_version"
	currentContentIndexSchemaVersion = "3"
)

// IndexContent indexes or updates a file's content in the content index.
// It tokenizes the text, computes a content hash for change detection, and
// inserts into content_entries + entries_content_fts. If the path already
// exists with the same hash, it skips (no change). Returns true if the index
// was actually updated. Retries on "database is locked" up to 3 times.
func (d *ContentSearchDB) IndexContent(ctx context.Context, path string, mtime, size int64, extension, text string) (bool, error) {
	updated, err := d.indexContentBatch(ctx, []contentIndexDocument{{
		Path:      path,
		Mtime:     mtime,
		Size:      size,
		Extension: extension,
		Text:      text,
	}})
	return updated > 0, err
}

// indexContentBatch tokenizes documents before entering one SQLite transaction and returns the number of changed rows.
func (d *ContentSearchDB) indexContentBatch(ctx context.Context, documents []contentIndexDocument) (int, error) {
	if d == nil || d.db == nil {
		return 0, fmt.Errorf("content search db not open")
	}
	if len(documents) == 0 {
		return 0, nil
	}

	prepared := make([]preparedContentIndexDocument, 0, len(documents))
	for _, document := range documents {
		prepared = append(prepared, preparedContentIndexDocument{
			contentIndexDocument: document,
			Hash:                 fnv32aContent(document.Text),
			Tokenized:            TokenizeForContentIndex(document.Text),
			IndexedBytes:         int64(len(document.Text)),
		})
	}

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		updated, err := d.indexContentBatchOnce(ctx, prepared)
		if err == nil {
			return updated, nil
		}
		lastErr = err
		// Retry only on "database is locked" or "SQLITE_BUSY".
		if !strings.Contains(err.Error(), "locked") && !strings.Contains(err.Error(), "busy") {
			return 0, err
		}
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(time.Duration(100*(attempt+1)) * time.Millisecond):
		}
	}
	return 0, lastErr
}

// indexContentBatchOnce applies one prepared batch atomically without retrying inside the transaction.
func (d *ContentSearchDB) indexContentBatchOnce(ctx context.Context, documents []preparedContentIndexDocument) (int, error) {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	updatedCount := 0
	for _, document := range documents {
		var existingRowid int64
		var existingHash int64
		var existingMtime, existingSize int64
		var existingExtension string
		err = tx.QueryRowContext(ctx, `SELECT rowid, content_hash, mtime, size, extension FROM content_entries WHERE path = ?`, document.Path).Scan(&existingRowid, &existingHash, &existingMtime, &existingSize, &existingExtension)
		if err != nil && err != sql.ErrNoRows {
			return 0, fmt.Errorf("query existing content entry %s: %w", document.Path, err)
		}
		if err == nil && uint32(existingHash) == document.Hash {
			if existingMtime != document.Mtime || existingSize != document.Size || existingExtension != document.Extension {
				if _, err := tx.ExecContext(ctx, `UPDATE content_entries SET mtime = ?, size = ?, extension = ?, indexed_text_bytes = ? WHERE rowid = ?`, document.Mtime, document.Size, document.Extension, document.IndexedBytes, existingRowid); err != nil {
					return 0, fmt.Errorf("refresh unchanged content metadata: %w", err)
				}
			}
			continue
		}

		if err == nil && existingRowid > 0 {
			if _, err := tx.ExecContext(ctx, `INSERT INTO entries_content_fts(entries_content_fts, rowid) VALUES('delete', ?)`, existingRowid); err != nil {
				return 0, fmt.Errorf("delete old content fts: %w", err)
			}
			if _, err := tx.ExecContext(ctx, `DELETE FROM content_entries WHERE rowid = ?`, existingRowid); err != nil {
				return 0, fmt.Errorf("delete old content entry: %w", err)
			}
		}

		res, err := tx.ExecContext(ctx,
			`INSERT INTO content_entries (path, mtime, size, content_hash, extension, indexed_text_bytes) VALUES (?, ?, ?, ?, ?, ?)`,
			document.Path, document.Mtime, document.Size, int64(document.Hash), document.Extension, document.IndexedBytes)
		if err != nil {
			return 0, fmt.Errorf("insert content entry: %w", err)
		}
		newRowid, err := res.LastInsertId()
		if err != nil {
			return 0, fmt.Errorf("get last insert id: %w", err)
		}
		if document.Tokenized != "" {
			if _, err := tx.ExecContext(ctx, `INSERT INTO entries_content_fts(rowid, content) VALUES (?, ?)`, newRowid, document.Tokenized); err != nil {
				return 0, fmt.Errorf("insert content fts: %w", err)
			}
		}
		updatedCount++
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}
	return updatedCount, nil
}

// ListContentEntryMetadata loads the small metadata working set used by one full content crawl.
func (d *ContentSearchDB) ListContentEntryMetadata(ctx context.Context) (map[string]ContentEntryMetadata, error) {
	if d == nil || d.db == nil {
		return nil, fmt.Errorf("content search db not open")
	}
	rows, err := d.db.QueryContext(ctx, `SELECT path, mtime, size, extension FROM content_entries`)
	if err != nil {
		return nil, fmt.Errorf("list content entry metadata: %w", err)
	}
	defer rows.Close()

	metadata := make(map[string]ContentEntryMetadata)
	for rows.Next() {
		var path string
		var entry ContentEntryMetadata
		if err := rows.Scan(&path, &entry.Mtime, &entry.Size, &entry.Extension); err != nil {
			return nil, err
		}
		metadata[path] = entry
	}
	return metadata, rows.Err()
}

// ListContentIndexCandidates reads one stable keyset page of eligible files from the name index.
func (d *FileSearchDB) ListContentIndexCandidates(ctx context.Context, afterEntryID int64, extensions map[string]bool, limit int) ([]ContentIndexCandidate, error) {
	if d == nil || d.db == nil {
		return nil, fmt.Errorf("filesearch db not open")
	}
	if limit <= 0 || len(extensions) == 0 {
		return nil, nil
	}

	extensionList := make([]string, 0, len(extensions))
	for extension, enabled := range extensions {
		if enabled {
			extensionList = append(extensionList, extension)
		}
	}
	if len(extensionList) == 0 {
		return nil, nil
	}
	sort.Strings(extensionList)

	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(extensionList)), ",")
	args := make([]any, 0, len(extensionList)+2)
	args = append(args, afterEntryID)
	for _, extension := range extensionList {
		args = append(args, extension)
	}
	args = append(args, limit)

	rows, err := d.db.QueryContext(ctx, `
		SELECT entry_id, path, mtime, size, extension
		FROM entries
		WHERE entry_id > ? AND is_dir = 0 AND extension IN (`+placeholders+`)
		ORDER BY entry_id ASC
		LIMIT ?
	`, args...)
	if err != nil {
		return nil, fmt.Errorf("list content index candidates: %w", err)
	}
	defer rows.Close()

	candidates := make([]ContentIndexCandidate, 0, limit)
	for rows.Next() {
		var candidate ContentIndexCandidate
		if err := rows.Scan(&candidate.EntryID, &candidate.Path, &candidate.Mtime, &candidate.Size, &candidate.Extension); err != nil {
			return nil, err
		}
		candidates = append(candidates, candidate)
	}
	return candidates, rows.Err()
}

// DeleteContent removes a file's content from the index.
func (d *ContentSearchDB) DeleteContent(ctx context.Context, path string) error {
	return d.DeleteContentBatch(ctx, []string{path})
}

// DeleteContentBatch removes multiple paths in one transaction.
func (d *ContentSearchDB) DeleteContentBatch(ctx context.Context, paths []string) error {
	if d == nil || d.db == nil {
		return fmt.Errorf("content search db not open")
	}
	if len(paths) == 0 {
		return nil
	}

	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	for _, path := range paths {
		var rowid int64
		err = tx.QueryRowContext(ctx, `SELECT rowid FROM content_entries WHERE path = ?`, path).Scan(&rowid)
		if err == sql.ErrNoRows {
			continue
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
	}

	return tx.Commit()
}

// ResetContentIndex clears content index rows while keeping the open database.
func (d *ContentSearchDB) ResetContentIndex(ctx context.Context) error {
	if d == nil || d.db == nil {
		return fmt.Errorf("content search db not open")
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

// SearchContent searches the content index for files matching all query terms
// and quoted phrases.
// Returns results sorted by FTS5 relevance (rank), limited to `limit`.
// Runs against the standalone content database so large content queries do not
// share SQLite cache or write pressure with name/path search.
func (d *ContentSearchDB) SearchContent(ctx context.Context, query string, limit int) ([]ContentSearchResult, error) {
	if d == nil || d.db == nil {
		return nil, fmt.Errorf("content search db not open")
	}
	if limit <= 0 {
		limit = 20
	}

	matchExpr := buildContentFTS5MatchExpression(query)
	if matchExpr == "" {
		return nil, nil
	}

	rows, err := d.db.QueryContext(ctx, `
		SELECT ce.path, bm25(entries_content_fts) AS rank
		FROM entries_content_fts f
		JOIN content_entries ce ON ce.rowid = f.rowid
		WHERE f.entries_content_fts MATCH ?
		ORDER BY rank
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
func (d *ContentSearchDB) ContentStats(ctx context.Context) (ContentStats, error) {
	if d == nil || d.db == nil {
		return ContentStats{}, fmt.Errorf("content search db not open")
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
func (d *ContentSearchDB) SetContentCrawlState(ctx context.Context, state string) error {
	if d == nil || d.db == nil {
		return fmt.Errorf("content search db not open")
	}
	_, err := d.db.ExecContext(ctx,
		`INSERT INTO meta(key, value) VALUES(?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		contentCrawlStateKey, state)
	return err
}

// GetContentCrawlState reads the content crawl state from the meta table.
func (d *ContentSearchDB) GetContentCrawlState(ctx context.Context) (string, error) {
	if d == nil || d.db == nil {
		return "", fmt.Errorf("content search db not open")
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
func (d *ContentSearchDB) GetContentEntryHash(ctx context.Context, path string) (uint32, error) {
	if d == nil || d.db == nil {
		return 0, fmt.Errorf("content search db not open")
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
func (d *ContentSearchDB) ListContentEntryPaths(ctx context.Context) ([]string, error) {
	if d == nil || d.db == nil {
		return nil, fmt.Errorf("content search db not open")
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
func (d *ContentSearchDB) ListContentEntryPathsUnderScope(ctx context.Context, scopePath string) ([]string, error) {
	if d == nil || d.db == nil {
		return nil, fmt.Errorf("content search db not open")
	}
	// Match the scope path exactly and any path beneath it (scopePath + "/" or "\").
	// SQLite LIKE with escaped separator handles both separators because paths
	// are stored with os-specific separators via filepath.Clean.
	cleanScope := filepath.Clean(scopePath)
	likePattern := escapeLikePattern(cleanScope+string(filepath.Separator)) + "%"
	rows, err := d.db.QueryContext(ctx,
		`SELECT path FROM content_entries WHERE path = ? OR path LIKE ? ESCAPE '\' ORDER BY path ASC`,
		cleanScope, likePattern)
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

// ListEntryPathsUnderScope returns indexed file paths for one authoritative name-index scope.
func (d *FileSearchDB) ListEntryPathsUnderScope(ctx context.Context, rootID, scopePath string) ([]string, error) {
	if d == nil || d.db == nil {
		return nil, fmt.Errorf("filesearch db not open")
	}
	cleanScope := filepath.Clean(scopePath)
	likePattern := escapeLikePattern(cleanScope+string(filepath.Separator)) + "%"
	rows, err := d.db.QueryContext(ctx, `
		SELECT path
		FROM entries
		WHERE root_id = ? AND is_dir = 0 AND (path = ? OR path LIKE ? ESCAPE '\')
		ORDER BY path ASC
	`, rootID, cleanScope, likePattern)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	paths := make([]string, 0)
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	return paths, rows.Err()
}

func buildContentFTS5MatchExpression(query string) string {
	parsed := parseQuotedSearchQuery(query)
	parts := make([]string, 0, len(parsed.phrases)+4)

	unquotedTokens := strings.Fields(TokenizeForContentIndex(parsed.unquoted))
	sort.Strings(unquotedTokens)
	for _, token := range unquotedTokens {
		parts = append(parts, quoteContentFTS5Phrase(token))
	}

	for _, phrase := range parsed.phrases {
		tokenized := TokenizeForContentIndex(phrase)
		if tokenized == "" {
			continue
		}
		parts = append(parts, quoteContentFTS5Phrase(tokenized))
	}

	return strings.Join(parts, " ")
}

func quoteContentFTS5Phrase(tokenized string) string {
	return `"` + strings.ReplaceAll(tokenized, `"`, `""`) + `"`
}

// fnv32aContent computes FNV-32a hash of text for content change detection.
func fnv32aContent(text string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(text))
	return h.Sum32()
}
