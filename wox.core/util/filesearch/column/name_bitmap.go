package column

import "math/bits"

// prepareNameBitmaps transposes per-name character masks so one intersection
// rejects 64 names before touching their strings. Unicode names are admitted
// conservatively: normalization and every pinyin reading remain scorer-owned.
func (s *Snapshot) prepareNameBitmaps() {
	words := (s.Names() + 63) / 64
	s.NameBitmaps = make([]uint64, 64*words)
	s.NameRepeatBitmaps = make([]uint64, 26*words)
	s.NonASCIINameBitmap = make([]uint64, words)
	for id := 0; id < s.Names(); id++ {
		word, bit := id/64, uint64(1)<<uint(id%64)
		mask := s.NameMasks[id]
		var repeated uint32
		if s.NameFlags[id]&NameNonASCII != 0 {
			mask = ^uint64(0)
			repeated = (1 << 26) - 1
			s.NonASCIINameBitmap[word] |= bit
		} else {
			var seen uint32
			name := s.Name(uint32(id))
			for i := 0; i < len(name); i++ {
				c := name[i]
				if c >= 'A' && c <= 'Z' {
					c += 'a' - 'A'
				}
				if c < 'a' || c > 'z' {
					continue
				}
				letter := uint32(1) << (c - 'a')
				repeated |= seen & letter
				seen |= letter
			}
		}
		for mask != 0 {
			c := bits.TrailingZeros64(mask)
			s.NameBitmaps[c*words+word] |= bit
			mask &= mask - 1
		}
		for repeated != 0 {
			c := bits.TrailingZeros32(repeated)
			s.NameRepeatBitmaps[c*words+word] |= bit
			repeated &= repeated - 1
		}
	}
}
