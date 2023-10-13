package util

import (
	"github.com/lithammer/fuzzysearch/fuzzy"
	"github.com/mozillazg/go-pinyin"
	"strings"
)

var MaxStringMatcherRankScore = 20

// PadChar left-pads s with the rune r, to length n.
// If n is smaller than s, PadChar is a no-op.
func LeftPad(s string, n int, r rune) string {
	if len(s) > n {
		return s
	}
	return strings.Repeat(string(r), n-len(s)) + s
}

func StringMatch(term string, subTerm string, usePinYin bool) bool {
	score, pinyinData := stringMatchScore(term, subTerm, usePinYin)
	if score == -1 {
		return false
	}

	if pinyinData != "" {
		// log
	}
	return score < MaxStringMatcherRankScore
}

func stringMatchScore(term string, subTerm string, usePinYin bool) (int, string) {
	if usePinYin {
		smaller := 10000
		yinpinData := ""
		pyterms := getPinYin(term)
		for _, newTerm := range pyterms {
			newRank := fuzzy.RankMatchFold(subTerm, newTerm)
			if newRank == -1 {
				continue
			}
			if newRank < smaller {
				smaller = newRank
				yinpinData = newTerm
			}
		}

		if smaller == 10000 {
			return -1, ""
		}

		return smaller, yinpinData
	} else {
		return fuzzy.RankMatchFold(subTerm, term), ""
	}

}

// "QQ音乐" => ["qqyinle", "qqyinyue","qqyl","qqyy"]
func getPinYin(term string) []string {
	args := pinyin.NewArgs()
	args.Heteronym = true
	args.Fallback = func(r rune, a pinyin.Args) []string {
		return []string{string(r)}
	}
	pinyinTerms := pinyin.Pinyin(term, args)

	// remove duplicate heteronym itemin pinyinTerms
	var newPinyinTerms [][]string
	for _, pinyinTerm := range pinyinTerms {
		var newPinyinTerm []string
		for _, word := range pinyinTerm {
			if !stringInSlice(word, newPinyinTerm) {
				newPinyinTerm = append(newPinyinTerm, word)
			}
		}
		newPinyinTerms = append(newPinyinTerms, newPinyinTerm)
	}

	var heteronymTerms [][]string
	for _, pinyinTerm := range pinyinTerms {
		heteronymTerms = multiplyTerms(heteronymTerms, pinyinTerm)
	}

	// use first letter of every heteronym item
	var firstLetterTerms [][]string
	for _, heteronymTerm := range heteronymTerms {
		var innerTerms []string
		for _, word := range heteronymTerm {
			innerTerms = append(innerTerms, word[0:1])
		}

		firstLetterTerms = append(firstLetterTerms, innerTerms)
	}

	var terms []string
	for _, newTerm := range heteronymTerms {
		terms = append(terms, strings.ToLower(strings.Join(newTerm, "")))
	}
	for _, firstLetterTerm := range firstLetterTerms {
		terms = append(terms, strings.ToLower(strings.Join(firstLetterTerm, "")))
	}

	return terms
}

func stringInSlice(term string, terms []string) bool {
	for _, v := range terms {
		if v == term {
			return true
		}
	}

	return false
}

// [["1","2"]] => [["1","2","3"], ["1","2","4"]]
func multiplyTerms(terms [][]string, n []string) [][]string {
	if len(terms) == 0 {
		for _, v := range n {
			terms = append(terms, []string{v})
		}
		return terms
	}

	newTerms := [][]string{}
	for _, term := range terms {
		for _, v := range n {
			newTerm := make([]string, len(term))
			copy(newTerm, term)
			newTerm = append(newTerm, v)
			newTerms = append(newTerms, newTerm)
		}
	}

	return newTerms
}
