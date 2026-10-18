package column

import (
	"math/bits"
	"wox/util/fuzzymatch"
)

// namePairClass folds ASCII letters and keeps the extension separator distinct.
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
	return -1
}

// NamePairClasses appends necessary ordered pairs, not adjacent byte pairs.
// Each literal must be ASCII; Unicode normalization remains scorer-owned.
func NamePairClasses(text string, pairs []int) []int {
	for i := range len(text) {
		if text[i] >= 128 {
			return pairs
		}
	}
	previous := -1
	for i := range len(text) {
		class := namePairClass(text[i])
		if class < 0 {
			continue
		}
		if previous >= 0 {
			pair := previous*27 + class
			found := false
			for _, existing := range pairs {
				found = found || existing == pair
			}
			if !found {
				pairs = append(pairs, pair)
			}
		}
		previous = class
	}
	return pairs
}

// prepareNamePairs runs only while writing a generation. Older mappings remain
// readable without allocating a replacement index or rewriting persisted data.
func (s *Snapshot) prepareNamePairs() {
	words := (s.Names() + 63) / 64
	if len(s.NamePairBitmaps) == 27*27*words {
		return
	}
	s.NamePairBitmaps = make([]uint64, 27*27*words)
	for id := 0; id < s.Names(); id++ {
		var seen uint32
		var pairs [27]uint32
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
				s.NamePairBitmaps[(first*27+second)*words+id/64] |= uint64(1) << uint(id%64)
			}
		}
	}
}
