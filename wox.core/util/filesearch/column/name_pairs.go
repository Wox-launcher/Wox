package column

import (
	"math/bits"
	"wox/util/fuzzymatch"
)

const (
	// NamePairAlphabet encodes a-z (0..25), '.' (26), and ASCII digits 0-9 (27).
	NamePairAlphabet       = 28
	legacyNamePairAlphabet = 27
)

// namePairClass folds ASCII letters and keeps extension dots and digits distinct.
func namePairClass(c byte) int {
	if c >= 'A' && c <= 'Z' {
		c += 'a' - 'A'
	}
	if c >= 'a' && c <= 'z' {
		return int(c - 'a')
	}
	if c == '.' {
		return 26
	}
	if c >= '0' && c <= '9' {
		return 27
	}
	return -1
}

func appendUniqueNamePair(pairs []int, pair int) []int {
	for _, existing := range pairs {
		if existing == pair {
			return pairs
		}
	}
	return append(pairs, pair)
}

// NamePairClasses appends necessary ordered pairs, not adjacent byte pairs.
// Each literal must be ASCII; Unicode normalization remains scorer-owned.
func NamePairClasses(text string, pairs []int) []int {
	for i := range len(text) {
		if text[i] >= 128 {
			return pairs
		}
	}
	firstClass := -1
	previous := -1
	classifiedCount := 0
	for i := range len(text) {
		class := namePairClass(text[i])
		if class < 0 {
			continue
		}
		classifiedCount++
		if firstClass < 0 {
			firstClass = class
		}
		if previous >= 0 {
			pairs = appendUniqueNamePair(pairs, previous*NamePairAlphabet+class)
		}
		previous = class
	}
	if classifiedCount >= 3 && firstClass >= 0 && previous >= 0 {
		pairs = appendUniqueNamePair(pairs, firstClass*NamePairAlphabet+previous)
	}
	return pairs
}

// prepareNamePairs runs only while writing a generation. Older mappings remain
// readable without allocating a replacement index or rewriting persisted data.
func (s *Snapshot) prepareNamePairs() {
	words := (s.Names() + 63) / 64
	if len(s.NamePairBitmaps) == NamePairAlphabet*NamePairAlphabet*words && s.NamePairStride == NamePairAlphabet {
		return
	}
	s.NamePairBitmaps = make([]uint64, NamePairAlphabet*NamePairAlphabet*words)
	s.NamePairStride = NamePairAlphabet
	for id := 0; id < s.Names(); id++ {
		var seen uint32
		var pairs [NamePairAlphabet]uint32
		name := s.Name(uint32(id))
		if s.NameFlags[id]&NameNonASCII != 0 {
			pairs = fuzzymatch.PossibleASCIIOrderedPairs(name)
		} else {
			for i := range len(name) {
				class := namePairClass(name[i])
				if class >= 0 {
					pairs[class] |= seen
					seen |= 1 << class
				}
			}
		}
		for second, firsts := range pairs {
			for firsts != 0 {
				first := bits.TrailingZeros32(firsts)
				firsts &= firsts - 1
				s.NamePairBitmaps[(first*NamePairAlphabet+second)*words+id/64] |= uint64(1) << uint(id%64)
			}
		}
	}
}
