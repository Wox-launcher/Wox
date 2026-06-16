package filesearch

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"wox/util"
)

const sqliteFTSRepairTimeout = 2 * time.Minute

type SQLiteSearchProvider struct {
	db *FileSearchDB
	// Stale FTS repair runs asynchronously after a fallback query returns the
	// current keystroke result; this guard keeps repeated broken FTS reads from
	// starting overlapping full-table rebuilds.
	ftsRepairMu      sync.Mutex
	ftsRepairRunning bool
}

func NewSQLiteSearchProvider(db *FileSearchDB) *SQLiteSearchProvider {
	return &SQLiteSearchProvider{db: db}
}

func (p *SQLiteSearchProvider) Name() string {
	return "sqlite-search"
}

func (p *SQLiteSearchProvider) Search(ctx context.Context, query SearchQuery, limit int) ([]SearchResult, error) {
	query = normalizeSearchQuery(query)
	if p == nil || p.db == nil || strings.TrimSpace(query.Raw) == "" {
		return nil, nil
	}

	candidateLimit := defaultPreRerankLimit
	if query.plan != nil && query.plan.preRerankLimit > 0 {
		candidateLimit = query.plan.preRerankLimit
	}
	if limit > 0 && candidateLimit < limit {
		candidateLimit = limit
	}

	candidateIDs, err := p.collectCandidateIDs(ctx, query, candidateLimit)
	if err != nil {
		return nil, err
	}
	if len(candidateIDs) == 0 {
		return nil, nil
	}

	rows, err := p.listEntriesByIDs(ctx, candidateIDs)
	if err != nil {
		return nil, err
	}

	results := make([]SearchResult, 0, len(rows))
	for _, row := range rows {
		record := docRecord{
			Path:           row.Path,
			IsDir:          row.IsDir,
			PinyinFull:     row.PinyinFull,
			PinyinInitials: row.PinyinInitials,
		}
		matched, score := scoreDocAgainstQuery(query, record)
		if !matched {
			continue
		}
		results = append(results, SearchResult{
			Path:       row.Path,
			Name:       row.Name,
			ParentPath: row.ParentPath,
			IsDir:      row.IsDir,
			Mtime:      row.Mtime,
			Size:       row.Size,
			Score:      score,
		})
	}

	return sortAndLimitResults(results, limit), nil
}

func (p *SQLiteSearchProvider) collectCandidateIDs(ctx context.Context, query SearchQuery, limit int) ([]int64, error) {
	if query.plan == nil {
		return nil, nil
	}

	plan := query.plan
	if plan.extensionOnly {
		return p.queryIDsByExtension(ctx, plan.extension, limit)
	}

	if query.wildcard != nil {
		return p.collectWildcardCandidateIDs(ctx, query, limit)
	}

	switch plan.shortQueryLength {
	case 1:
		return p.collectOneCharacterCandidateIDs(ctx, query, limit)
	case 2:
		return p.collectTwoCharacterCandidateIDs(ctx, query, limit)
	default:
		return p.collectGeneralCandidateIDs(ctx, query, limit)
	}
}

func (p *SQLiteSearchProvider) collectOneCharacterCandidateIDs(ctx context.Context, query SearchQuery, limit int) ([]int64, error) {
	plan := query.plan
	if plan == nil || len(plan.rawLettersDigits) != 1 || plan.pathLike {
		return nil, nil
	}

	prefix := plan.rawLettersDigits
	return p.queryIDs(ctx, `
		SELECT entry_id
		FROM entries
		WHERE name_key >= ? AND name_key < ?
		ORDER BY name_key ASC, entry_id ASC
		LIMIT ?
	`, prefix, nextPrefixUpperBound(prefix), limit)
}

func (p *SQLiteSearchProvider) collectTwoCharacterCandidateIDs(ctx context.Context, query SearchQuery, limit int) ([]int64, error) {
	plan := query.plan
	if plan == nil {
		return nil, nil
	}

	if plan.pathLike {
		return p.queryPathFallbackIDs(ctx, plan.pathQuery, limit)
	}

	if !plan.asciiLettersDigits || len(plan.rawLettersDigits) != 2 {
		return nil, nil
	}

	// Two-character substring matching produced very broad recall and forced the
	// indexer to maintain the expensive bigram side table. Tightening short
	// queries to the same indexed name-key prefix path keeps response time fast
	// while making the reduced recall explicit and predictable.
	prefix := plan.rawLettersDigits
	return p.queryIDs(ctx, `
		SELECT entry_id
		FROM entries
		WHERE name_key >= ? AND name_key < ?
		ORDER BY name_key ASC, entry_id ASC
		LIMIT ?
	`, prefix, nextPrefixUpperBound(prefix), limit)
}

