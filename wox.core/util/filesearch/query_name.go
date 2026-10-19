package filesearch

import (
	"context"
	"path/filepath"
	"strings"
	"wox/util/filesearch/column"
	"wox/util/fuzzymatch"
)

// scoreColumnName runs compiled filename predicates without constructing paths
// or repeating pattern normalization for every distinct name.
func scoreColumnName(query SearchQuery, name string, ascii bool, minimum int64, bounds *columnNameBounds) (bool, int64) {
	plan := query.plan
	normalizedName := ""
	if len(plan.exactPhrases) > 0 {
		normalizedName = normalizeIndexText(name)
		for _, phrase := range plan.exactNamePhrases {
			if !strings.Contains(normalizedName, phrase) {
				return false, 0
			}
		}
	}
	if query.wildcard != nil {
		if suffix := query.wildcard.asciiSuffix; suffix != "" && ascii && !strings.ContainsRune(name, '\n') {
			if len(name) < len(suffix) || !strings.EqualFold(name[len(name)-len(suffix):], suffix) {
				return false, 0
			}
			return true, int64(query.wildcard.literalCount*1000 - len(name))
		}
		return query.wildcard.match(name, name)
	}
	if len(plan.andTerms) > 0 {
		// Extension clauses are exact filters and should reject before fuzzy work,
		// regardless of where the user placed the clause in the query.
		extension := ""
		for _, term := range plan.andTerms {
			if term.plan.extensionOnly {
				if extension == "" {
					extension = normalizeExtension(filepath.Ext(name))
				}
				if extension != term.plan.extension {
					return false, 0
				}
			}
		}
		remaining := int64(0)
		bounded := ascii && bounds != nil
		if bounded {
			for _, term := range plan.andTerms {
				upper := int64(500)
				if !term.plan.extensionOnly {
					upper = columnNameUpperBound(term, bounds.length, bounds.first, bounds.second)
				}
				if upper >= 1<<60 || remaining >= 1<<60-upper {
					bounded = false
					break
				}
				remaining += upper
			}
		}
		var total int64
		for _, term := range plan.andTerms {
			if bounded {
				upper := int64(500)
				if !term.plan.extensionOnly {
					upper = columnNameUpperBound(term, bounds.length, bounds.first, bounds.second)
				}
				remaining -= upper
			}
			if term.plan.extensionOnly {
				if normalizeExtension(filepath.Ext(name)) != term.plan.extension {
					return false, 0
				}
				total += 500
			} else {
				required := int64(0)
				if bounded {
					required = minimum - total - remaining
				}
				matched, score := scoreColumnName(term, name, ascii, required, bounds)
				if !matched {
					return false, 0
				}
				total += score
			}
		}
		return true, total
	}
	if plan.raw == "" && len(plan.exactPhrases) > 0 {
		score := int64(3000)
		for _, phrase := range plan.exactNamePhrases {
			score += int64(5000 + utf8Len(phrase)*100 - utf8Len(normalizedName))
		}
		return true, score
	}
	fuzzyMinimum := minimum - 4000
	if plan.extension != "" {
		fuzzyMinimum -= 500
	}
	result := fuzzymatch.FuzzyMatchPatternAtLeast(strings.TrimSpace(name), plan.pattern, ascii, plan.usePinyin, fuzzyMinimum)
	score := result.Score + 4000
	if plan.extension != "" && normalizeExtension(filepath.Ext(name)) == plan.extension {
		if !result.IsMatch {
			return plan.extensionOnly, 500
		}
		score += 500
	}
	return result.IsMatch, score
}

