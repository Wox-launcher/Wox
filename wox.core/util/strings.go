package util

import (
	"strings"
)

// PadChar left-pads s with the rune r, to length n.
// If n is smaller than s, PadChar is a no-op.
func LeftPad(s string, n int, r rune) string {
	if len(s) > n {
		return s
	}
	return strings.Repeat(string(r), n-len(s)) + s
}

func EllipsisEnd(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen < 3 {
		maxLen = 3
	}
	return string(runes[0:maxLen-3]) + "..."
}

func EllipsisMiddle(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen < 3 {
		maxLen = 3
	}
	half := maxLen / 2
	return string(runes[0:half-1]) + "..." + string(runes[len(runes)-half+2:])

}

// IsStringMatch performs fuzzy string matching between term and subTerm.
// It uses a multi-factor scoring algorithm similar to fzf, with support for:
// - Diacritics normalization (é -> e, ü -> u, etc.)
// - Chinese pinyin matching (when usePinYin is true)
// - CamelCase and boundary matching bonuses
// - Consecutive character matching bonuses
func IsStringMatch(term string, subTerm string, usePinYin bool) bool {
	result := FuzzyMatch(term, subTerm, usePinYin)
	return result.IsMatch
}

// IsStringMatchScore performs fuzzy string matching and returns both match status and score.
// Higher scores indicate better matches. The scoring considers:
// - Exact matches (highest score)
// - Prefix matches (high score)
// - Boundary matches (CamelCase, after delimiters)
// - Consecutive character matches
// - Gap penalties for non-consecutive matches
func IsStringMatchScore(term string, subTerm string, usePinYin bool) (isMatch bool, score int64) {
	result := FuzzyMatch(term, subTerm, usePinYin)
	return result.IsMatch, result.Score
}

// UniqueStrings removes empty entries and de-duplicates while preserving order.
func UniqueStrings(inputs []string) []string {
	seen := make(map[string]struct{}, len(inputs))
	var result []string
	for _, v := range inputs {
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		result = append(result, v)
	}
	return result
}
