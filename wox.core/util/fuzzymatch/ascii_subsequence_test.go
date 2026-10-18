package fuzzymatch

import "testing"

// TestNormalizedASCIISubsequence compares the byte shortcut with the Unicode
// filter, including patterns that normalize diacritics or require literal Han.
func TestNormalizedASCIISubsequence(t *testing.T) {
	for _, text := range []string{"", "a/b/cafe", "c:/users/a-b/report.txt", "aaaa/ab"} {
		for _, raw := range []string{"", "AB/C", "a/b", "café", "aa/a", "z", "项目"} {
			p := PreparePattern(raw)
			if got, want := p.MatchesNormalizedASCII(text), p.AdvanceSubsequence(text, 0) == len(p.normalizedRunes); got != want {
				t.Fatalf("text=%q pattern=%q: got %v want %v", text, raw, got, want)
			}
		}
	}
}
