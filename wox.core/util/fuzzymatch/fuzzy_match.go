package fuzzymatch

import (
	"strings"
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

	// Get buffer for pattern runes
	patternBufPtr := getRuneBuffer()
	defer putRuneBuffer(patternBufPtr)

	// Normalize pattern to runes
	// We can use the same processText logic but ignore original buffer and hasChinese?
	// Or just keep normalizeToRunes as a simple version for pattern.
	// Pattern is usually short, so existing normalizeToRunes is fine, but let's optimize it too with ASCII check.
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
		// Pass runes to strict pinyin matcher
		pinyinResult := matchPinyinStrict(text, patternRunes)
		if pinyinResult.IsMatch {
			return pinyinResult
		}
	}

	// Fallback: substring match (lower score)
	if containsRunes(textRunes, patternRunes) {
		// Lower score for non-prefix substring matches
		score := int64(len(patternRunes))
		return FuzzyMatchResult{IsMatch: true, Score: score}
	}

	return FuzzyMatchResult{IsMatch: false, Score: 0}
}

// matchPinyinStrict performs strict pinyin matching
// Only allows: all first letters (e.g., "nh" for "你好") OR all full pinyin (e.g., "nihao" for "你好")
// Does NOT allow mixed mode (e.g., "nhao" or "nih")
func matchPinyinStrict(text string, patternRunes []rune) FuzzyMatchResult {
	pinyinVariants := getPinYin(text)

	var bestResult FuzzyMatchResult

	for _, pinyinText := range pinyinVariants {
		parts := strings.Split(pinyinText, " ")

		// Filter parts to only include alphabetic pinyin (exclude non-letter characters like ".", "Q", etc.)
		var pinyinParts []string
		for _, part := range parts {
			// Check if part is purely alphabetic (pinyin)
			if len(part) > 0 && isAlphabeticPinyin(part) {
				pinyinParts = append(pinyinParts, part)
			}
		}

		if len(pinyinParts) == 0 {
			continue
		}

		// Build first letters buffer
		firstLettersBufPtr := getRuneBuffer()
		firstLettersBuf := *firstLettersBufPtr
		for _, part := range pinyinParts {
			firstLettersBuf = append(firstLettersBuf, rune(part[0]))
		}
		*firstLettersBufPtr = firstLettersBuf // Update pointer just in case append reallocs

		// Convert firstLetters to lowercase runes
		for i, r := range firstLettersBuf {
			firstLettersBuf[i] = unicode.ToLower(r)
		}

		// Check 1: Exact first letters match
		if equalRunes(patternRunes, firstLettersBuf) {
			score := bonusExactMatch + int64(len(patternRunes)*scoreMatch)
			if score > bestResult.Score {
				bestResult = FuzzyMatchResult{IsMatch: true, Score: score}
			}
			putRuneBuffer(firstLettersBufPtr)
			continue
		}

		// Check 2: First letters prefix match
		if hasPrefixRunes(firstLettersBuf, patternRunes) {
			score := bonusPrefixMatch + int64(len(patternRunes)*scoreMatch) + bonusFirstCharMatch
			if score > bestResult.Score {
				bestResult = FuzzyMatchResult{IsMatch: true, Score: score}
			}
			putRuneBuffer(firstLettersBufPtr)
			continue
		}
		putRuneBuffer(firstLettersBufPtr) // Done with this buffer

		// Check 3: Syllable-level matching
		if syllableResult := matchSyllables(pinyinParts, patternRunes); syllableResult.IsMatch {
			if syllableResult.Score > bestResult.Score {
				bestResult = syllableResult
			}
		}
	}

	return bestResult
}

// isAlphabeticPinyin checks if a string is a valid pinyin syllable
// Valid pinyin: all lowercase letters, length >= 1
// Single uppercase letters (like "Q") are NOT pinyin, they're original characters
func isAlphabeticPinyin(s string) bool {
	if len(s) == 0 {
		return false
	}

	hasLowercase := false
	for _, r := range s {
		if !unicode.IsLetter(r) {
			return false
		}
		if unicode.IsLower(r) {
			hasLowercase = true
		}
	}

	// Must have at least one lowercase letter to be considered pinyin
	// This filters out single uppercase chars like "Q" which are original text
	return hasLowercase
}

