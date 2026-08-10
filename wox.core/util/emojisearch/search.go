package emojisearch

import "strings"

// Entry contains one emoji and its pre-normalized multilingual search terms.
type Entry struct {
	Emoji       string
	SearchTerms []string
}

// BuildTerms flattens translated names and categories into unique lowercase search terms.
func BuildTerms(groups ...map[string]string) []string {
	terms := make([]string, 0, len(groups)*2)
	for _, group := range groups {
		for _, value := range group {
			terms = append(terms, value)
		}
	}
	return NormalizeTerms(terms)
}

// NormalizeTerms removes empty and duplicate terms while preserving their first occurrence.
func NormalizeTerms(terms []string) []string {
	seen := make(map[string]struct{}, len(terms))
	normalized := make([]string, 0, len(terms))
	for _, term := range terms {
		term = strings.ToLower(strings.TrimSpace(term))
		if term == "" {
			continue
		}
		if _, exists := seen[term]; exists {
			continue
		}
		seen[term] = struct{}{}
		normalized = append(normalized, term)
	}
	return normalized
}

// Match reports whether an emoji character or multilingual term contains the query.
func Match(entry Entry, query string) bool {
	query = strings.ToLower(strings.TrimSpace(query))
	return matchNormalized(entry, query)
}

func matchNormalized(entry Entry, query string) bool {
	if query == "" {
		return true
	}
	if strings.Contains(entry.Emoji, query) {
		return true
	}
	for _, term := range entry.SearchTerms {
		if strings.Contains(term, query) {
			return true
		}
	}
	return false
}

// Filter returns matching entries in catalog order up to the optional positive limit.
func Filter(entries []Entry, query string, limit int) []Entry {
	query = strings.ToLower(strings.TrimSpace(query))
	results := make([]Entry, 0, min(len(entries), max(0, limit)))
	for _, entry := range entries {
		if !matchNormalized(entry, query) {
			continue
		}
		results = append(results, entry)
		if limit > 0 && len(results) >= limit {
			break
		}
	}
	return results
}
