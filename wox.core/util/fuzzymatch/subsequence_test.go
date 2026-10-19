package fuzzymatch

import "testing"

// TestSubsequenceAcrossComponents keeps the streaming rejection filter lossless
// for case, diacritics, Unicode, and matches crossing directory boundaries.
func TestSubsequenceAcrossComponents(t *testing.T) {
	for _, tc := range []struct {
		pattern string
		parts   []string
		want    int
	}{
		{"ab/cafe", []string{"/root/", "a/", "b/", "Café"}, 7},
		{"目标/f", []string{"/root/", "目标/", "folder"}, 4},
		{"a/z", []string{"/root/", "a/", "folder"}, 2},
		{"", []string{"/root/"}, 0},
	} {
		pattern := PreparePattern(tc.pattern)
		matched := 0
		for _, part := range tc.parts {
			matched = pattern.AdvanceSubsequence(part, matched)
		}
		if matched != tc.want {
			t.Fatalf("%q parts=%v: progress=%d want=%d", tc.pattern, tc.parts, matched, tc.want)
		}
	}
}
