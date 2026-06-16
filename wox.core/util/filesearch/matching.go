package filesearch

import (
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"wox/util"
)

type wildcardQuery struct {
	expression       *regexp.Regexp
	hasPathSeparator bool
	literalCount     int
}

func normalizeQuery(raw string) string {
	return strings.TrimSpace(raw)
}

func normalizeSearchQuery(query SearchQuery) SearchQuery {
	query.Raw = normalizeQuery(query.Raw)
	query.wildcard = buildWildcardQuery(query.Raw)
	query.plan = buildQueryPlan(query)
	return query
}

func normalizePath(path string) string {
	cleaned := filepath.Clean(strings.TrimSpace(path))
	if util.IsWindows() {
		return strings.ToLower(cleaned)
	}
	return cleaned
}

func buildSearchTerms(name string, path string, pinyinFull string, pinyinInitials string, usePinyin bool) []string {
	terms := []string{name, path}
	// Linear fallback searches share the same SearchQuery contract as SQLite.
	// Keep pinyin terms out completely when the caller disabled pinyin so broad
	// ASCII fragments from generated pinyin payloads cannot pass final scoring.
	if usePinyin {
		terms = append(terms, pinyinFull, pinyinInitials)
	}
	return util.UniqueStrings(filterNonEmpty(terms))
}

func matchSearchQuery(query SearchQuery, name string, path string, pinyinFull string, pinyinInitials string) (bool, int64) {
	if query.Raw == "" {
		return false, 0
	}
	if query.wildcard != nil {
		return query.wildcard.match(name, path)
	}
	if query.plan != nil && query.plan.pathLike {
		return scorePathMatch(path, query.plan.pathQuery)
	}
	usePinyin := !query.DisablePinyin
	return scoreSearchTerms(query.Raw, buildSearchTerms(name, path, pinyinFull, pinyinInitials, usePinyin), usePinyin)
}

func scoreSearchTerms(query string, terms []string, usePinyin bool) (bool, int64) {
	bestScore := int64(0)
	matched := false

	for _, term := range terms {
		isMatch, score := util.IsStringMatchScore(term, query, usePinyin)
		if !isMatch {
			continue
		}

		if !matched || score > bestScore {
			matched = true
			bestScore = score
		}
	}

	return matched, bestScore
}

func compareSearchResults(a SearchResult, b SearchResult) int {
	switch {
	case a.Score > b.Score:
		return -1
	case a.Score < b.Score:
		return 1
	case a.IsDir && !b.IsDir:
		return -1
	case !a.IsDir && b.IsDir:
		return 1
	case a.Name < b.Name:
		return -1
	case a.Name > b.Name:
		return 1
	case a.Path < b.Path:
		return -1
	case a.Path > b.Path:
		return 1
	default:
		return 0
	}
}

func sortAndLimitResults(results []SearchResult, limit int) []SearchResult {
	sort.Slice(results, func(i, j int) bool {
		return compareSearchResults(results[i], results[j]) < 0
	})

	if limit > 0 && len(results) > limit {
		return append([]SearchResult(nil), results[:limit]...)
	}

	return append([]SearchResult(nil), results...)
}

func filterNonEmpty(values []string) []string {
	filtered := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		filtered = append(filtered, value)
	}
	return filtered
}

func buildWildcardQuery(raw string) *wildcardQuery {
	if !strings.Contains(raw, "*") {
		return nil
	}

	pattern := filepath.ToSlash(strings.TrimSpace(raw))
	if pattern == "" {
		return nil
	}

	quoted := regexp.QuoteMeta(pattern)
	quoted = strings.ReplaceAll(quoted, "\\*", ".*")

	if strings.Contains(pattern, "/") && !isRootedWildcardPattern(pattern) {
		quoted = "(?:.*/)?" + quoted
	}

	expression, err := regexp.Compile("(?i)^" + quoted + "$")
	if err != nil {
		return nil
	}

	return &wildcardQuery{
		expression:       expression,
		hasPathSeparator: strings.Contains(pattern, "/"),
		literalCount:     len(strings.ReplaceAll(pattern, "*", "")),
	}
}

func isRootedWildcardPattern(pattern string) bool {
	if strings.HasPrefix(pattern, "/") {
		return true
	}
	return filepath.VolumeName(filepath.FromSlash(pattern)) != ""
}

func (q *wildcardQuery) match(name string, fullPath string) (bool, int64) {
	if q == nil || q.expression == nil {
		return false, 0
	}

	target := name
	if q.hasPathSeparator {
		target = path.Clean(filepath.ToSlash(fullPath))
	}

	if !q.expression.MatchString(target) {
		return false, 0
	}

	return true, int64(q.literalCount*1000 - len(target))
}

func scorePathMatch(path string, query string) (bool, int64) {
	normalizedPath := normalizePathQuery(path)
	normalizedQuery := normalizePathQuery(query)
	if normalizedPath == "" || normalizedQuery == "" {
		return false, 0
	}

	if strings.Contains(normalizedPath, normalizedQuery) {
		return true, int64(8000 + utf8Len(normalizedQuery)*100 - utf8Len(normalizedPath))
	}

	matched, score := util.IsStringMatchScore(normalizedPath, normalizedQuery, false)
	if !matched {
		return false, 0
	}

	return true, score + 2000
}
