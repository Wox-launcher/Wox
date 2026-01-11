package fuzzymatch

import (
	"strings"
	"sync"
	"sync/atomic"
	"unicode"
)

// Pinyin cache to avoid repeated computation
var (
	pinyinCache     sync.Map // map[string][]string
	pinyinCacheSize atomic.Int32
)

const (
	maxPinyinCacheSize = 4096 // Maximum cache entries
	maxPinyinVariants  = 16   // Limit multiplyTerms output to avoid exponential growth
)

// getCharPinyin returns pre-processed pinyin variants for a single rune
// If the rune is a Chinese character, returns its pinyin list (no tones)
// Otherwise, returns the character itself within a slice
func getCharPinyin(r rune) []string {
	if pinyins, ok := PinyinDict[int(r)]; ok {
		return pinyins
	}
	// Non-Chinese character: return as-is
	return []string{string(r)}
}

func getPinYin(term string) []string {
	if !hasChinese(term) {
		return []string{term}
	}

	// Check cache first
	if cached, ok := pinyinCache.Load(term); ok {
		return cached.([]string)
	}

	// Step 1: Convert to pinyin terms, grouping non-Chinese characters
	// e.g. "Hello世界" -> [ ["Hello"], ["shi"], ["jie"] ]
	// This dramatically reduces the depth of multiplyTerms recursion for mixed text
	var pinyinTerms [][]string

	var asciiBuilder strings.Builder
	asciiBuilder.Grow(16)

	for _, r := range term {
		if unicode.Is(unicode.Han, r) {
			// Flush buffered ASCII if any
			if asciiBuilder.Len() > 0 {
				pinyinTerms = append(pinyinTerms, []string{asciiBuilder.String()})
				asciiBuilder.Reset()
			}

			// Handle Chinese char
			pinyinTerms = append(pinyinTerms, getCharPinyin(r))
		} else {
			// Buffer non-Chinese char
			asciiBuilder.WriteRune(r)
		}
	}
	// Flush remaining ASCII
	if asciiBuilder.Len() > 0 {
		pinyinTerms = append(pinyinTerms, []string{asciiBuilder.String()})
	}

	// Step 2: Generate heteronym combinations (Cartesian product)
	var heteronymTerms [][]string
	for _, pinyinTerm := range pinyinTerms {
		// if pinyinTerm is too long, only use first letter, otherwise it will generate too many terms and cost too much time
		// Optimization cleanup: restored checking input length (pinyinTerms) instead of result length
		if len(pinyinTerms) > 10 {
			if len(pinyinTerm) > 1 {
				pinyinTerm = pinyinTerm[:1]
			}
		}

		heteronymTerms = multiplyTerms(heteronymTerms, pinyinTerm)
	}

	// Step 3: Generate output strings
	// We need two types of output:
	// 1. Full Pinyin: "zhong guo ren"
	// 2. First Letters: "z g r"

	var terms []string
	terms = make([]string, 0, len(heteronymTerms)*2)

	// Combine full pinyin
	for _, termParts := range heteronymTerms {
		terms = append(terms, strings.Join(termParts, " "))
	}

	// Combine first letters
	// Optimization: Reuse the structure of heteronymTerms but take first char
	for _, termParts := range heteronymTerms {
		var sb strings.Builder
		// Pre-calculate length: len(parts) + (len(parts)-1) for spaces
		needed := 0
		if len(termParts) > 0 {
			needed = len(termParts)*2 - 1
		}
		if needed > 0 {
			sb.Grow(needed)
			for i, part := range termParts {
				if i > 0 {
					sb.WriteByte(' ')
				}
				if len(part) > 0 {
					sb.WriteByte(part[0])
				}
			}
			terms = append(terms, sb.String())
		}
	}

	// Store in cache (simple LRU-like: clear if too large)
	if pinyinCacheSize.Load() >= maxPinyinCacheSize {
		// Simple eviction: clear all
		pinyinCache.Clear()
		pinyinCacheSize.Store(0)
	}
	pinyinCache.Store(term, terms)
	pinyinCacheSize.Add(1)

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
			// Limit initial terms as well
			if len(terms) >= maxPinyinVariants {
				break
			}
		}
		return terms
	}

	// Limit variants to avoid exponential growth
	// If we already have maxPinyinVariants, only add first pronunciation
	if len(terms) >= maxPinyinVariants {
		n = n[:1]
	}

	newTerms := make([][]string, 0, len(terms)*len(n))
	for _, term := range terms {
		for _, v := range n {
			newTerm := make([]string, len(term), len(term)+1)
			copy(newTerm, term)
			newTerm = append(newTerm, v)
			newTerms = append(newTerms, newTerm)
			// Hard limit on total variants
			if len(newTerms) >= maxPinyinVariants {
				return newTerms
			}
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
