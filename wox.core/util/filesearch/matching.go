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
	asciiSuffix      string
}

type quotedSearchQuery struct {
	unquoted string
	phrases  []string
}

func normalizeQuery(raw string) string {
	return strings.TrimSpace(raw)
}

func normalizeSearchQuery(query SearchQuery) SearchQuery {
	query.Raw = normalizeQuery(query.Raw)
	query.wildcard = buildWildcardQuery(parseQuotedSearchQuery(query.Raw).unquoted)
	query.plan = buildQueryPlan(query)
	return query
}

func parseQuotedSearchQuery(query string) quotedSearchQuery {
	var unquoted strings.Builder
	var quoted strings.Builder
	inQuote := false
	var phrases []string

	for _, r := range query {
		if r == '"' {
			if inQuote {
				if phrase := strings.TrimSpace(quoted.String()); phrase != "" {
					phrases = append(phrases, phrase)
				}
				quoted.Reset()
				inQuote = false
			} else {
				inQuote = true
			}
			continue
		}
		if inQuote {
			quoted.WriteRune(r)
		} else {
			unquoted.WriteRune(r)
		}
	}

	if inQuote {
		unquoted.WriteRune('"')
		unquoted.WriteString(quoted.String())
	}

	return quotedSearchQuery{
		unquoted: strings.TrimSpace(unquoted.String()),
		phrases:  phrases,
	}
}

func normalizePath(path string) string {
	cleaned := filepath.Clean(strings.TrimSpace(path))
	if util.IsWindows() {
		return strings.ToLower(cleaned)
	}
	return cleaned
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
	case len(a.Name) < len(b.Name):
		return -1
	case len(a.Name) > len(b.Name):
		return 1
	case a.Name < b.Name:
		return -1
	case a.Name > b.Name:
		return 1
	case len(a.Path) < len(b.Path):
		return -1
	case len(a.Path) > len(b.Path):
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

	suffix := ""
	if strings.HasPrefix(pattern, "*") && strings.Count(pattern, "*") == 1 && !strings.Contains(pattern, "/") {
		suffix = pattern[1:]
		for _, c := range suffix {
			if c >= 128 {
				suffix = ""
				break
			}
		}
	}
	return &wildcardQuery{
		expression:       expression,
		asciiSuffix:      suffix,
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
