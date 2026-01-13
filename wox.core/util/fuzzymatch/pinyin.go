package fuzzymatch

import (
	"strings"
	"sync"
	"sync/atomic"
	"unicode"
)

// Pinyin cache to avoid repeated computation
var (
	pinyinCache     sync.Map // map[string][]PinyinSegment
	pinyinCacheSize atomic.Int32
)

const (
	maxPinyinCacheSize = 4096 // Maximum cache entries
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

// PinyinSegment represents a segment of the pinyin string (one character or a block of non-Chinese text)
// It contains all possible pronunciations for that segment.
type PinyinSegment struct {
	Syllables    []string // All possible full pinyin syllables for this segment (e.g. ["xing", "hang"])
	FirstLetters []rune   // Pre-calculated lowercase first letters for each syllable (e.g. ['x', 'h'])
	IsChinese    bool     // Whether this segment is a Chinese character (true) or non-Chinese text block (false)
}

// getPinYin returns the pinyin segments for the term.
// Unlike the previous implementation, this returns a graph of segments (possibilities)
// rather than expanding all combinations into full strings.
func getPinYin(term string) []PinyinSegment {
	if !hasChinese(term) {
		// Non-Chinese: single segment
		var firstLetter rune
		if len(term) > 0 {
			firstLetter = toLowerASCII(rune(term[0]))
		}

		return []PinyinSegment{{
			Syllables:    []string{term},
			FirstLetters: []rune{firstLetter},
			IsChinese:    false,
		}}
	}

	// Check cache first
	if cached, ok := pinyinCache.Load(term); ok {
		return cached.([]PinyinSegment)
	}

	var segments []PinyinSegment

	var asciiBuilder strings.Builder
	asciiBuilder.Grow(16)

	// Helper to flush ASCII buffer
	flushASCII := func() {
		if asciiBuilder.Len() > 0 {
			s := asciiBuilder.String()
			var fl rune
			if len(s) > 0 {
				fl = toLowerASCII(rune(s[0]))
			}
			segments = append(segments, PinyinSegment{
				Syllables:    []string{s},
				FirstLetters: []rune{fl},
				IsChinese:    false,
			})
			asciiBuilder.Reset()
		}
	}

	for _, r := range term {
		if unicode.Is(unicode.Han, r) {
			flushASCII()

			// Handle Chinese char
			pinyins := getCharPinyin(r)

			// Compute first letters
			firstLetters := make([]rune, len(pinyins))
			for i, p := range pinyins {
				if len(p) > 0 {
					firstLetters[i] = toLowerASCII(rune(p[0]))
				}
			}

			segments = append(segments, PinyinSegment{
				Syllables:    pinyins,
				FirstLetters: firstLetters,
				IsChinese:    true,
			})
		} else {
			// Buffer non-Chinese char
			asciiBuilder.WriteRune(r)
		}
	}
	flushASCII()

	// Store in cache (simple LRU-like: clear if too large)
	if pinyinCacheSize.Load() >= maxPinyinCacheSize {
		// Simple eviction: clear all
		pinyinCache.Clear()
		pinyinCacheSize.Store(0)
	}
	pinyinCache.Store(term, segments)
	pinyinCacheSize.Add(1)

	return segments
}

func hasChinese(str string) bool {
	for _, runeValue := range str {
		if unicode.Is(unicode.Han, runeValue) {
			return true
		}
	}

	return false
}
