package fuzzymatch

import (
	"sync/atomic"
	"unicode"
)

// Global generation counter to ensure unique generation IDs across function calls.
// This prevents stale generation values from causing incorrect matches when buffers are reused.
var globalGeneration atomic.Uint32

//////////////////////////////////////////////////////////////////////////////////
///
///   SHOULD KEEP THIS FILE IN SYNC WITH wox_fuzzy_match_util.dart IN wox.ui.flutter
///
/////////////////////////////////////////////////////////////////////////////////////

// FuzzyMatchResult represents the result of a fuzzy match operation
type FuzzyMatchResult struct {
	IsMatch bool  // Whether the pattern matches the text
	Score   int64 // The match score (higher is better)
}

// Scoring constants inspired by fzf algorithm
const (
	scoreMatch          = 16
	scoreGapStart       = -3
	scoreGapExtension   = -1
	bonusBoundary       = scoreMatch / 2    // 8
	bonusNonWord        = scoreMatch / 2    // 8
	bonusCamelCase      = bonusBoundary + 2 // 10
	bonusFirstCharMatch = bonusBoundary + 4 // 12
	bonusConsecutive    = 5
	bonusPrefixMatch    = 20  // Exact prefix match bonus
	bonusExactMatch     = 100 // Exact match bonus
)

// FuzzyMatch performs fuzzy matching between pattern and text
// It supports:
// - Multi-factor scoring similar to fzf
// - Diacritics normalization (é -> e, ü -> u, etc.)
// - Chinese pinyin matching when usePinYin is true
func FuzzyMatch(text string, pattern string, usePinYin bool) FuzzyMatchResult {
	if pattern == "" {
		return FuzzyMatchResult{IsMatch: true, Score: 0}
	}
	if text == "" {
		return FuzzyMatchResult{IsMatch: false, Score: 0}
	}

	// OPTIMIZATION 1: Pure ASCII fast path
	// If both text and pattern are pure ASCII, we can skip rune conversion entirely
	patternIsASCII := isASCII(pattern)
	textIsASCII := isASCII(text)

	if patternIsASCII && textIsASCII {
		return fuzzyMatchASCII(text, pattern)
	}

	// For non-ASCII text (Chinese etc.), skip first-char screening
	// as pinyin matching handles it differently

	// Standard path for non-ASCII text
	// Get buffer for pattern runes
	patternBufPtr := getRuneBuffer()
	defer putRuneBuffer(patternBufPtr)

	patternRunes := normalizeToRunes(pattern, *patternBufPtr)
	*patternBufPtr = patternRunes

	// Get buffer for text runes
	textBufPtr := getRuneBuffer()
	defer putRuneBuffer(textBufPtr)

	// Get buffer for original runes
	originalBufPtr := getRuneBuffer()
	defer putRuneBuffer(originalBufPtr)

	// Process text: normalize, populate original buffer, and check for Chinese in ONE PASS
	hasChineseChar := processText(text, originalBufPtr, textBufPtr)
	textRunes := *textBufPtr
	originalRunes := *originalBufPtr

	// Try exact match first (highest priority)
	if equalRunes(textRunes, patternRunes) {
		return FuzzyMatchResult{IsMatch: true, Score: bonusExactMatch + int64(len(pattern)*scoreMatch)}
	}

	// Try prefix match (high priority)
	if hasPrefixRunes(textRunes, patternRunes) {
		patternRuneCount := len(patternRunes)
		score := bonusPrefixMatch + int64(patternRuneCount*scoreMatch) + bonusFirstCharMatch
		return FuzzyMatchResult{IsMatch: true, Score: score}
	}

	// Try fuzzy match on the original text
	result := fuzzyMatchCore(originalRunes, textRunes, patternRunes)
	if result.IsMatch {
		return result
	}

	// Try pinyin matching for Chinese text
	if usePinYin && hasChineseChar {
		pinyinResult := matchPinyinStrict(text, patternRunes)
		if pinyinResult.IsMatch {
			return pinyinResult
		}
	}

	// Fallback: substring match (lower score)
	if containsRunes(textRunes, patternRunes) {
		score := int64(len(patternRunes))
		return FuzzyMatchResult{IsMatch: true, Score: score}
	}

	return FuzzyMatchResult{IsMatch: false, Score: 0}
}

