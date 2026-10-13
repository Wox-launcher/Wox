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
