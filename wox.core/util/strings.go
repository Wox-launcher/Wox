package util

import (
	"math/rand"
	"strings"
	"time"

	"wox/util/fuzzymatch"
)

// LeftPad pads a string to length l with char pad
func LeftPad(s string, pad byte, l int) string {
	if len(s) >= l {
		return s
	}
	return strings.Repeat(string(pad), l-len(s)) + s
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

func RandomString(n int) string {
	var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func init() {
	rand.Seed(time.Now().UnixNano())
}
