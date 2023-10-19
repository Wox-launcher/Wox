package util

import (
	"github.com/mozillazg/go-pinyin"
	"strings"
	"unicode"
)

func init() {
	//https://github.com/mozillazg/pinyin-data/blob/889cde8bb0769747849f1d26bfc60c18efee1db3/kTGHZ2013.txt
	//how to generate? https://github.com/mozillazg/go-pinyin/issues/57#issuecomment-1126608914

	pinyin.PinyinDict = make(map[int]string)
	for k, v := range PinyinDict {
		pinyin.PinyinDict[k] = v
	}
}

func getPinYin(term string) []string {
	if !hasChinese(term) {
		return []string{term}
	}

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
	for _, pinyinTerm := range newPinyinTerms {
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
		terms = append(terms, strings.Join(newTerm, " "))
	}
	for _, firstLetterTerm := range firstLetterTerms {
		terms = append(terms, strings.Join(firstLetterTerm, " "))
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

func hasChinese(str string) bool {
	for _, runeValue := range str {
		if unicode.Is(unicode.Han, runeValue) {
			return true
		}
	}

	return false
}
