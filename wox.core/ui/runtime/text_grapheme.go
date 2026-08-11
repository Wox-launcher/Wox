package woxui

import (
	"unicode"

	"github.com/rivo/uniseg"
)

// graphemeBoundaries returns rune offsets at each user-perceived character start,
// including a trailing sentinel equal to the rune count.
func graphemeBoundaries(text string) []int {
	runes := []rune(text)
	if len(runes) == 0 {
		return []int{0}
	}
	boundaries := make([]int, 0, len(runes)+1)
	boundaries = append(boundaries, 0)
	gr := uniseg.NewGraphemes(text)
	offset := 0
	for gr.Next() {
		offset += len([]rune(gr.Str()))
		boundaries = append(boundaries, offset)
	}
	if boundaries[len(boundaries)-1] != len(runes) {
		boundaries = append(boundaries, len(runes))
	}
	return boundaries
}

// snapGraphemeBoundary clamps offset onto a grapheme boundary.
// When offset falls inside a cluster, biasBackward chooses the start; otherwise the end.
func snapGraphemeBoundary(boundaries []int, offset int, biasBackward bool) int {
	if len(boundaries) == 0 {
		return 0
	}
	offset = max(0, min(boundaries[len(boundaries)-1], offset))
	for index := 0; index < len(boundaries)-1; index++ {
		start, end := boundaries[index], boundaries[index+1]
		if offset == start || offset == end {
			return offset
		}
		if offset > start && offset < end {
			if biasBackward {
				return start
			}
			return end
		}
	}
	return offset
}

// previousGraphemeBoundary moves one user-perceived character before offset.
func previousGraphemeBoundary(text string, offset int) int {
	boundaries := graphemeBoundaries(text)
	offset = snapGraphemeBoundary(boundaries, offset, true)
	for index := len(boundaries) - 1; index >= 0; index-- {
		if boundaries[index] < offset {
			return boundaries[index]
		}
	}
	return 0
}

// nextGraphemeBoundary moves one user-perceived character after offset.
func nextGraphemeBoundary(text string, offset int) int {
	boundaries := graphemeBoundaries(text)
	offset = snapGraphemeBoundary(boundaries, offset, false)
	for _, boundary := range boundaries {
		if boundary > offset {
			return boundary
		}
	}
	return boundaries[len(boundaries)-1]
}

// graphemeCount returns the number of user-perceived characters in text.
func graphemeCount(text string) int {
	count := 0
	gr := uniseg.NewGraphemes(text)
	for gr.Next() {
		count++
	}
	return count
}

// runeOffsetToGraphemeIndex converts a rune caret offset into a grapheme index.
func runeOffsetToGraphemeIndex(text string, offset int) int {
	boundaries := graphemeBoundaries(text)
	offset = snapGraphemeBoundary(boundaries, offset, true)
	for index, boundary := range boundaries {
		if boundary >= offset {
			return index
		}
	}
	return len(boundaries) - 1
}

// graphemeIndexToRuneOffset converts a grapheme index into a rune caret offset.
func graphemeIndexToRuneOffset(text string, index int) int {
	boundaries := graphemeBoundaries(text)
	if index <= 0 {
		return 0
	}
	if index >= len(boundaries) {
		return boundaries[len(boundaries)-1]
	}
	return boundaries[index]
}

// wordBoundaryOffsets returns UAX #29 word-boundary rune offsets, including 0 and len(runes).
// Every offset is snapped onto a grapheme boundary so navigation never lands mid-cluster.
func wordBoundaryOffsets(text string) []int {
	boundaries := graphemeBoundaries(text)
	if len(boundaries) <= 1 {
		return []int{0}
	}
	offsets := make([]int, 0, len(boundaries))
	offsets = append(offsets, 0)
	rest := text
	state := -1
	runeOffset := 0
	for rest != "" {
		word, next, newState := uniseg.FirstWordInString(rest, state)
		runeOffset += len([]rune(word))
		runeOffset = snapGraphemeBoundary(boundaries, runeOffset, true)
		if runeOffset > offsets[len(offsets)-1] {
			offsets = append(offsets, runeOffset)
		}
		rest = next
		state = newState
	}
	end := boundaries[len(boundaries)-1]
	if offsets[len(offsets)-1] != end {
		offsets = append(offsets, end)
	}
	return offsets
}

type wordSegment struct {
	start, end int
	text       string
}

// wordSegments splits text into UAX #29 word segments with absolute rune offsets.
func wordSegments(text string) []wordSegment {
	offsets := wordBoundaryOffsets(text)
	runes := []rune(text)
	if len(offsets) <= 1 {
		return nil
	}
	segments := make([]wordSegment, 0, len(offsets)-1)
	for index := 0; index < len(offsets)-1; index++ {
		start, end := offsets[index], offsets[index+1]
		segments = append(segments, wordSegment{start: start, end: end, text: string(runes[start:end])})
	}
	return segments
}

// isWhitespaceWordSegment reports whether a UAX segment is only whitespace (the desktop
// "separator" class used when expanding word delete/move beyond raw adjacent boundaries).
func isWhitespaceWordSegment(text string) bool {
	if text == "" {
		return true
	}
	for _, current := range text {
		if !unicode.IsSpace(current) {
			return false
		}
	}
	return true
}

func segmentIndexContaining(segments []wordSegment, offset int) int {
	for index, segment := range segments {
		if offset >= segment.start && offset < segment.end {
			return index
		}
	}
	return -1
}

func segmentIndexEndingAt(segments []wordSegment, offset int) int {
	for index := len(segments) - 1; index >= 0; index-- {
		if segments[index].end == offset {
			return index
		}
	}
	return -1
}