// isASCII checks if a string contains only ASCII characters
func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 128 {
			return false
		}
	}
	return true
}

// fuzzyMatchASCII is an optimized path for pure ASCII text and pattern
// It avoids rune conversion entirely, working directly with bytes
func fuzzyMatchASCII(text string, pattern string) FuzzyMatchResult {
	textLen := len(text)
	patternLen := len(pattern)

	if patternLen > textLen {
		return FuzzyMatchResult{IsMatch: false, Score: 0}
	}

	// Exact match check (case-insensitive)
	if textLen == patternLen && equalFoldASCII(text, pattern) {
		return FuzzyMatchResult{IsMatch: true, Score: bonusExactMatch + int64(patternLen*scoreMatch)}
	}

	// Prefix match check (case-insensitive)
	if hasPrefixFoldASCII(text, pattern) {
		return FuzzyMatchResult{IsMatch: true, Score: bonusPrefixMatch + int64(patternLen*scoreMatch) + bonusFirstCharMatch}
	}

	// Fuzzy match - find all pattern chars in order
	patternIdx := 0
	var matchedIndexes [64]int // Stack buffer for common cases
	matchedSlice := matchedIndexes[:0]

	for textIdx := 0; textIdx < textLen && patternIdx < patternLen; textIdx++ {
		if toLowerByte(text[textIdx]) == toLowerByte(pattern[patternIdx]) {
			matchedSlice = append(matchedSlice, textIdx)
			patternIdx++
		}
	}

	if patternIdx != patternLen {
		return FuzzyMatchResult{IsMatch: false, Score: 0}
	}

	// Calculate score
	score := calculateScoreASCII(text, matchedSlice, patternLen)

	// Apply minimum score threshold
	minScore := calculateMinScoreThreshold(patternLen, textLen)
	if score < minScore {
		return FuzzyMatchResult{IsMatch: false, Score: 0}
	}

	return FuzzyMatchResult{IsMatch: true, Score: score}
}

// equalFoldASCII checks if two ASCII strings are equal (case-insensitive)
func equalFoldASCII(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		if toLowerByte(a[i]) != toLowerByte(b[i]) {
			return false
		}
	}
	return true
}

// hasPrefixFoldASCII checks if ASCII string a starts with b (case-insensitive)
func hasPrefixFoldASCII(a, b string) bool {
	if len(a) < len(b) {
		return false
	}
	for i := 0; i < len(b); i++ {
		if toLowerByte(a[i]) != toLowerByte(b[i]) {
			return false
		}
	}
	return true
}

// toLowerByte converts ASCII byte to lowercase
func toLowerByte(b byte) byte {
	if b >= 'A' && b <= 'Z' {
		return b + ('a' - 'A')
	}
	return b
}

// calculateScoreASCII calculates match score for ASCII fuzzy match
func calculateScoreASCII(text string, matchedIndexes []int, patternLen int) int64 {
	if len(matchedIndexes) == 0 {
		return 0
	}

	var score int64 = 0
	prevMatchIdx := -1

	for i, matchIdx := range matchedIndexes {
		score += scoreMatch

		// First char bonus
		if matchIdx == 0 {
			score += bonusFirstCharMatch
		}

		// Boundary bonus
		if matchIdx > 0 {
			prevChar := text[matchIdx-1]
			currChar := text[matchIdx]

			// CamelCase
			if prevChar >= 'a' && prevChar <= 'z' && currChar >= 'A' && currChar <= 'Z' {
				score += bonusCamelCase
			}

			// After delimiter
			if isDelimiterByte(prevChar) {
				score += bonusBoundary
			}

			// Non-word to word
			if !isAlnumByte(prevChar) && isAlnumByte(currChar) {
				score += bonusNonWord
			}
		}

		// Consecutive bonus
		if i > 0 && matchIdx == prevMatchIdx+1 {
			score += bonusConsecutive
		}

		// Gap penalty
		if prevMatchIdx >= 0 {
			gap := matchIdx - prevMatchIdx - 1
			if gap > 0 {
				score += scoreGapStart + int64(gap-1)*scoreGapExtension
			}
		} else if matchIdx > 0 {
			penalty := int64(matchIdx) * scoreGapExtension
			if penalty < -15 {
				penalty = -15
			}
			score += penalty
		}

		prevMatchIdx = matchIdx
	}

	// Trailing gap penalty
	textLen := len(text)
	if prevMatchIdx >= 0 && prevMatchIdx < textLen-1 {
		trailingGap := textLen - prevMatchIdx - 1
		penalty := int64(trailingGap) * scoreGapExtension / 2
		if penalty < -10 {
			penalty = -10
		}
		score += penalty
	}

	// Match ratio bonus
	matchRatio := float64(patternLen) / float64(textLen)
	if matchRatio > 0.5 {
		score += int64(matchRatio * 10)
	}

	return score
}

