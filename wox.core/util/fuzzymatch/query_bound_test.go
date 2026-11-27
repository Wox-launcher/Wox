package fuzzymatch

import (
	"math/rand"
	"strings"
	"testing"
)

// TestASCIIQueryBounds compares pruning and linear alignment against unpruned
// scoring and the previous exhaustive alignment transition.
func TestASCIIQueryBounds(t *testing.T) {
	rng := rand.New(rand.NewSource(19))
	alphabet := "aAbBcCst_ .-/012"
	for iteration := 0; iteration < 30000; iteration++ {
		text := make([]byte, 1+rng.Intn(64))
		for i := range text {
			text[i] = alphabet[rng.Intn(len(alphabet))]
		}
		pattern := make([]byte, 1+rng.Intn(8))
		for i := range pattern {
			pattern[i] = alphabet[rng.Intn(len(alphabet))]
		}
		a, okA := calculateBestAlignmentScoreASCII(string(text), string(pattern))
		if len(pattern) == 2 {
			a, okA = scoreTwoASCII(string(text), string(pattern))
		}
		b, okB := referenceAlignmentASCII(string(text), string(pattern))
		if okA != okB || okA && a != b {
			t.Fatalf("alignment %q %q: %v/%d != %v/%d", text, pattern, okA, a, okB, b)
		}
		prepared := PreparePattern(string(pattern))
		candidate := strings.TrimSpace(string(text))
		full := FuzzyMatch(candidate, string(pattern), false)
		if full.IsMatch && ASCIIScoreUpperBound(prepared, candidate, ASCIIScoreHints(string(text))) < full.Score {
			t.Fatalf("invalid bound %q %q: %+v", text, pattern, full)
		}
		for _, minimum := range []int64{0, 32, 64, 100, full.Score, full.Score + 1} {
			got := FuzzyMatchPatternAtLeast(candidate, prepared, true, false, minimum)
			if full.IsMatch && full.Score >= minimum && got != full || got.IsMatch && (!full.IsMatch || got.Score != full.Score || got.Score < minimum) {
				t.Fatalf("pruning %q %q minimum=%d: %+v != %+v", text, pattern, minimum, got, full)
			}
		}
	}

	// Targeted edge cases: repeated letters with different boundary slopes, digits,
	// punctuation, leading/trailing spaces, and non-ASCII/pinyin pruning parity.
	for _, tc := range []struct {
		text    string
		pattern string
	}{
		{"ba-_a", "ba-a"},
		{"ba_a", "aa"},
		{"baA", "aa"},
		{"xaba_a", "aba"},
		{"docx.scale-200.png", "dsc2"},
		{"200_dsc.png", "dsc2"},
		{" a_b2 ", "ab2"},
		{"test-123_abc.txt", "t1a"},
		{"a", "a"},
		{"x_a", "a"},
	} {
		candidate := strings.TrimSpace(tc.text)
		prepared := PreparePattern(tc.pattern)
		full := FuzzyMatch(candidate, tc.pattern, false)
		if full.IsMatch && ASCIIScoreUpperBound(prepared, candidate, ASCIIScoreHints(tc.text)) < full.Score {
			t.Fatalf("invalid edge bound %q %q: %+v", tc.text, tc.pattern, full)
		}
		for _, minimum := range []int64{0, 1, 16, 32, 64, full.Score, full.Score + 1} {
			got := FuzzyMatchPatternAtLeast(candidate, prepared, true, false, minimum)
			if full.IsMatch && full.Score >= minimum && got != full || got.IsMatch && (!full.IsMatch || got.Score != full.Score || got.Score < minimum) {
				t.Fatalf("edge pruning %q %q minimum=%d: %+v != %+v", tc.text, tc.pattern, minimum, got, full)
			}
		}
	}

	for _, tc := range []struct {
		text    string
		pattern string
	}{
		{"重庆火锅.md", "cq"},
		{"重庆火锅.md", "chongqing"},
		{"Wox测试2026.txt", "woxcs2026"},
		{"项目报告文档管理系统.txt", "xm"},
		{"测试abcdefg.txt", "abc"},
	} {
		prepared := PreparePattern(tc.pattern)
		full := FuzzyMatch(tc.text, tc.pattern, true)
		for minimum := int64(0); minimum <= 300; minimum += 15 {
			got := FuzzyMatchPatternAtLeast(tc.text, prepared, false, true, minimum)
			if full.IsMatch && full.Score >= minimum && got != full || got.IsMatch && (!full.IsMatch || got.Score != full.Score || got.Score < minimum) {
				t.Fatalf("pinyin pruning %q %q minimum=%d: %+v != %+v", tc.text, tc.pattern, minimum, got, full)
			}
		}
	}

	// Long ASCII names (>64 bytes) and digit-repeat/triple-digit hints, plus legacy-hint compatibility.
	for _, tc := range []struct {
		text    string
		pattern string
	}{
		{"amd64_microsoft-windows-foo_10.0.26100.8972_none_abcdef1234567890abcdef1234.dll", "100"},
		{"amd64_microsoft-windows-foo_10.0.26100.8972_none_abcdef1234567890abcdef1234.dll", "1000"},
		{"amd64_microsoft-windows-foo_10.0.26100.8972_none_abcdef1234567890abcdef1234.dll", "10.0"},
		{"amd64_microsoft-windows-foo_10.0.26100.8972_none_abcdef1234567890abcdef1234.dll", "000"},
		{"amd64_microsoft-windows-userexperience-shared_31bf3856ad364e35_10.0.26100.8972_none_b2893c8d70f22708_docx.scale-200.png", "dsc2"},
		{"amd64_microsoft-windows-userexperience-shared_31bf3856ad364e35_2026.0.1000_none_b2893c8d70f22708_report.log", "2026"},
	} {
		prepared := PreparePattern(tc.pattern)
		full := FuzzyMatch(tc.text, tc.pattern, false)
		hints, second := ASCIINameScoreHints(tc.text)
		if full.IsMatch && ASCIIScoreUpperBoundHint(prepared, len(tc.text), hints, second) < full.Score {
			t.Fatalf("invalid long digit bound %q %q: %+v", tc.text, tc.pattern, full)
		}
		legacyHints := hints &^ (uint64(0xf) << 60)
		legacySecond := second &^ (uint64(0xfff) << 52)
		if full.IsMatch && ASCIIScoreUpperBoundHint(prepared, len(tc.text), legacyHints, legacySecond) < full.Score {
			t.Fatalf("invalid legacy digit bound %q %q: %+v", tc.text, tc.pattern, full)
		}
		for _, minimum := range []int64{0, 1, 16, 50, 84, 103, full.Score, full.Score + 1} {
			got := FuzzyMatchPatternAtLeast(tc.text, prepared, true, false, minimum)
			if full.IsMatch && full.Score >= minimum && got != full || got.IsMatch && (!full.IsMatch || got.Score != full.Score || got.Score < minimum) {
				t.Fatalf("long pruning %q %q minimum=%d: %+v != %+v", tc.text, tc.pattern, minimum, got, full)
			}
		}
	}

	for iteration := 0; iteration < 5000; iteration++ {
		text := make([]byte, 65+rng.Intn(96))
		for i := range text {
			text[i] = alphabet[rng.Intn(len(alphabet))]
		}
		pattern := make([]byte, 1+rng.Intn(8))
		for i := range pattern {
			pattern[i] = alphabet[rng.Intn(len(alphabet))]
		}
		candidate := strings.TrimSpace(string(text))
		prepared := PreparePattern(string(pattern))
		full := FuzzyMatch(candidate, string(pattern), false)
		hints, second := ASCIINameScoreHints(string(text))
		if full.IsMatch && ASCIIScoreUpperBoundHint(prepared, len(candidate), hints, second) < full.Score {
			t.Fatalf("invalid random long bound %q %q: %+v", text, pattern, full)
		}
		for _, minimum := range []int64{0, 16, 50, 84, 103, full.Score, full.Score + 1} {
			got := FuzzyMatchPatternAtLeast(candidate, prepared, true, false, minimum)
			if full.IsMatch && full.Score >= minimum && got != full || got.IsMatch && (!full.IsMatch || got.Score != full.Score || got.Score < minimum) {
				t.Fatalf("random long pruning %q %q minimum=%d: %+v != %+v", text, pattern, minimum, got, full)
			}
		}
	}
}

