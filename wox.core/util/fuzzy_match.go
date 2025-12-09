package util

import (
	"strings"
	"unicode"
)

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

	// Normalize both strings (lowercase + remove diacritics)
	normalizedText := normalizeString(text)
	normalizedPattern := normalizeString(pattern)

	// Try exact match first (highest priority)
	if normalizedText == normalizedPattern {
		return FuzzyMatchResult{IsMatch: true, Score: bonusExactMatch + int64(len(pattern)*scoreMatch)}
	}

	// Try prefix match (high priority)
	if strings.HasPrefix(normalizedText, normalizedPattern) {
		patternRunes := []rune(pattern)
		score := bonusPrefixMatch + int64(len(patternRunes)*scoreMatch) + bonusFirstCharMatch
		return FuzzyMatchResult{IsMatch: true, Score: score}
	}

	// Try fuzzy match on the original text
	result := fuzzyMatchCore(text, normalizedText, normalizedPattern)
	if result.IsMatch {
		return result
	}

	// Try pinyin matching for Chinese text
	// Only allow strict matching: all first letters OR all full pinyin
	if usePinYin && hasChinese(text) {
		pinyinResult := matchPinyinStrict(text, normalizedPattern)
		if pinyinResult.IsMatch {
			return pinyinResult
		}
	}

	// Fallback: substring match (lower score)
	if strings.Contains(normalizedText, normalizedPattern) {
		patternRunes := []rune(pattern)
		// Lower score for non-prefix substring matches
		score := int64(len(patternRunes))
		return FuzzyMatchResult{IsMatch: true, Score: score}
	}

	return FuzzyMatchResult{IsMatch: false, Score: 0}
}

