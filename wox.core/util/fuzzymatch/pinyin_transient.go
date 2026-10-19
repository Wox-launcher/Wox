package fuzzymatch

import (
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

// pinyinScratch lends segment headers and literal slots to one synchronous
// match. Dictionary readings are shared; source substrings never survive Put.
type pinyinScratch struct {
	segments []PinyinSegment
	literals []string
	initials []rune
}

var pinyinScratchPool = sync.Pool{New: func() any {
	return &pinyinScratch{segments: make([]PinyinSegment, 0, 64), literals: make([]string, 0, 64), initials: make([]rune, 0, 64)}
}}

// matchPinyinText avoids filling a memoization cache for thousands of distinct
// indexed names. Scratch is cleared before pooling so borrowed mmap strings and
// dictionary generations cannot be retained after their owners release them.
func matchPinyinText(text string, pattern []rune) FuzzyMatchResult {
	scratch := pinyinScratchPool.Get().(*pinyinScratch)
	defer func() {
		clear(scratch.segments)
		clear(scratch.literals)
		scratch.segments, scratch.literals, scratch.initials = scratch.segments[:0], scratch.literals[:0], scratch.initials[:0]
		if cap(scratch.segments) <= 1024 && cap(scratch.literals) <= 1024 && cap(scratch.initials) <= 4096 {
			pinyinScratchPool.Put(scratch)
		}
	}()
	if !utf8.ValidString(text) {
		// The previous rune builder emitted one replacement rune per invalid byte.
		text = strings.Map(func(r rune) rune { return r }, text)
	}
	start := 0
	for index, r := range text {
		if !unicode.Is(unicode.Han, r) {
			continue
		}
		if start < index {
			scratch.appendLiteral(text[start:index], false)
		}
		end := index + utf8.RuneLen(r)
		if syllables, initials, ok := lookupPinyinReading(r); ok {
			scratch.segments = append(scratch.segments, PinyinSegment{Syllables: syllables, FirstLetters: initials, IsChinese: true})
		} else {
			scratch.appendLiteral(text[index:end], true)
		}
		start = end
	}
	if start < len(text) {
		scratch.appendLiteral(text[start:], false)
	}
	return matchPinyinStrict(scratch.segments, pattern)
}

// appendLiteral lends one source substring without allocating a builder or an
// independent string/slice for every non-Han block or unknown Han character.
func (s *pinyinScratch) appendLiteral(text string, chinese bool) {
	s.literals = append(s.literals, text)
	s.initials = append(s.initials, toLowerASCII(rune(text[0])))
	i := len(s.literals) - 1
	s.segments = append(s.segments, PinyinSegment{Syllables: s.literals[i : i+1], FirstLetters: s.initials[i : i+1], IsChinese: chinese})
}