// Maximum allowed consecutive skipped syllables before rejecting match
// This prevents matching scattered syllables like "道"..."沿" in "J道:解惑授道-国际软件架构前沿"
const maxConsecutiveSkippedSyllables = 3

// matchSyllables performs unified syllable-level matching
func matchSyllables(parts []string, patternRunes []rune) FuzzyMatchResult {
	if len(patternRunes) == 0 || len(parts) == 0 {
		return FuzzyMatchResult{IsMatch: false, Score: 0}
	}

	// We work with runes now for pattern
	patternPos := 0
	partIdx := 0
	matchedSyllables := 0
	totalSkippedSyllables := 0
	consecutiveSkipped := 0
	lastMatchWasPartial := false

	for patternPos < len(patternRunes) && partIdx < len(parts) {
		partLower := strings.ToLower(parts[partIdx])
		partRunes := []rune(partLower) // Allocation here? "parts" are strings. optimizing this is next level.

		remainingLen := len(patternRunes) - patternPos
		// We need to compare runes.

		// Case 1: Remaining starts with full syllable
		if hasPrefixRunes(patternRunes[patternPos:], partRunes) {
			patternPos += len(partRunes)
			matchedSyllables++
			partIdx++
			lastMatchWasPartial = false
			consecutiveSkipped = 0
			continue
		}

		// Case 2: Remaining is a prefix of this syllable (typing in progress)
		// ...
		checkLen := remainingLen
		if checkLen > len(partRunes) {
			checkLen = len(partRunes)
		}

		if remainingLen <= len(partRunes) {
			if hasPrefixRunes(partRunes, patternRunes[patternPos:]) {
				// Strict Mode Rule: If we skipped syllables, we cannot match partially.
				// We must match the FULL syllable to allow jumping to it.
				// Exception: "h" matching "hao" (if "h" is valid Initial?).
				// But "h" match is handled via Initials check in matchPinyinStrict.
				// Here we are in Pinyin Sytllables check.
				// So "h" matching "nihao" (skip "ni", match "hao" partial) -> Should FAIL.
				// And "ha" matching "nihao" (skip "ni", match "hao" partial) -> Should FAIL.
				// "hao" matching "nihao" (skip "ni", match "hao" full) -> SHOULD PASS.

				if totalSkippedSyllables > 0 && remainingLen < len(partRunes) {
					// Skipping allowed only for full syllable matches
					// But wait, what if pattern is "haox"?
					// If pattern is longer than part, we fall through to Case 3 (No match).
					// Case 2 is strictly "Remaining <= Part".
					// So if Remaining < Part, it's a Partial Match.
					// If TotalSkipped > 0, REJECT Partial Match.
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

	// ... same scoring logic ...
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

// fuzzyMatchCore performs the core fuzzy matching algorithm
func fuzzyMatchCore(originalRunes []rune, textRunes []rune, patternRunes []rune) FuzzyMatchResult {
	textLen := len(textRunes)
	patternLen := len(patternRunes)

	if patternLen == 0 {
		return FuzzyMatchResult{IsMatch: true, Score: 0}
	}
	if textLen == 0 || patternLen > textLen {
		return FuzzyMatchResult{IsMatch: false, Score: 0}
	}

	// First pass: check if all pattern characters exist in text (in order)
	patternIdx := 0
	for textIdx := 0; textIdx < textLen && patternIdx < patternLen; textIdx++ {
		if textRunes[textIdx] == patternRunes[patternIdx] {
			patternIdx++
		}
	}

	// If not all pattern characters were found, no match
	if patternIdx != patternLen {
		return FuzzyMatchResult{IsMatch: false, Score: 0}
	}

	// Use pooled buffer for matchedIndexes
	matchedIndexesPtr := getIntBuffer()
	matchedIndexes := *matchedIndexesPtr
	// Ensure we have space for patternLen
	if cap(matchedIndexes) < patternLen {
		// Pool gave us something too small, allocate proper size
		// We don't update *matchedIndexesPtr here because we don't want to put back this new slice if it's too big/new?
		// Or we DO want to upgrade the pool?
		// Let's allocate new slice and put it back later.
		matchedIndexes = make([]int, patternLen)
	} else {
		matchedIndexes = matchedIndexes[:patternLen]
	}
	// Important: update pointer so deferred Put sees the slice we are using (including if we grew it)
	*matchedIndexesPtr = matchedIndexes
	defer putIntBuffer(matchedIndexesPtr)

	// Also optimizeMatchPositions needs to know it should fill this slice
	optimizeMatchPositions(originalRunes, textRunes, patternRunes, matchedIndexes)

	// Calculate final score
	score := calculateScore(originalRunes, textRunes, matchedIndexes, patternLen)

	// Apply minimum score threshold to filter out low-quality matches
	minScore := calculateMinScoreThreshold(patternLen, textLen)
	if score < minScore {
		return FuzzyMatchResult{IsMatch: false, Score: 0}
	}

	return FuzzyMatchResult{IsMatch: true, Score: score}
}

// optimizeMatchPositions finds optimal positions for pattern characters in text
func optimizeMatchPositions(originalRunes []rune, textRunes []rune, patternRunes []rune, matchedIndexes []int) {
	textLen := len(textRunes)
	patternLen := len(patternRunes)
	// matchedIndexes is assumed to be of size patternLen

	// Use a greedy approach with lookahead to prefer better match positions
	patternIdx := 0
	for textIdx := 0; textIdx < textLen && patternIdx < patternLen; textIdx++ {
		if textRunes[textIdx] != patternRunes[patternIdx] {
			continue
		}

		// Check if this is a good position to match
		isBoundary := textIdx == 0 || isBoundaryChar(originalRunes, textIdx)
		isConsecutive := patternIdx > 0 && matchedIndexes[patternIdx-1] == textIdx-1

		// If this is a boundary or consecutive match, take it immediately
		if isBoundary || isConsecutive {
			matchedIndexes[patternIdx] = textIdx
			patternIdx++
			continue
		}

		// Look ahead to see if there's a better position
		foundBetter := false
		for lookAhead := textIdx + 1; lookAhead < textLen && lookAhead < textIdx+10; lookAhead++ {
			if textRunes[lookAhead] == patternRunes[patternIdx] {
				if isBoundaryChar(originalRunes, lookAhead) {
					// Found a boundary match ahead, skip current position
					foundBetter = true
					break
				}
			}
		}

		if !foundBetter {
			matchedIndexes[patternIdx] = textIdx
			patternIdx++
		}
	}

	// If optimization failed, fall back to simple sequential matching
	if patternIdx != patternLen {
		patternIdx = 0
		for textIdx := 0; textIdx < textLen && patternIdx < patternLen; textIdx++ {
			if textRunes[textIdx] == patternRunes[patternIdx] {
				matchedIndexes[patternIdx] = textIdx
				patternIdx++
			}
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

		// ASCII fast path
		if r < 128 {
			if 'A' <= r && r <= 'Z' {
				r += 'a' - 'A'
			}
			norm = append(norm, r)
			continue
		}

		if unicode.Is(unicode.Han, r) {
			hasChinese = true
		}

		// Convert to lowercase
		r = unicode.ToLower(r)

		// Remove diacritics by mapping to base character
		if normalized, ok := diacriticsMap[r]; ok {
			norm = append(norm, normalized)
		} else {
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
	delimiters := []rune{' ', '-', '_', '.', '/', '\\', ':', ',', ';', '(', ')', '[', ']', '{', '}'}
	for _, d := range delimiters {
		if r == d {
			return true
		}
	}
	return false
}