// exactColumnNameThreshold establishes a lower bound from K live exact-name
// entries before fuzzy scoring. It changes neither recall nor tie ordering.
func exactColumnNameThreshold(ctx context.Context, view column.View, query SearchQuery, limit int) int64 {
	plan := query.plan
	if query.wildcard != nil {
		// A wildcard with no literal characters has a negative length score;
		// zero is not a valid pruning floor before any results are retained.
		return -1 << 60
	}
	if limit <= 0 || len(plan.andTerms) != 0 || len(plan.exactPhrases) != 0 || plan.extension != "" {
		return 0
	}
	if !plan.patternASCII {
		return 0
	}
	base := view.Base
	for _, exact := range []bool{true, false} {
		count := 0
		for _, id := range base.FrequentNameIDs {
			if ctx.Err() != nil {
				return 0
			}
			name := base.Name(id)
			if base.NameFlags[id]&column.NameNonASCII != 0 || len(name) < len(plan.raw) {
				continue
			}
			if exact && len(name) != len(plan.raw) || !strings.EqualFold(name[:len(plan.raw)], plan.raw) {
				continue
			}
			for _, row := range base.NameRows[base.NameStarts[id]:base.NameStarts[id+1]] {
				if _, changed := view.Delta[base.IDs[row]]; changed {
					continue
				}
				if base.Flags[row]&column.Structural != 0 {
					continue
				}
				parent := base.Parents[row]
				if view.DirectoryUnchanged(parent) {
					if base.DirectoryPath(base.RowDirectory[parent]) == "" {
						continue
					}
				} else if !view.Reachable(base.IDs[row]) {
					continue
				}
				count++
				if count >= limit {
					if exact {
						return 4000 + 100 + int64(len(plan.raw)*16)
					}
					return 4000 + 32 + int64(len(plan.raw)*16)
				}
			}
		}
	}
	return 0
}

// columnNameFilter combines necessary literal characters across AND clauses.
// A non-ASCII clause disables only its own ASCII mask, never another clause's.
func columnNameFilter(query SearchQuery) (mask uint64, requiresNonASCII bool) {
	plan := query.plan
	if len(plan.andTerms) > 0 {
		for _, term := range plan.andTerms {
			m, n := columnNameFilter(term)
			mask |= m
			requiresNonASCII = requiresNonASCII || n
		}
	} else {
		raw := plan.raw
		if query.wildcard != nil {
			raw = strings.Join(plan.wildcardLiterals, "")
		}
		m, nonASCII := column.CharacterMask(raw)
		if !nonASCII {
			mask |= m
		} else {
			requiresNonASCII = fuzzymatch.PatternRequiresNonASCII(raw)
		}
	}
	for _, phrase := range plan.exactNamePhrases {
		m, nonASCII := column.CharacterMask(phrase)
		if !nonASCII {
			mask |= m
		} else {
			requiresNonASCII = requiresNonASCII || fuzzymatch.PatternRequiresNonASCII(phrase)
		}
	}
	return
}

// columnRepeatedLetters keeps repetitions within each AND term independent:
// "a a" can reuse the same character, whereas "aa" must match two positions.
func columnRepeatedLetters(query SearchQuery) uint32 {
	if len(query.plan.andTerms) > 0 {
		var repeated uint32
		for _, term := range query.plan.andTerms {
			repeated |= columnRepeatedLetters(term)
		}
		return repeated
	}
	raw := query.plan.raw
	if query.wildcard != nil {
		raw = strings.Join(query.plan.wildcardLiterals, "")
	}
	var seen, repeated uint32
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		if c < 'a' || c > 'z' {
			continue
		}
		letter := uint32(1) << (c - 'a')
		repeated |= seen & letter
		seen |= letter
	}
	return repeated
}

// columnNameUpperBound composes safe score bounds using only name metadata.
// Unknown phrase/Unicode bounds stay unbounded rather than restricting recall.
func columnNameUpperBound(query SearchQuery, length int, hints, second uint64) int64 {
	plan := query.plan
	if len(plan.exactPhrases) > 0 {
		return 1 << 60
	}
	if query.wildcard != nil {
		return int64(query.wildcard.literalCount*1000 - length)
	}
	if len(plan.andTerms) > 0 {
		var total int64
		for _, term := range plan.andTerms {
			if term.plan.extensionOnly {
				total += 500
			} else {
				bound := columnNameUpperBound(term, length, hints, second)
				if bound >= 1<<60 || total >= 1<<60-bound {
					return 1 << 60
				}
				total += bound
			}
		}
		return total
	}
	if !plan.patternASCII {
		return 1 << 60
	}
	bound := fuzzymatch.ASCIIScoreUpperBoundHint(plan.pattern, length, hints, second) + 4000
	if plan.extension != "" {
		bound += 500
	}
	return bound
}

// columnNameBounds borrows a candidate's scalar metadata while composing AND
// thresholds; each term need only reach the score the other terms cannot supply.
type columnNameBounds struct {
	length        int
	first, second uint64
}