// wordDeleteBefore returns the start offset for a desktop-style word delete backward.
// Whitespace immediately before the caret is skipped so the previous non-whitespace word is included.
func wordDeleteBefore(text string, offset int) int {
	boundaries := graphemeBoundaries(text)
	offset = snapGraphemeBoundary(boundaries, offset, true)
	if offset == 0 {
		return 0
	}
	segments := wordSegments(text)
	if len(segments) == 0 {
		return 0
	}
	index := segmentIndexEndingAt(segments, offset)
	if index < 0 {
		index = segmentIndexContaining(segments, offset-1)
	}
	if index < 0 {
		return 0
	}
	start := segments[index].start
	if isWhitespaceWordSegment(segments[index].text) {
		for index > 0 && isWhitespaceWordSegment(segments[index].text) {
			index--
			start = segments[index].start
		}
	}
	return start
}

// wordDeleteAfter returns the end offset for a desktop-style word delete forward.
// Leading whitespace after the caret is included together with the following word.
func wordDeleteAfter(text string, offset int) int {
	boundaries := graphemeBoundaries(text)
	offset = snapGraphemeBoundary(boundaries, offset, false)
	end := boundaries[len(boundaries)-1]
	if offset >= end {
		return end
	}
	segments := wordSegments(text)
	if len(segments) == 0 {
		return end
	}
	index := segmentIndexContaining(segments, offset)
	if index < 0 {
		index = segmentIndexEndingAt(segments, offset)
		if index >= 0 && index+1 < len(segments) {
			index++
		}
	}
	if index < 0 || index >= len(segments) {
		return end
	}
	if isWhitespaceWordSegment(segments[index].text) {
		for index < len(segments) && isWhitespaceWordSegment(segments[index].text) {
			offset = segments[index].end
			index++
		}
		if index < len(segments) {
			return segments[index].end
		}
		return offset
	}
	return segments[index].end
}

// wordMoveBefore returns the caret offset for Ctrl/Option+Left word navigation.
func wordMoveBefore(text string, offset int) int {
	boundaries := graphemeBoundaries(text)
	offset = snapGraphemeBoundary(boundaries, offset, true)
	if offset == 0 {
		return 0
	}
	segments := wordSegments(text)
	if len(segments) == 0 {
		return 0
	}
	index := segmentIndexEndingAt(segments, offset)
	if index < 0 {
		index = segmentIndexContaining(segments, offset-1)
	}
	if index < 0 {
		return 0
	}
	// Mid-word (or standing on a word's end boundary): jump to that word's start.
	if !isWhitespaceWordSegment(segments[index].text) && offset > segments[index].start {
		return segments[index].start
	}
	for index >= 0 && isWhitespaceWordSegment(segments[index].text) {
		index--
	}
	if index < 0 {
		return 0
	}
	if offset <= segments[index].start {
		index--
		for index >= 0 && isWhitespaceWordSegment(segments[index].text) {
			index--
		}
		if index < 0 {
			return 0
		}
	}
	return segments[index].start
}

// wordMoveAfter returns the caret offset for Ctrl/Option+Right word navigation.
func wordMoveAfter(text string, offset int) int {
	boundaries := graphemeBoundaries(text)
	offset = snapGraphemeBoundary(boundaries, offset, false)
	end := boundaries[len(boundaries)-1]
	if offset >= end {
		return end
	}
	segments := wordSegments(text)
	if len(segments) == 0 {
		return end
	}
	index := segmentIndexContaining(segments, offset)
	if index < 0 {
		index = segmentIndexEndingAt(segments, offset)
		if index >= 0 && index+1 < len(segments) {
			index++
		}
	}
	if index < 0 || index >= len(segments) {
		return end
	}
	// Mid-word: jump to that word's end.
	if !isWhitespaceWordSegment(segments[index].text) && offset < segments[index].end {
		return segments[index].end
	}
	for index < len(segments) && (isWhitespaceWordSegment(segments[index].text) || segments[index].end <= offset) {
		index++
	}
	if index >= len(segments) {
		return end
	}
	return segments[index].end
}

// wordBoundaryBefore is kept as the delete-backward expansion used by word deletes.
func wordBoundaryBefore(text string, offset int) int {
	return wordDeleteBefore(text, offset)
}

// wordBoundaryAfter is kept as the delete-forward expansion used by word deletes.
func wordBoundaryAfter(text string, offset int) int {
	return wordDeleteAfter(text, offset)
}

// GraphemeSpan is one user-perceived character with absolute rune offsets.
type GraphemeSpan struct {
	Start int
	End   int
	Text  string
}

// GraphemeSpans returns contiguous grapheme spans for text.
func GraphemeSpans(text string) []GraphemeSpan {
	boundaries := graphemeBoundaries(text)
	runes := []rune(text)
	if len(boundaries) <= 1 {
		return nil
	}
	spans := make([]GraphemeSpan, 0, len(boundaries)-1)
	for index := 0; index < len(boundaries)-1; index++ {
		start, end := boundaries[index], boundaries[index+1]
		spans = append(spans, GraphemeSpan{Start: start, End: end, Text: string(runes[start:end])})
	}
	return spans
}

// graphemeUnits is kept for internal soft-wrap callers that still use the local alias.
func graphemeUnits(text string) []graphemeUnit {
	spans := GraphemeSpans(text)
	units := make([]graphemeUnit, len(spans))
	for index, span := range spans {
		units[index] = graphemeUnit{start: span.Start, end: span.End, text: span.Text}
	}
	return units
}

type graphemeUnit struct {
	start int
	end   int
	text  string
}

// isTextWordRune reports whether a rune participates in legacy word-style classification.
func isTextWordRune(current rune) bool {
	return unicode.IsLetter(current) || unicode.IsDigit(current) || unicode.IsMark(current) || current == '_'
}
