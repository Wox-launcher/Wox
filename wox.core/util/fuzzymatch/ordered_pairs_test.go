package fuzzymatch

import "testing"

// TestOrderedPairsRetainMatches checks normalization, polyphonic readings,
// initials, repeated letters and punctuation against the independent scorer.
func TestOrderedPairsRetainMatches(t *testing.T) {
	for _, text := range []string{"重庆火锅.md", "Wox测试2026.txt", "QQ音乐播放器", "café_Über.txt", "项目报告.txt", "重行长乐.txt", "maibn.go", "A.a", "报告𠀀.go", string([]byte{'a', 255, 'b', '.', 'c'})} {
		pairs := PossibleASCIIOrderedPairs(text)
		for _, pattern := range []string{"cq", "chongqing", "zhongqing", "woxcs", "qqyy", "cafe", "uber", "xm", "xiangmu", "baogao", "zhongxing", "changyue", "maibn.go", "aa", "a.a", "bg.go", "abc", "a.c"} {
			if !FuzzyMatch(text, pattern, true).IsMatch {
				continue
			}
			previous := -1
			for _, c := range pattern {
				class := orderedPairClass(c)
				if class < 0 {
					continue
				}
				if previous >= 0 && pairs[class]&(1<<previous) == 0 {
					t.Fatalf("%q matching %q lost ordered pair %d,%d", text, pattern, previous, class)
				}
				previous = class
			}
		}
	}
}