func (p *SQLiteSearchProvider) collectGeneralCandidateIDs(ctx context.Context, query SearchQuery, limit int) ([]int64, error) {
	plan := query.plan
	if plan == nil {
		return nil, nil
	}

	if plan.pathLike {
		return p.queryPathFTSIDs(ctx, plan, limit)
	}

	ids := make([]int64, 0, limit)
	nameIDs, err := p.queryFTSLiteralContainsIDs(ctx, "entries_name_fts", "normalized_name", plan.nameTerm, limit)
	if err != nil {
		return nil, err
	}
	ids = append(ids, nameIDs...)

	if plan.asciiLettersDigits && len(plan.rawLettersDigits) >= 3 {
		// Optimization: the indexer no longer stores ASCII-only filenames in the
		// pinyin FTS tables, because that doubled derived-index maintenance for
		// normal code trees. name_key is the cheap punctuation-insensitive recall
		// path for queries like "maingo" against "main.go" without reintroducing
		// the old ASCII pinyin payload.
		nameKeyIDs, err := p.queryNameKeyPrefixIDs(ctx, plan.rawLettersDigits, limit)
		if err != nil {
			return nil, err
		}
		ids = append(ids, nameKeyIDs...)
	}

	// The indexed SQLite provider does not go through the generic plugin fuzzy
	// matcher, so it must also honor SearchQuery.DisablePinyin before touching
	// pinyin FTS tables. Otherwise disabling pinyin only affected non-file
	// result filtering while filesearch still recalled pinyin-derived matches.
	if plan.usePinyin && plan.asciiLettersDigits && len(plan.rawLettersDigits) >= 3 {
		pinyinFullIDs, err := p.queryFTSLiteralContainsIDs(ctx, "entries_pinyin_full_fts", "pinyin_full", plan.rawLettersDigits, limit)
		if err != nil {
			return nil, err
		}
		ids = append(ids, pinyinFullIDs...)

		initialsIDs, err := p.queryFTSMatchIDs(ctx, "entries_initials_fts", plan.rawLettersDigits+"*", limit)
		if err != nil {
			return nil, err
		}
		ids = append(ids, initialsIDs...)
	}

	if plan.extension != "" {
		extensionIDs, err := p.queryIDsByExtension(ctx, plan.extension, limit)
		if err != nil {
			return nil, err
		}
		ids = append(ids, extensionIDs...)
	}

	return trimCandidateIDs(ids, limit), nil
}

func (p *SQLiteSearchProvider) queryNameKeyPrefixIDs(ctx context.Context, prefix string, limit int) ([]int64, error) {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return nil, nil
	}
	return p.queryIDs(ctx, `
		SELECT entry_id
		FROM entries
		WHERE name_key >= ? AND name_key < ?
		ORDER BY name_key ASC, entry_id ASC
		LIMIT ?
	`, prefix, nextPrefixUpperBound(prefix), limit)
}

func (p *SQLiteSearchProvider) collectWildcardCandidateIDs(ctx context.Context, query SearchQuery, limit int) ([]int64, error) {
	plan := query.plan
	if plan == nil {
		return nil, nil
	}

	targetTable := "entries_name_fts"
	targetColumn := "normalized_name"
	literal := wildcardRecallLiteral(query.Raw, false)
	if plan.pathLike {
		targetTable = "entries_path_fts"
		targetColumn = "normalized_path"
		literal = wildcardRecallLiteral(query.Raw, true)
	}

	if utf8LenString(literal) < 3 {
		if plan.pathLike {
			return p.queryPathWildcardFallbackIDs(ctx, wildcardRecallLikePattern(plan.pathQuery), limit)
		}
		if plan.extension != "" {
			return p.queryIDsByExtension(ctx, plan.extension, limit)
		}
		return p.queryNameWildcardFallbackIDs(ctx, wildcardRecallLikePattern(plan.rawLower), limit)
	}

	return p.queryFTSLiteralContainsIDs(ctx, targetTable, targetColumn, literal, limit)
}