// referenceAlignmentASCII retains the original exhaustive transitions as an
// independent oracle for the optimized prefix-maximum recurrence.
func referenceAlignmentASCII(text string, pattern string) (int64, bool) {
	textLen := len(text)
	patternLen := len(pattern)
	if patternLen == 0 || patternLen > textLen || patternLen > optimalAlignmentPatternLimit || textLen > optimalAlignmentTextLimit {
		return 0, false
	}

	var scores [optimalAlignmentStateSize]int64
	var seen [optimalAlignmentStateSize]bool

	for textIdx := 0; textIdx < textLen; textIdx++ {
		if toLowerByte(text[textIdx]) != toLowerByte(pattern[0]) {
			continue
		}

		stateIdx := textIdx
		scores[stateIdx] = scoreMatch + calculateMatchBonusASCII(text, textIdx) + calculateLeadingGapPenalty(textIdx)
		seen[stateIdx] = true
	}

	for patternIdx := 1; patternIdx < patternLen; patternIdx++ {
		rowFound := false
		rowBase := patternIdx * textLen
		prevRowBase := (patternIdx - 1) * textLen

		for textIdx := patternIdx; textIdx < textLen; textIdx++ {
			if toLowerByte(text[textIdx]) != toLowerByte(pattern[patternIdx]) {
				continue
			}

			bestScore := int64(0)
			foundScore := false
			matchBonus := scoreMatch + calculateMatchBonusASCII(text, textIdx)

			for prevIdx := patternIdx - 1; prevIdx < textIdx; prevIdx++ {
				stateIdx := prevRowBase + prevIdx
				if !seen[stateIdx] {
					continue
				}

				score := scores[stateIdx] + matchBonus + calculateGapPenalty(prevIdx, textIdx)
				if textIdx == prevIdx+1 {
					score += bonusConsecutive
				}
				if !foundScore || score > bestScore {
					bestScore = score
					foundScore = true
				}
			}

			if foundScore {
				scores[rowBase+textIdx] = bestScore
				seen[rowBase+textIdx] = true
				rowFound = true
			}
		}

		if !rowFound {
			return 0, false
		}
	}

	bestScore := int64(0)
	found := false
	lastRowBase := (patternLen - 1) * textLen
	matchRatioBonus := calculateMatchRatioBonus(patternLen, textLen)
	for textIdx := patternLen - 1; textIdx < textLen; textIdx++ {
		stateIdx := lastRowBase + textIdx
		if !seen[stateIdx] {
			continue
		}

		score := scores[stateIdx] + calculateTrailingGapPenalty(textLen, textIdx) + matchRatioBonus
		if !found || score > bestScore {
			bestScore = score
			found = true
		}
	}

	return bestScore, found
}
