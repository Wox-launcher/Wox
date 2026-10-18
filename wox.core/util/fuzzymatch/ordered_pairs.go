package fuzzymatch

import "unicode"

// PossibleASCIIOrderedPairs returns necessary-order filters over letters and
// dots. It unions normalized text and every pronunciation without enumerating
// complete pinyin combinations. Rows are indexed by the second character.
func PossibleASCIIOrderedPairs(text string) [27]uint32 {
	var pairs [27]uint32
	var originalBuffer, normalizedBuffer [64]rune
	original, normalized := originalBuffer[:0], normalizedBuffer[:0]
	hasChinese := processText(text, &original, &normalized)
	var seen uint32
	for _, r := range normalized {
		if class := orderedPairClass(r); class >= 0 {
			pairs[class] |= seen
			seen |= 1 << class
		}
	}
	if !hasChinese {
		return pairs
	}
	seen = 0
	for _, r := range original {
		readings, _ := LookupCharPinyin(r)
		if !unicode.Is(unicode.Han, r) || len(readings) == 0 {
			if class := orderedPairClass(r); class >= 0 {
				pairs[class] |= seen
				seen |= 1 << class
			}
			continue
		}
		var alternatives uint32
		for _, reading := range readings {
			local := seen
			for _, c := range reading {
				if class := orderedPairClass(c); class >= 0 {
					pairs[class] |= local
					local |= 1 << class
				}
			}
			alternatives |= local
		}
		seen |= alternatives
	}
	// Initials are subsequences of the same readings; their pairs are included.
	return pairs
}

// orderedPairClass shares one scalar alphabet across normalization and pinyin.
func orderedPairClass(r rune) int {
	r = unicode.ToLower(r)
	if r >= 'a' && r <= 'z' {
		return int(r - 'a')
	}
	if r == '.' {
		return 26
	}
	return -1
}
