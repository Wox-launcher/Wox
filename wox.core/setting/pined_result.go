package setting

import "strings"

// PinedQueryResult is the set of query texts that keep one result pinned.
// A result can be pinned under more than one query. Each entry matches only that query.
type PinedQueryResult struct {
	Queries []string
}

// normalizePinedQuery trims the raw query the user pinned under.
// Surrounding space is not part of the prompt, so "b r" and "b r " stay one pin.
func normalizePinedQuery(query string) string {
	return strings.TrimSpace(query)
}

func (p PinedQueryResult) hasQuery(query string) bool {
	query = normalizePinedQuery(query)
	for _, saved := range p.Queries {
		if saved == query {
			return true
		}
	}
	return false
}

func (p PinedQueryResult) withQuery(query string) PinedQueryResult {
	query = normalizePinedQuery(query)
	if p.hasQuery(query) {
		return p
	}
	queries := make([]string, 0, len(p.Queries)+1)
	queries = append(queries, p.Queries...)
	queries = append(queries, query)
	return PinedQueryResult{Queries: queries}
}

// withoutQuery drops one query. The bool is false when the result is no longer pinned anywhere.
func (p PinedQueryResult) withoutQuery(query string) (PinedQueryResult, bool) {
	query = normalizePinedQuery(query)
	queries := make([]string, 0, len(p.Queries))
	for _, saved := range p.Queries {
		if saved != query {
			queries = append(queries, saved)
		}
	}
	return PinedQueryResult{Queries: queries}, len(queries) > 0
}
