package util

import (
	"strings"

	"github.com/sahilm/fuzzy"
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

func IsStringMatch(term string, subTerm string, usePinYin bool) bool {
	isMatch, _ := IsStringMatchScore(term, subTerm, usePinYin)
	return isMatch
}

func IsStringMatchScore(term string, subTerm string, usePinYin bool) (isMatch bool, score int64) {
	term = strings.ToLower(term)
	subTerm = strings.ToLower(subTerm)

	match, s := isStringMatchScoreFuzzy(term, subTerm, usePinYin)
	if match {
		return true, s
	}

	// fuzzy match is not good enough, E.g. term = 我爱摄影, subTerm = 摄, isStringMatchScoreFuzzy will return negative score
	// So we need to check if subTerm is a substring of term if fuzzy match failed
	if strings.Contains(term, subTerm) {
		return true, int64(len(subTerm))
	}

	return false, 0
}

func isStringMatchScoreFuzzy(term string, subTerm string, usePinYin bool) (isMatch bool, score int64) {
	var minMatchScore int64 = 0

	if usePinYin {
		var matchScore int64 = -100000000
		pyTerms := getPinYin(term)
		pyTerms = append(pyTerms, term) // add original term
		for _, newTerm := range pyTerms {
			matches := fuzzy.Find(subTerm, []string{newTerm})
			if len(matches) == 0 {
				continue
			}

			if int64(matches[0].Score) > matchScore {
				matchScore = int64(matches[0].Score)
			}
		}

		if matchScore == -100000000 {
			return false, 0
		}

		return matchScore >= minMatchScore, matchScore
	} else {
		matches := fuzzy.Find(subTerm, []string{term})
		if len(matches) == 0 {
			return false, 0
		}
		return int64(matches[0].Score) >= minMatchScore, int64(matches[0].Score)
	}
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