func (p *SQLiteSearchProvider) queryPathFTSIDs(ctx context.Context, plan *queryPlan, limit int) ([]int64, error) {
	if plan == nil {
		return nil, nil
	}

	segments := plan.pathSegments
	if len(segments) == 0 && strings.TrimSpace(plan.pathQuery) != "" {
		segments = []string{plan.pathQuery}
	}
	if len(segments) == 0 {
		return nil, nil
	}

	var intersected []int64
	for _, segment := range segments {
		if strings.TrimSpace(segment) == "" {
			continue
		}

		var ids []int64
		var err error
		if utf8LenString(segment) >= 3 {
			ids, err = p.queryFTSLiteralContainsIDs(ctx, "entries_path_fts", "normalized_path", segment, plan.perClauseLimit)
		} else {
			ids, err = p.queryPathFallbackIDs(ctx, segment, plan.perClauseLimit)
		}
		if err != nil {
			return nil, err
		}
		if len(ids) == 0 {
			return nil, nil
		}

		if intersected == nil {
			intersected = ids
			continue
		}
		intersected = intersectInt64(intersected, ids, limit)
		if len(intersected) == 0 {
			return nil, nil
		}
	}

	if len(plan.pathQuery) >= 3 {
		fullPathIDs, err := p.queryFTSLiteralContainsIDs(ctx, "entries_path_fts", "normalized_path", plan.pathQuery, limit)
		if err != nil {
			return nil, err
		}
		intersected = append(intersected, fullPathIDs...)
	}

	return trimCandidateIDs(intersected, limit), nil
}

func (p *SQLiteSearchProvider) queryIDsByExtension(ctx context.Context, extension string, limit int) ([]int64, error) {
	if strings.TrimSpace(extension) == "" {
		return nil, nil
	}
	return p.queryIDs(ctx, `
		SELECT entry_id
		FROM entries
		WHERE extension = ?
		ORDER BY entry_id ASC
		LIMIT ?
	`, extension, limit)
}

func (p *SQLiteSearchProvider) queryNameFallbackIDs(ctx context.Context, term string, limit int) ([]int64, error) {
	term = strings.TrimSpace(term)
	if term == "" {
		return nil, nil
	}
	return p.queryIDs(ctx, `
		SELECT entry_id
		FROM entries
		WHERE normalized_name LIKE ? ESCAPE '\'
		ORDER BY entry_id ASC
		LIMIT ?
	`, "%"+escapeLikePattern(term)+"%", limit)
}

func (p *SQLiteSearchProvider) queryPathFallbackIDs(ctx context.Context, term string, limit int) ([]int64, error) {
	term = strings.TrimSpace(term)
	if term == "" {
		return nil, nil
	}
	return p.queryIDs(ctx, `
		SELECT entry_id
		FROM entries
		WHERE normalized_path LIKE ? ESCAPE '\'
		ORDER BY entry_id ASC
		LIMIT ?
	`, "%"+escapeLikePattern(term)+"%", limit)
}

func (p *SQLiteSearchProvider) queryNameWildcardFallbackIDs(ctx context.Context, pattern string, limit int) ([]int64, error) {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return nil, nil
	}
	return p.queryIDs(ctx, `
		SELECT entry_id
		FROM entries
		WHERE normalized_name LIKE ?
		ORDER BY entry_id ASC
		LIMIT ?
	`, pattern, limit)
}

func (p *SQLiteSearchProvider) queryPathWildcardFallbackIDs(ctx context.Context, pattern string, limit int) ([]int64, error) {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return nil, nil
	}
	return p.queryIDs(ctx, `
		SELECT entry_id
		FROM entries
		WHERE normalized_path LIKE ?
		ORDER BY entry_id ASC
		LIMIT ?
	`, pattern, limit)
}

