package util

import (
	"strings"
	"unicode/utf8"

	"wox/util/fuzzymatch"
)

// LeftPad left-pads s with the rune r, to length n.
func LeftPad(s string, n int, r rune) string {
	if len(s) >= n {
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
	if maxLen >= 4 && utf8.ValidString(s) {
		if utf8.RuneCountInString(s) <= maxLen {
			return s
		}
		// Only the ends are displayed; allocating a rune slice for the entire
		// clipboard text can consume several times the input size.
		half := maxLen / 2
		prefixEnd := 0
		for i := 0; i < half-1; i++ {
			_, size := utf8.DecodeRuneInString(s[prefixEnd:])
			prefixEnd += size
		}
		suffixStart := len(s)
		for i := 0; i < half-2; i++ {
			_, size := utf8.DecodeLastRuneInString(s[:suffixStart])
			suffixStart -= size
		}
		return s[:prefixEnd] + "..." + s[suffixStart:]
	}
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
	result := fuzzymatch.FuzzyMatch(term, subTerm, usePinYin)
	return result.IsMatch
}

func IsStringMatchScore(term string, subTerm string, usePinYin bool) (isMatch bool, score int64) {
	result := fuzzymatch.FuzzyMatch(term, subTerm, usePinYin)
	return result.IsMatch, result.Score
}

func UniqueStrings(slice []string) []string {
	keys := make(map[string]bool)
	var list []string
	for _, entry := range slice {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}
