package fuzzymatch

import (
	"unicode"
	"unicode/utf8"
)

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

// containsByte checks if string contains a byte (faster than strings.IndexByte for simple check)
func containsByte(s string, b byte) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return true
		}
	}
	return false
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

// matchPinyinStrict performs strict pinyin matching
// Only allows: all first letters (e.g., "nh" for "你好") OR all full pinyin (e.g., "nihao" for "你好")
// Does NOT allow mixed mode (e.g., "nhao" or "nih")
func matchPinyinStrict(text string, patternRunes []rune) FuzzyMatchResult {
	// getPinYin now returns []PinyinVariant
	pinyinVariants := getPinYin(text)

	var bestResult FuzzyMatchResult

	for _, variant := range pinyinVariants {
		// variant.FirstLetters is pre-calculated logic
		if len(variant.FirstLetters) == 0 {
			continue
		}

		// Check 1: Exact first letters match - EARLY EXIT for high score
		if equalRunes(patternRunes, variant.FirstLetters) {
			score := bonusExactMatch + int64(len(patternRunes)*scoreMatch)
			// Early exit: exact first letter match is already very high score
			return FuzzyMatchResult{IsMatch: true, Score: score}
		}

		// Check 2: First letters prefix match
		if hasPrefixRunes(variant.FirstLetters, patternRunes) {
			score := bonusPrefixMatch + int64(len(patternRunes)*scoreMatch) + bonusFirstCharMatch
			if score > bestResult.Score {
				bestResult = FuzzyMatchResult{IsMatch: true, Score: score}
			}
			continue
		}

		// Check 3: Syllable-level matching
		// matchSyllables needs Parts
		if syllableResult := matchSyllables(variant.Parts, patternRunes); syllableResult.IsMatch {
			if syllableResult.Score > bestResult.Score {
				bestResult = syllableResult
			}
		}
	}

	return bestResult
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

// matchSyllables performs unified syllable-level matching
// Optimized to avoid allocations by using inline ASCII comparison
func matchSyllables(parts []string, patternRunes []rune) FuzzyMatchResult {
	if len(patternRunes) == 0 || len(parts) == 0 {
		return FuzzyMatchResult{IsMatch: false, Score: 0}
	}

	patternPos := 0
	partIdx := 0
	matchedSyllables := 0
	totalSkippedSyllables := 0
	consecutiveSkipped := 0
	lastMatchWasPartial := false

	for patternPos < len(patternRunes) && partIdx < len(parts) {
		part := parts[partIdx]
		partLen := len(part) // For ASCII pinyin, byte length == rune length

		remainingLen := len(patternRunes) - patternPos

		// Case 1: Remaining pattern starts with full syllable
		// Check if pattern[patternPos:patternPos+partLen] matches part (case-insensitive)
		if remainingLen >= partLen && matchASCIIPrefix(patternRunes[patternPos:patternPos+partLen], part) {
			patternPos += partLen
			matchedSyllables++
			partIdx++
			lastMatchWasPartial = false
			consecutiveSkipped = 0
			continue
		}

		// Case 2: Remaining pattern is a prefix of this syllable (typing in progress)
		if remainingLen < partLen {
			if matchASCIIPrefix(patternRunes[patternPos:], part[:remainingLen]) {
				// Strict Mode Rule: If we skipped syllables, we cannot match partially
				if totalSkippedSyllables > 0 {
					goto NoMatch
				}

				// Check mixed mode
				if matchedSyllables > 0 && remainingLen == 1 {
					return FuzzyMatchResult{IsMatch: false, Score: 0}
				}
				patternPos += remainingLen
				matchedSyllables++
				lastMatchWasPartial = true
				partIdx++
				consecutiveSkipped = 0
				continue
			}
		}

	NoMatch:
		// Case 3: No match
		totalSkippedSyllables++
		consecutiveSkipped++
		partIdx++

		if matchedSyllables > 0 && consecutiveSkipped > maxConsecutiveSkippedSyllables {
			return FuzzyMatchResult{IsMatch: false, Score: 0}
		}
	}

	// Pattern must be fully consumed
	if patternPos != len(patternRunes) {
		return FuzzyMatchResult{IsMatch: false, Score: 0}
	}

	if matchedSyllables == 0 {
		return FuzzyMatchResult{IsMatch: false, Score: 0}
	}

	score := int64(matchedSyllables) * scoreMatch * 2
	if totalSkippedSyllables == 0 {
		score += bonusConsecutive * int64(matchedSyllables)
	}

	if !lastMatchWasPartial && partIdx == len(parts) && totalSkippedSyllables == 0 {
		score += bonusExactMatch
	}

	return FuzzyMatchResult{IsMatch: true, Score: score}
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

// containsRunesAll checks if all runes in chars are present in s (in order)
func containsRunesAll(s string, chars []rune) bool {
	textStr := s
	for _, pRune := range chars {
		found := false
		for i, r := range textStr {
			// Normalize rune on the fly
			r = unicode.ToLower(r)
			if normalized, ok := diacriticsMap[r]; ok {
				r = normalized
			}

			if r == pRune {
				found = true
				textStr = textStr[i+utf8.RuneLen(r):]
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
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