func (p *SQLiteSearchProvider) queryFTSLiteralContainsIDs(ctx context.Context, tableName string, columnName string, term string, limit int) ([]int64, error) {
	term = strings.TrimSpace(term)
	if term == "" {
		return nil, nil
	}

	// FTS5 trigram can accelerate LIKE/GLOB contains queries, but SQLite disables
	// that optimization when ESCAPE is present. File search only treats '*' as a
	// wildcard at the query language layer, so other LIKE metacharacters may
	// over-recall here and the final scorer/wildcard matcher filters the rows.
	return p.queryFTSLikeIDs(ctx, tableName, columnName, "%"+term+"%", limit)
}

func (p *SQLiteSearchProvider) queryFTSLikeIDs(ctx context.Context, tableName string, columnName string, pattern string, limit int) ([]int64, error) {
	ids, err := p.queryIDs(ctx, fmt.Sprintf(`
		SELECT rowid
		FROM %s
		WHERE %s LIKE ?
		LIMIT ?
	`, tableName, columnName), pattern, limit)
	if !isMissingFTSContentRowError(err) {
		return ids, err
	}

	p.scheduleFTSRepair(ctx, tableName, err)
	fallbackIDs, fallbackErr := p.queryFTSLikeFallbackIDs(ctx, tableName, pattern, limit)
	if fallbackErr != nil {
		return nil, fmt.Errorf("fallback %s LIKE query after stale FTS row: %w; original query error: %v", tableName, fallbackErr, err)
	}
	return fallbackIDs, nil
}

func (p *SQLiteSearchProvider) queryFTSMatchIDs(ctx context.Context, tableName string, expression string, limit int) ([]int64, error) {
	ids, err := p.queryIDs(ctx, fmt.Sprintf(`
		SELECT rowid
		FROM %s
		WHERE %s MATCH ?
		LIMIT ?
	`, tableName, tableName), expression, limit)
	if !isMissingFTSContentRowError(err) {
		return ids, err
	}

	p.scheduleFTSRepair(ctx, tableName, err)
	fallbackIDs, fallbackErr := p.queryFTSMatchFallbackIDs(ctx, tableName, expression, limit)
	if fallbackErr != nil {
		return nil, fmt.Errorf("fallback %s MATCH query after stale FTS row: %w; original query error: %v", tableName, fallbackErr, err)
	}
	return fallbackIDs, nil
}

func (p *SQLiteSearchProvider) queryFTSLikeFallbackIDs(ctx context.Context, tableName string, pattern string, limit int) ([]int64, error) {
	columnName, ok := ftsContentColumn(tableName)
	if !ok {
		return nil, fmt.Errorf("unsupported FTS fallback table %q", tableName)
	}
	return p.queryIDs(ctx, fmt.Sprintf(`
		SELECT entry_id
		FROM entries
		WHERE %s LIKE ?
		ORDER BY entry_id ASC
		LIMIT ?
	`, columnName), pattern, limit)
}

func (p *SQLiteSearchProvider) queryFTSMatchFallbackIDs(ctx context.Context, tableName string, expression string, limit int) ([]int64, error) {
	if tableName != "entries_initials_fts" {
		return nil, fmt.Errorf("unsupported FTS MATCH fallback table %q", tableName)
	}

	// MATCH is currently only used for prefix searches against pinyin initials.
	// A bounded range scan preserves that behavior without asking the broken FTS
	// table to resolve rowids that may no longer exist in the content table.
	prefix := strings.TrimSuffix(strings.TrimSpace(expression), "*")
	if prefix == "" {
		return nil, nil
	}
	return p.queryIDs(ctx, `
		SELECT entry_id
		FROM entries
		WHERE pinyin_initials >= ? AND pinyin_initials < ?
		ORDER BY pinyin_initials ASC, entry_id ASC
		LIMIT ?
	`, prefix, nextPrefixUpperBound(prefix), limit)
}

func ftsContentColumn(tableName string) (string, bool) {
	switch tableName {
	case "entries_name_fts":
		return "normalized_name", true
	case "entries_path_fts":
		return "normalized_path", true
	case "entries_pinyin_full_fts":
		return "pinyin_full", true
	case "entries_initials_fts":
		return "pinyin_initials", true
	default:
		return "", false
	}
}

