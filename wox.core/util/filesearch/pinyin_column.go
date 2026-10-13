package filesearch

import (
	"wox/util/filesearch/column"
	"wox/util/fuzzymatch"
)

// prepareColumnNames augments the existing per-name mask without storing full
// pinyin combinations. Every reading contributes; exact scoring still chooses
// a valid sequence, so this compact rejection filter cannot change recall.
func prepareColumnNames(base *column.Snapshot) error {
	for id := 0; id < base.Names(); id++ {
		if base.NameFlags[id]&column.NameNonASCII == 0 || base.NameFlags[id]&column.NameMaskComplete != 0 {
			continue
		}
		characters := fuzzymatch.PossibleASCIICharacters(base.Name(uint32(id)), true)
		var buffer [128]byte
		text := buffer[:0]
		for character, present := range characters {
			if present {
				text = append(text, byte(character))
			}
		}
		mask, _ := column.CharacterMask(string(text))
		base.NameMasks[id] |= mask
		base.NameFlags[id] |= column.NameMaskComplete
	}
	return nil
}