// matchPinyinStrict performs strict pinyin matching
// Only allows: all first letters (e.g., "nh" for "你好") OR all full pinyin (e.g., "nihao" for "你好")
// Does NOT allow mixed mode (e.g., "nhao" or "nih")
func matchPinyinStrict(text string, normalizedPattern string) FuzzyMatchResult {
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

		// Build first letters string from filtered parts
		var firstLetters strings.Builder
		for _, part := range pinyinParts {
			firstLetters.WriteByte(part[0])
		}
		firstLettersStr := strings.ToLower(firstLetters.String())

		// Check 1: Exact first letters match (e.g., "nh" matches "你好")
		if normalizedPattern == firstLettersStr {
			score := bonusExactMatch + int64(len(normalizedPattern)*scoreMatch)
			if score > bestResult.Score {
				bestResult = FuzzyMatchResult{IsMatch: true, Score: score}
			}
			continue
		}

		// Check 2: First letters prefix match (e.g., "n" matches "你好")
		if strings.HasPrefix(firstLettersStr, normalizedPattern) {
			score := bonusPrefixMatch + int64(len(normalizedPattern)*scoreMatch) + bonusFirstCharMatch
			if score > bestResult.Score {
				bestResult = FuzzyMatchResult{IsMatch: true, Score: score}
			}
			continue
		}

		// Check 3: Syllable-level matching (covers exact match, prefix match, and subsequence match)
		// - Exact match: "nihao" matches "你好" (all syllables matched completely)
		// - Prefix match: "niha" matches "你好" (last syllable partial, typing in progress)
		// - Subsequence: "wangyiyinyue" matches "网易云音乐" (can skip syllables)
		// - Mixed mode rejected: "nih" does NOT match (ni=full + h=first letter)
		if syllableResult := matchSyllables(pinyinParts, normalizedPattern); syllableResult.IsMatch {
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
// Supports: exact match, prefix match (typing in progress), and subsequence match (can skip syllables)
// Rejects mixed mode: full pinyin + first letter combination (e.g., "nih" for "你好")
// Rejects scattered matches: if consecutive skipped syllables exceed threshold
//
// Examples:
//   - "nihao" matches "你好" (exact match)
//   - "niha" matches "你好" (prefix match, typing "hao" in progress)
//   - "wangyiyinyue" matches "网易云音乐" (subsequence, skipping 1 syllable "yun")
//   - "nih" does NOT match "你好" (mixed mode: ni=full + h=first letter)
//   - "daoyan" does NOT match "J道:解惑授道-国际软件架构前沿" (too many skipped syllables)
func matchSyllables(parts []string, pattern string) FuzzyMatchResult {
	if len(pattern) == 0 || len(parts) == 0 {
		return FuzzyMatchResult{IsMatch: false, Score: 0}
	}

	patternPos := 0
	partIdx := 0
	matchedSyllables := 0
	totalSkippedSyllables := 0
	consecutiveSkipped := 0 // Track consecutive skipped syllables
	lastMatchWasPartial := false

	for patternPos < len(pattern) && partIdx < len(parts) {
		partLower := strings.ToLower(parts[partIdx])
		remaining := pattern[patternPos:]

		// Case 1: Remaining starts with full syllable - consume it
		if strings.HasPrefix(remaining, partLower) {
			patternPos += len(partLower)
			matchedSyllables++
			partIdx++
			lastMatchWasPartial = false
			consecutiveSkipped = 0 // Reset consecutive counter on match
			continue
		}

		// Case 2: Remaining is a prefix of this syllable (typing in progress)
		if strings.HasPrefix(partLower, remaining) {
			// Check for mixed mode: if we've matched full syllables before,
			// and remaining is EXACTLY the first letter, this is mixed mode
			if matchedSyllables > 0 && len(remaining) == 1 {
				// Mixed mode detected - reject
				return FuzzyMatchResult{IsMatch: false, Score: 0}
			}
			// Valid partial match (typing in progress)
			patternPos += len(remaining)
			matchedSyllables++
			lastMatchWasPartial = true
			partIdx++
			consecutiveSkipped = 0 // Reset consecutive counter on match
			continue
		}

		// Case 3: No match - skip this syllable and try next
		totalSkippedSyllables++
		consecutiveSkipped++
		partIdx++

		// Check if we've skipped too many consecutive syllables
		// Only check after we've matched at least one syllable
		if matchedSyllables > 0 && consecutiveSkipped > maxConsecutiveSkippedSyllables {
			// Too many consecutive skips - scattered match, reject
			return FuzzyMatchResult{IsMatch: false, Score: 0}
		}
	}

	// Pattern must be fully consumed
	if patternPos != len(pattern) {
		return FuzzyMatchResult{IsMatch: false, Score: 0}
	}

	// Must have matched at least one syllable
	if matchedSyllables == 0 {
		return FuzzyMatchResult{IsMatch: false, Score: 0}
	}

	// Calculate score
	score := int64(matchedSyllables) * scoreMatch * 2

	// Bonus for no skipped syllables (consecutive match)
	if totalSkippedSyllables == 0 {
		score += bonusConsecutive * int64(matchedSyllables)
	}

	// Bonus for exact match (all syllables matched, no partial)
	if !lastMatchWasPartial && partIdx == len(parts) && totalSkippedSyllables == 0 {
		score += bonusExactMatch
	}

	return FuzzyMatchResult{IsMatch: true, Score: score}
}

// fuzzyMatchCore performs the core fuzzy matching algorithm
func fuzzyMatchCore(originalText string, normalizedText string, normalizedPattern string) FuzzyMatchResult {
	textRunes := []rune(normalizedText)
	patternRunes := []rune(normalizedPattern)
	originalRunes := []rune(originalText)

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
		if equalRune(textRunes[textIdx], patternRunes[patternIdx]) {
			patternIdx++
		}
	}

	// If not all pattern characters were found, no match
	if patternIdx != patternLen {
		return FuzzyMatchResult{IsMatch: false, Score: 0}
	}

	// Second pass: find optimal match positions using greedy algorithm with scoring
	// Try to find better match positions (prefer boundaries, consecutive matches)
	matchedPositions := optimizeMatchPositions(originalRunes, textRunes, patternRunes)

	// Calculate final score
	score := calculateScore(originalRunes, textRunes, matchedPositions, patternLen)

	// Apply minimum score threshold to filter out low-quality matches
	// The threshold is dynamic based on pattern length
	minScore := calculateMinScoreThreshold(patternLen, textLen)
	if score < minScore {
		return FuzzyMatchResult{IsMatch: false, Score: 0}
	}

	return FuzzyMatchResult{IsMatch: true, Score: score}
}

// optimizeMatchPositions finds optimal positions for pattern characters in text
func optimizeMatchPositions(originalRunes []rune, textRunes []rune, patternRunes []rune) []int {
	textLen := len(textRunes)
	patternLen := len(patternRunes)
	matchedIndexes := make([]int, patternLen)

	// Use a greedy approach with lookahead to prefer better match positions
	patternIdx := 0
	for textIdx := 0; textIdx < textLen && patternIdx < patternLen; textIdx++ {
		if !equalRune(textRunes[textIdx], patternRunes[patternIdx]) {
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
			if equalRune(textRunes[lookAhead], patternRunes[patternIdx]) {
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
			if equalRune(textRunes[textIdx], patternRunes[patternIdx]) {
				matchedIndexes[patternIdx] = textIdx
				patternIdx++
			}
		}
	}

	return matchedIndexes
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

// normalizeString converts string to lowercase and removes diacritics
func normalizeString(s string) string {
	var result strings.Builder
	result.Grow(len(s))

	for _, r := range s {
		// Convert to lowercase
		r = unicode.ToLower(r)

		// Remove diacritics by mapping to base character
		if normalized, ok := diacriticsMap[r]; ok {
			result.WriteRune(normalized)
		} else {
			result.WriteRune(r)
		}
	}

	return result.String()
}

// equalRune compares two runes for equality (case-insensitive, diacritics-insensitive)
func equalRune(a, b rune) bool {
	if a == b {
		return true
	}

	// Normalize both runes
	a = unicode.ToLower(a)
	b = unicode.ToLower(b)

	if a == b {
		return true
	}

	// Check diacritics mapping
	if normalizedA, ok := diacriticsMap[a]; ok {
		a = normalizedA
	}
	if normalizedB, ok := diacriticsMap[b]; ok {
		b = normalizedB
	}

	return a == b
}

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
