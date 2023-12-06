package util

import (
	"github.com/sahilm/fuzzy"
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

func IsStringMatch(term string, subTerm string, usePinYin bool) bool {
	isMatch, _ := IsStringMatchScore(term, subTerm, usePinYin)
	return isMatch
}

func IsStringMatchScore(term string, subTerm string, usePinYin bool) (isMatch bool, score int64) {
	var minMatchScore int64 = 0

	if usePinYin {
		var matchScore int64 = -100000000
		pyTerms := getPinYin(term)
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
