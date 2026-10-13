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
	pinyinCacheMu   sync.Mutex

	// pinyinDict is the shared character table. It is built on the first Chinese
	// lookup and dropped after a long hide so the map buckets do not stay resident.
	pinyinDictMu sync.Mutex
	pinyinDict   atomic.Pointer[map[int][]string]
)

const (
	maxPinyinCacheSize = 4096 // Maximum cache entries
)

// getCharPinyin returns pre-processed pinyin variants for a single rune
// If the rune is a Chinese character, returns its pinyin list (no tones)
// Otherwise, returns the character itself within a slice
func getCharPinyin(r rune) []string {
	if pinyins, ok := LookupCharPinyin(r); ok {
		return pinyins
	}
	// Non-Chinese character: return as-is
	return []string{string(r)}
}

// LookupCharPinyin returns the tone-stripped syllables for one Han character.
// The table is loaded on first use.
func LookupCharPinyin(r rune) ([]string, bool) {
	dict := pinyinDict.Load()
	if dict == nil {
		pinyinDictMu.Lock()
		dict = pinyinDict.Load()
		if dict == nil {
			loaded := loadPinyinDict()
			dict = &loaded
			pinyinDict.Store(dict)
		}
		pinyinDictMu.Unlock()
	}
	pinyins, ok := (*dict)[int(r)]
	return pinyins, ok
}

// ReleasePinyinDictionary drops the shared pinyin table after the launcher has
// stayed hidden. The next Chinese lookup rebuilds it.
func ReleasePinyinDictionary() {
	pinyinDictMu.Lock()
	pinyinDict.Store(nil)
	pinyinDictMu.Unlock()
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

	// A full-index scan can exceed this bound by only a few names. Clearing
	// every entry at capacity made each subsequent scan rebuild the entire cache.
	// Keep admitted entries until the existing idle-release lifecycle clears them.
	pinyinCacheMu.Lock()
	if pinyinCacheSize.Load() < maxPinyinCacheSize {
		if _, loaded := pinyinCache.LoadOrStore(strings.Clone(term), segments); !loaded {
			pinyinCacheSize.Add(1)
		}
	}
	pinyinCacheMu.Unlock()

	return segments
}

// ReleaseIdleCaches drops the pinyin memoization table while the UI is hidden.
// Entries are cheap to recompute per term, so trading a cold lookup after the
// next show for a smaller idle footprint is worthwhile.
func ReleaseIdleCaches() {
	pinyinCacheMu.Lock()
	defer pinyinCacheMu.Unlock()
	pinyinCache.Clear()
	pinyinCacheSize.Store(0)
}

func hasChinese(str string) bool {
	for _, runeValue := range str {
		if unicode.Is(unicode.Han, runeValue) {
			return true
		}
	}

	return false
}