func (p *SQLiteSearchProvider) scheduleFTSRepair(ctx context.Context, tableName string, cause error) {
	if p == nil || p.db == nil {
		return
	}

	p.ftsRepairMu.Lock()
	if p.ftsRepairRunning {
		p.ftsRepairMu.Unlock()
		return
	}
	p.ftsRepairRunning = true
	p.ftsRepairMu.Unlock()

	// Stale external-content FTS rows can make SQLite spend longer than the UI's
	// query wait budget rebuilding derived data. Return fallback candidates from
	// the entries table immediately, then rebuild FTS once in the background so
	// later queries regain the fast path without turning this keystroke into an
	// empty timed-out result.
	util.GetLogger().Warn(ctx, fmt.Sprintf("filesearch detected stale %s content row, scheduling FTS rebuild: %v", tableName, cause))
	util.Go(ctx, "filesearch stale fts repair", func() {
		defer func() {
			p.ftsRepairMu.Lock()
			p.ftsRepairRunning = false
			p.ftsRepairMu.Unlock()
		}()

		repairCtx, cancel := context.WithTimeout(util.NewTraceContext(), sqliteFTSRepairTimeout)
		defer cancel()
		if err := p.db.rebuildFTSTables(repairCtx, false); err != nil {
			util.GetLogger().Error(repairCtx, fmt.Sprintf("filesearch stale FTS rebuild failed for %s: %v", tableName, err))
			return
		}
		util.GetLogger().Info(repairCtx, fmt.Sprintf("filesearch stale FTS rebuild completed after %s error", tableName))
	})
}

func isMissingFTSContentRowError(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	return strings.Contains(message, "fts5: missing row") && strings.Contains(message, "content table")
}

func (p *SQLiteSearchProvider) queryIDs(ctx context.Context, query string, args ...any) ([]int64, error) {
	rows, err := p.db.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var entryID int64
		if err := rows.Scan(&entryID); err != nil {
			return nil, err
		}
		ids = append(ids, entryID)
	}
	return ids, rows.Err()
}

func (p *SQLiteSearchProvider) listEntriesByIDs(ctx context.Context, entryIDs []int64) ([]storedEntryRecord, error) {
	entryIDs = trimCandidateIDs(entryIDs, 0)
	if len(entryIDs) == 0 {
		return nil, nil
	}

	placeholders := make([]string, 0, len(entryIDs))
	args := make([]any, 0, len(entryIDs))
	for _, entryID := range entryIDs {
		placeholders = append(placeholders, "?")
		args = append(args, entryID)
	}

	rows, err := p.db.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT entry_id, path, root_id, parent_path, name, normalized_name, name_key, normalized_path,
		       pinyin_full, pinyin_initials, extension, is_dir, mtime, size, updated_at
		FROM entries
		WHERE entry_id IN (%s)
	`, strings.Join(placeholders, ", ")), args...)
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
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return loaded, nil
}

func wildcardRecallLiteral(raw string, pathLike bool) string {
	if !strings.Contains(raw, "*") {
		return ""
	}

	parts := strings.Split(raw, "*")
	longest := ""
	for _, part := range parts {
		if pathLike {
			part = normalizePathQuery(part)
		} else {
			part = normalizeIndexText(part)
		}
		part = strings.TrimSpace(part)
		if utf8LenString(part) > utf8LenString(longest) {
			longest = part
		}
	}
	return longest
}

func wildcardRecallLikePattern(term string) string {
	term = strings.TrimSpace(term)
	if term == "" {
		return ""
	}

	// This is only a broad candidate-recall query for '*' wildcard searches.
	// Do not escape other LIKE metacharacters here: the search language does not
	// promise exact '%' or '_' handling, and the final wildcard matcher enforces
	// the real '*' semantics before results are returned.
	return "%" + strings.ReplaceAll(term, "*", "%") + "%"
}

func intersectInt64(left []int64, right []int64, limit int) []int64 {
	if len(left) == 0 || len(right) == 0 {
		return nil
	}

	rightSet := make(map[int64]struct{}, len(right))
	for _, value := range right {
		rightSet[value] = struct{}{}
	}

	intersection := make([]int64, 0, min(len(left), len(right)))
	for _, value := range left {
		if _, ok := rightSet[value]; !ok {
			continue
		}
		intersection = append(intersection, value)
		if limit > 0 && len(intersection) >= limit {
			break
		}
	}
	return intersection
}
