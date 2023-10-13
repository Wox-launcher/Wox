package util

import (
	"github.com/mozillazg/go-pinyin"
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

func StringContains(term string, subTerm string) bool {
	term = strings.ToLower(term)
	subTerm = strings.ToLower(subTerm)
	return strings.Contains(term, subTerm)
}

func StringContainsPinYin(term string, subTerm string) bool {
	termPinyins := pinyin.LazyPinyin(term, pinyin.NewArgs())
	for _, termPinyin := range termPinyins {
		if strings.Contains(termPinyin, subTerm) {
			return true
		}
	}

	return false
}