// isDelimiterByte checks if byte is a delimiter
func isDelimiterByte(b byte) bool {
	switch b {
	case ' ', '-', '_', '.', '/', '\\', ':', ',', ';', '(', ')', '[', ']', '{', '}':
		return true
	}
	return false
}

// isAlnumByte checks if byte is alphanumeric
func isAlnumByte(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

// toLowerASCII is a fast path for lowercase conversion of ASCII letters
func toLowerASCII(r rune) rune {
	if r >= 'A' && r <= 'Z' {
		return r + ('a' - 'A')
	}
	return r
}

// Maximum allowed consecutive skipped syllables before rejecting match
// This prevents matching scattered syllables like "道"..."沿" in "J道:解惑授道-国际软件架构前沿"
const maxConsecutiveSkippedSyllables = 3

// matchPinyinStrict performs strict pinyin matching using segment graph.
// Only allows: all first letters (e.g., "nh" for "你好") OR all full pinyin (e.g., "nihao" for "你好")
// Does NOT allow mixed mode (e.g., "nhao" or "nih")
// Now uses a state-based search (limited beam) to handle polyphonic ambiguities without exponential complexity.
func matchPinyinStrict(text string, patternRunes []rune) FuzzyMatchResult {
	segments := getPinYin(text)
	if len(segments) == 0 {
		return FuzzyMatchResult{IsMatch: false, Score: 0}
	}

	bestScore := int64(0)
	matched := false

	// Check 1: Exact first letters match / prefix match
	// Iterate pattern and segments in lockstep
	// For pattern[i], it must match one of segments[i].FirstLetters
	firstLetMatch := true
	firstLetScore := int64(0)

	if len(patternRunes) <= len(segments) {
		for i, r := range patternRunes {
			seg := segments[i]
			found := false
			for _, fl := range seg.FirstLetters {
				if fl == r {
					found = true
					break
				}
			}
			if !found {
				firstLetMatch = false
				break
			}
		}

		if firstLetMatch {
			// Calculate score for first letter match
			if len(patternRunes) == len(segments) {
				firstLetScore = bonusExactMatch + int64(len(patternRunes)*scoreMatch)
			} else {
				firstLetScore = bonusPrefixMatch + int64(len(patternRunes)*scoreMatch) + bonusFirstCharMatch
			}
			matched = true
			bestScore = firstLetScore
		}
	}

	// Check 2: Syllable-level matching (Beam Search)
	// If first letter exact match found, it's likely the best, but full pinyin might have higher score?
	// Usually Exact First Letters is very high. Let's keep checking syllables just in case (e.g. fewer skips).
	// Actually, "Exact First Letter" is usually unbeatable unless Full Pinyin is also Exact.

	const (
		ModeAny         = 0
		ModeFirstLetter = 1
		ModeFullPinyin  = 2
	)

	type searchState struct {
		patternIdx          int
		consecutiveSkipped  int
		matchedSyllables    int
		score               int64
		lastMatchWasPartial bool
		matchMode           int // 0: Any, 1: FirstLetter, 2: FullPinyin
	}

	// Active states
	states := []searchState{{0, 0, 0, 0, false, ModeAny}}

	// Pre-allocate next states buffer
	nextStates := make([]searchState, 0, 32)

	// Optimization: Use flat slices instead of map for deduplication
	// Key space: (PatternIdx+1) * (MaxMatched+1) * 3 (Modes) * (MaxSkips+1)
	rows := len(patternRunes) + 1
	cols1 := len(segments) + 1
	cols2 := 3 * (maxConsecutiveSkippedSyllables + 1)
	size := rows * cols1 * cols2

	// Use pool to avoid allocation
	bestScoresPtr := getInt64Buffer()
	defer putInt64Buffer(bestScoresPtr)
	bestScores := *bestScoresPtr
	if cap(bestScores) < size {
		bestScores = make([]int64, size)
	} else {
		bestScores = bestScores[:size]
	}
	*bestScoresPtr = bestScores

	bestIndexPtr := getIntBuffer()
	defer putIntBuffer(bestIndexPtr)
	bestIndex := *bestIndexPtr
	if cap(bestIndex) < size {
		bestIndex = make([]int, size)
	} else {
		bestIndex = bestIndex[:size]
	}
	*bestIndexPtr = bestIndex

	generationPtr := getUint32Buffer()
	defer putUint32Buffer(generationPtr)
	generation := *generationPtr
	if cap(generation) < size {
		generation = make([]uint32, size)
	} else {
		generation = generation[:size]
	}
	*generationPtr = generation

	currentGen := globalGeneration.Add(uint32(len(segments) + 1))

	for _, seg := range segments {
		nextStates = nextStates[:0]
		currentGen++

		for _, state := range states {
			// Branch 1: Skip this segment (syllable)
			if state.matchedSyllables == 0 || state.consecutiveSkipped < maxConsecutiveSkippedSyllables {
				newScore := state.score
				newSkips := state.consecutiveSkipped + 1
				effectiveSkips := newSkips
				if effectiveSkips > maxConsecutiveSkippedSyllables {
					effectiveSkips = maxConsecutiveSkippedSyllables
				}

				// Key: (patternIdx * cols1 + matchedSyllables) * cols2 + Mode * 4 + Skips
				idx := (state.patternIdx*cols1+state.matchedSyllables)*cols2 + state.matchMode*4 + effectiveSkips
				if generation[idx] != currentGen || newScore > bestScores[idx] {
					newState := searchState{
						patternIdx:          state.patternIdx,
						consecutiveSkipped:  effectiveSkips,
						matchedSyllables:    state.matchedSyllables,
						score:               newScore,
						lastMatchWasPartial: false,
						matchMode:           state.matchMode,
					}
					if generation[idx] == currentGen {
						nextStates[bestIndex[idx]] = newState
					} else {
						bestIndex[idx] = len(nextStates)
						generation[idx] = currentGen
						nextStates = append(nextStates, newState)
					}
					bestScores[idx] = newScore
				}
			}

			// Branch 2: Try to match
			if state.patternIdx < len(patternRunes) {
				for _, syllable := range seg.Syllables {
					remainingRunes := len(patternRunes) - state.patternIdx
					syllableLen := len(syllable)

					// 1. Try Full Syllable Match
					if remainingRunes >= syllableLen {
						if matchASCIIPrefix(patternRunes[state.patternIdx:state.patternIdx+syllableLen], syllable) {
							if syllableLen == 1 || state.matchMode != ModeFirstLetter {
								newScore := state.score + scoreMatch*2
								if state.matchedSyllables > 0 && state.consecutiveSkipped == 0 {
									newScore += bonusConsecutive
								}

								newMode := state.matchMode
								if syllableLen > 1 {
									newMode = ModeFullPinyin
								}

								newPatternIdx := state.patternIdx + syllableLen
								newMatchedSyllables := state.matchedSyllables + 1
								idx := (newPatternIdx*cols1+newMatchedSyllables)*cols2 + newMode*4 + 0

								if generation[idx] != currentGen || newScore > bestScores[idx] {
									newState := searchState{
										patternIdx:          newPatternIdx,
										consecutiveSkipped:  0,
										matchedSyllables:    newMatchedSyllables,
										score:               newScore,
										lastMatchWasPartial: false,
										matchMode:           newMode,
									}
									if generation[idx] == currentGen {
										nextStates[bestIndex[idx]] = newState
									} else {
										bestIndex[idx] = len(nextStates)
										generation[idx] = currentGen
										nextStates = append(nextStates, newState)
									}
									bestScores[idx] = newScore
								}
							}
						}
					}

					// 2. Try Partial Match (Prefix)
					if remainingRunes < syllableLen {
						if matchASCIIPrefix(patternRunes[state.patternIdx:], syllable[:remainingRunes]) {
							// Sub-case A: Length 1 (First Letter)
							if remainingRunes == 1 {
								if state.matchMode != ModeFullPinyin {
									newScore := state.score + scoreMatch + 5
									if state.matchedSyllables > 0 && state.consecutiveSkipped == 0 {
										newScore += bonusConsecutive
									}

									newMode := ModeFirstLetter
									newPatternIdx := state.patternIdx + 1
									newMatchedSyllables := state.matchedSyllables + 1
									idx := (newPatternIdx*cols1+newMatchedSyllables)*cols2 + newMode*4 + 0

									if generation[idx] != currentGen || newScore > bestScores[idx] {
										newState := searchState{
											patternIdx:          newPatternIdx,
											consecutiveSkipped:  0,
											matchedSyllables:    newMatchedSyllables,
											score:               newScore,
											lastMatchWasPartial: true,
											matchMode:           newMode,
										}
										if generation[idx] == currentGen {
											nextStates[bestIndex[idx]] = newState
										} else {
											bestIndex[idx] = len(nextStates)
											generation[idx] = currentGen
											nextStates = append(nextStates, newState)
										}
										bestScores[idx] = newScore
									}
								}
							} else {
								// Sub-case B: Length > 1
								if state.matchMode != ModeFirstLetter && state.consecutiveSkipped == 0 {
									newScore := state.score + int64(remainingRunes)*scoreMatch
									if state.matchedSyllables > 0 {
										newScore += bonusConsecutive
									}

									newMode := ModeFullPinyin
									newPatternIdx := state.patternIdx + remainingRunes
									newMatchedSyllables := state.matchedSyllables + 1
									idx := (newPatternIdx*cols1+newMatchedSyllables)*cols2 + newMode*4 + 0

									if generation[idx] != currentGen || newScore > bestScores[idx] {
										newState := searchState{
											patternIdx:          newPatternIdx,
											consecutiveSkipped:  0,
											matchedSyllables:    newMatchedSyllables,
											score:               newScore,
											lastMatchWasPartial: true,
											matchMode:           newMode,
										}
										if generation[idx] == currentGen {
											nextStates[bestIndex[idx]] = newState
										} else {
											bestIndex[idx] = len(nextStates)
											generation[idx] = currentGen
											nextStates = append(nextStates, newState)
										}
										bestScores[idx] = newScore
									}
								}
							}
						}
					}
				}
			}
		}

		states = append(states[:0], nextStates...)
		if len(states) == 0 {
			break
		}
	}

	// Find best finished state
	for _, state := range states {
		if state.patternIdx == len(patternRunes) && state.matchedSyllables > 0 {
			finalScore := state.score
			if state.matchedSyllables == len(segments) && !state.lastMatchWasPartial {
				finalScore += bonusExactMatch
			}

			if !matched || finalScore > bestScore {
				bestScore = finalScore
				matched = true
			}
		}
	}

	return FuzzyMatchResult{IsMatch: matched, Score: bestScore}
}

// matchASCIIPrefix checks if pattern runes match the start of an ASCII string (case-insensitive)
// This avoids allocations from strings.ToLower and []rune conversion
func matchASCIIPrefix(pattern []rune, s string) bool {
	if len(pattern) > len(s) {
		return false
	}
	for i, pr := range pattern {
		sr := rune(s[i])
		// Normalize both to lowercase for comparison
		if toLowerASCII(pr) != toLowerASCII(sr) {
			return false
		}
	}
	return true
}

// fuzzyMatchCore performs the core matching algorithm
func fuzzyMatchCore(originalRunes []rune, textRunes []rune, patternRunes []rune) FuzzyMatchResult {
	textLen := len(textRunes)
	patternLen := len(patternRunes)

	if patternLen == 0 {
		return FuzzyMatchResult{IsMatch: true, Score: 0}
	}
	if textLen == 0 || patternLen > textLen {
		return FuzzyMatchResult{IsMatch: false, Score: 0}
	}

	// Use pooled buffer for matchedIndexes
	matchedIndexesPtr := getIntBuffer()

	// Ensure we have space for patternLen
	matchedIndexes := *matchedIndexesPtr
	if cap(matchedIndexes) < patternLen {
		matchedIndexes = make([]int, patternLen)
	} else {
		matchedIndexes = matchedIndexes[:patternLen]
	}
	*matchedIndexesPtr = matchedIndexes // Update pool pointer

	// Phase 1: distinct sequential scan to find the *first possible* valid match
	// This replaces the existence check call and the fallback logic
	// Optimizes for "No Match" speed (via fail fast) by combining check and record
	patternIdx := 0
	for textIdx := 0; textIdx < textLen && patternIdx < patternLen; textIdx++ {
		if textRunes[textIdx] == patternRunes[patternIdx] {
			matchedIndexes[patternIdx] = textIdx
			patternIdx++
		}
	}

	if patternIdx != patternLen {
		// Optimization: Put buffer back immediately if no match
		putIntBuffer(matchedIndexesPtr)
		return FuzzyMatchResult{IsMatch: false, Score: 0}
	}

	// Phase 2: heuristic optimization (improve match positions)
	// Try to shift matches to better positions (boundaries) if possible
	improveMatchPositions(originalRunes, textRunes, patternRunes, matchedIndexes)

	// Calculate final score
	score := calculateScore(originalRunes, textRunes, matchedIndexes, patternLen)

	// We are done with the buffer
	if matchedIndexesPtr != nil {
		putIntBuffer(matchedIndexesPtr)
	}

	// Apply minimum score threshold
	minScore := calculateMinScoreThreshold(patternLen, textLen)
	if score < minScore {
		return FuzzyMatchResult{IsMatch: false, Score: 0}
	}

	return FuzzyMatchResult{IsMatch: true, Score: score}
}

// improveMatchPositions tries to shift matches to better positions (boundaries/camelCase)
// It assumes matchedIndexes contains a valid match set (left-most)
func improveMatchPositions(originalRunes []rune, textRunes []rune, patternRunes []rune, matchedIndexes []int) {
	textLen := len(textRunes)
	patternLen := len(patternRunes)

	// Iterate backwards to allow shifting right-side matches further right?
	// Actually, iterating forward is safer to maintain order with fixed right-side constraints?
	// Constraint: matchedIndexes[i] < matchedIndexes[i+1]
	// If we start from left:
	// range for i is (matchedIndexes[i], matchedIndexes[i+1])
	// We can search in this gap for a 'better' match for pattern[i].

	// We only look ahead a small amount to avoid scanning whole strings
	const maxScanDist = 10

	for i := 0; i < patternLen; i++ {
		currentIdx := matchedIndexes[i]

		// If current is already a boundary, we are happy (greedy) regarding this specific character
		// (Unless a later boundary is "better"? No, usually first boundary is best for prefix/consecutive reasons)
		if isBoundaryChar(originalRunes, currentIdx) {
			// Mark as boundary for score calculation?
			// We can use negative index to flag boundary state to calculateScore,
			// BUT calculateScore needs positive indices to calculate gaps.
			// Let's stick to just finding the index.
			continue
		}

		// It's not a boundary. Can we find a boundary occurrence of pattern[i] appearing later?
		// Limit: up to next match index or maxScanDist
		limit := textLen
		if i < patternLen-1 {
			limit = matchedIndexes[i+1]
		}

		scanEnd := currentIdx + maxScanDist
		if scanEnd < limit {
			limit = scanEnd
		}

		// Scan forward
		foundBetter := false
		bestNewIdx := -1

		for next := currentIdx + 1; next < limit; next++ {
			if textRunes[next] == patternRunes[i] {
				if isBoundaryChar(originalRunes, next) {
					bestNewIdx = next
					foundBetter = true
					break // Found a boundary, take it!
				}
			}
		}

		if foundBetter {
			matchedIndexes[i] = bestNewIdx
		}
	}
}

// calculateScore computes the match score based on multiple factors
func calculateScore(originalRunes []rune, textRunes []rune, matchedIndexes []int, patternLen int) int64 {
	if len(matchedIndexes) == 0 {
		return 0
	}

	var score int64 = 0
	prevMatchIdx := -1

	for i, matchIdx := range matchedIndexes {
		// Base score for each match
		score += scoreMatch

		// Bonus for first character match
		if matchIdx == 0 {
			score += bonusFirstCharMatch
		}

		// Bonus for boundary matches (after delimiter, camelCase, etc.)
		if matchIdx > 0 {
			prevChar := originalRunes[matchIdx-1]
			currChar := originalRunes[matchIdx]

			// CamelCase bonus
			if unicode.IsLower(prevChar) && unicode.IsUpper(currChar) {
				score += bonusCamelCase
			}

			// Boundary bonus (after delimiter)
			if isDelimiter(prevChar) {
				score += bonusBoundary
			}

			// Non-word to word transition
			if !unicode.IsLetter(prevChar) && !unicode.IsNumber(prevChar) &&
				(unicode.IsLetter(currChar) || unicode.IsNumber(currChar)) {
				score += bonusNonWord
			}
		}

		// Consecutive match bonus
		if i > 0 && matchIdx == prevMatchIdx+1 {
			score += bonusConsecutive
		}

		// Gap penalty
		if prevMatchIdx >= 0 {
			gap := matchIdx - prevMatchIdx - 1
			if gap > 0 {
				score += scoreGapStart + int64(gap-1)*scoreGapExtension
			}
		} else if matchIdx > 0 {
			// Leading gap penalty (characters before first match)
			leadingGap := matchIdx
			if leadingGap > 0 {
				penalty := int64(leadingGap) * scoreGapExtension
				if penalty < -15 {
					penalty = -15 // Cap the penalty
				}
				score += penalty
			}
		}

		prevMatchIdx = matchIdx
	}

	// Trailing gap penalty (unmatched characters at the end)
	textLen := len(textRunes)
	if prevMatchIdx >= 0 && prevMatchIdx < textLen-1 {
		trailingGap := textLen - prevMatchIdx - 1
		penalty := int64(trailingGap) * scoreGapExtension / 2 // Half penalty for trailing
		if penalty < -10 {
			penalty = -10
		}
		score += penalty
	}

	// Bonus for matching a higher percentage of the text
	matchRatio := float64(patternLen) / float64(textLen)
	if matchRatio > 0.5 {
		score += int64(matchRatio * 10)
	}

	return score
}

// calculateMinScoreThreshold calculates the minimum acceptable score for a match
// Based on pattern length and text length to filter out low-quality scattered matches
func calculateMinScoreThreshold(patternLen int, textLen int) int64 {
	// Stricter thresholds to filter out scattered/random matches
	// We want: prefix matches, boundary matches, or highly consecutive matches

	if patternLen == 1 {
		// Single character must be at boundary or first position
		if textLen <= 2 {
			return scoreMatch // Very short text, ok
		}
		// Require boundary match (first char or after delimiter/camelCase)
		return scoreMatch + bonusBoundary
	}

	if patternLen == 2 {
		// Two characters: require some quality (consecutive or boundary)
		if textLen <= 4 {
			return scoreMatch * 2
		}
		// Longer text: need consecutive or boundary bonus
		return scoreMatch*2 + bonusConsecutive
	}

	if patternLen == 3 {
		// Three characters: require decent quality
		if textLen <= 6 {
			return int64(patternLen * scoreMatch * 2 / 3)
		}
		// Longer text: stricter requirement
		return int64(patternLen*scoreMatch*2/3) + bonusConsecutive
	}

	// For 4+ character patterns:
	// Calculate based on ratio - scattered matches over long text are low quality
	ratio := float64(patternLen) / float64(textLen)

	if ratio < 0.15 {
		// Pattern is very small relative to text - require high quality
		// e.g., "test" in "Microsoft Remote Desktop" (4/25 = 0.16)
		return int64(patternLen * scoreMatch)
	}

	if ratio < 0.3 {
		// Pattern is small relative to text
		return int64(patternLen * scoreMatch * 3 / 4)
	}

	if ratio < 0.5 {
		// Moderate ratio
		return int64(patternLen * scoreMatch * 2 / 3)
	}

	// Pattern is large relative to text - more lenient
	return int64(patternLen * scoreMatch / 2)
}

// Helper functions

// normalizeToRunes converts string to lowercase and removes diacritics, appending to buf
func normalizeToRunes(s string, buf []rune) []rune {
	for _, r := range s {
		// ASCII fast path
		if r < 128 {
			if 'A' <= r && r <= 'Z' {
				r += 'a' - 'A'
			}
			buf = append(buf, r)
			continue
		}

		// Convert to lowercase
		r = unicode.ToLower(r)

		// Remove diacritics by mapping to base character
		if normalized, ok := diacriticsMap[r]; ok {
			buf = append(buf, normalized)
		} else {
			buf = append(buf, r)
		}
	}

	return buf
}

// processText normalizes text, populates original buffer, and detects Chinese in one pass
func processText(text string, origBufPtr *[]rune, normBufPtr *[]rune) (hasChinese bool) {
	orig := *origBufPtr
	norm := *normBufPtr

	hasChinese = false
	for _, r := range text {
		orig = append(orig, r)

		// ASCII fast path (most common case)
		if r < 128 {
			if 'A' <= r && r <= 'Z' {
				r += 'a' - 'A'
			}
			norm = append(norm, r)
			continue
		}

		// Check for Chinese (U+4E00 - U+9FFF covers vast majority)
		// Chinese characters don't have case, so we can skip ToLower
		if r >= 0x4E00 && r <= 0x9FFF {
			hasChinese = true
			norm = append(norm, r)
			continue
		}

		// Extended CJK check (less common)
		if unicode.Is(unicode.Han, r) {
			hasChinese = true
			norm = append(norm, r)
			continue
		}

		// For diacritics range (Latin/Greek/Cyrillic), apply ToLower and diacritics mapping
		// Only lookup map for characters in relevant blocks
		// The highest character in our map is around U+2122. Safe cutoff U+3000.
		if r < 0x3000 {
			// Apply lowercase conversion for Latin-range characters
			r = unicode.ToLower(r)
			if normalized, ok := diacriticsMap[r]; ok {
				norm = append(norm, normalized)
			} else {
				norm = append(norm, r)
			}
		} else {
			// Other non-ASCII, non-Chinese characters (rare)
			norm = append(norm, r)
		}
	}

	*origBufPtr = orig
	*normBufPtr = norm
	return hasChinese
}

// Helper functions for rune slices

func equalRunes(a, b []rune) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func hasPrefixRunes(s, prefix []rune) bool {
	return len(s) >= len(prefix) && equalRunes(s[:len(prefix)], prefix)
}

func containsRunes(s, substr []rune) bool {
	if len(substr) == 0 {
		return true
	}
	if len(substr) > len(s) {
		return false
	}

	// Simple brute force - same as strings.Contains default for short strings
	for i := 0; i <= len(s)-len(substr); i++ {
		if equalRunes(s[i:i+len(substr)], substr) {
			return true
		}
	}
	return false
}

// equalRune compares two runes for equality (case-insensitive, diacritics-insensitive)

// isBoundaryChar checks if the character at the given index is at a word boundary
func isBoundaryChar(runes []rune, idx int) bool {
	if idx == 0 {
		return true
	}
	if idx >= len(runes) {
		return false
	}

	prev := runes[idx-1]
	curr := runes[idx]

	// CamelCase boundary
	if unicode.IsLower(prev) && unicode.IsUpper(curr) {
		return true
	}

	// After delimiter
	if isDelimiter(prev) {
		return true
	}

	// Non-letter/number to letter/number transition
	if !unicode.IsLetter(prev) && !unicode.IsNumber(prev) &&
		(unicode.IsLetter(curr) || unicode.IsNumber(curr)) {
		return true
	}

	return false
}

// isDelimiter checks if a rune is a delimiter character
func isDelimiter(r rune) bool {
	switch r {
	case ' ', '-', '_', '.', '/', '\\', ':', ',', ';', '(', ')', '[', ']', '{', '}':
		return true
	}
	return false
}
