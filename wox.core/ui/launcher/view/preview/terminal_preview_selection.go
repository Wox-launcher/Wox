package preview

import (
	"math"
	"strings"
	"unicode"
	"unicode/utf8"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const terminalOutputLineHeight = float32(18)

type terminalLineRange struct {
	text  string
	start int
	end   int
}

// TerminalOutputKey keeps output selection, copy, and the context menu on one retained identity.
func TerminalOutputKey(sessionID string) woxwidget.Key {
	return woxwidget.Key("terminal-preview-output-" + sessionID)
}

func terminalOutputSelectable(text string) bool {
	return strings.TrimSpace(text) != ""
}

// terminalLineRanges maps wrapped display lines back to the original output bytes.
func terminalLineRanges(value string, lines []string) []terminalLineRange {
	ranges := make([]terminalLineRange, 0, len(lines))
	cursor := 0
	for _, line := range lines {
		lineStart := cursor
		if line != "" {
			if offset := strings.Index(value[cursor:], line); offset >= 0 {
				lineStart = cursor + offset
			}
		}
		lineEnd := lineStart + len(line)
		ranges = append(ranges, terminalLineRange{text: line, start: lineStart, end: lineEnd})
		cursor = lineEnd
		// Consume one line break (including CRLF), preserving subsequent empty lines.
		if cursor < len(value) && value[cursor] == '\r' {
			cursor++
		}
		if cursor < len(value) && value[cursor] == '\n' {
			cursor++
		}
	}
	return ranges
}

// TerminalClampUTF8Offset snaps a byte offset onto a UTF-8 rune start.
func TerminalClampUTF8Offset(value string, offset int) int {
	if offset <= 0 {
		return 0
	}
	if offset >= len(value) {
		return len(value)
	}
	for offset > 0 && !utf8.RuneStart(value[offset]) {
		offset--
	}
	return offset
}

func terminalMeasureWidth(window *woxui.Window, text string, style woxui.TextStyle) float32 {
	if window != nil {
		if metrics, err := window.MeasureText(text, style); err == nil {
			return metrics.Size.Width
		}
	}
	return float32(utf8.RuneCountInString(text)) * max(style.Size/2, 1)
}

func terminalColumnAtX(line string, targetX float32, window *woxui.Window, style woxui.TextStyle) int {
	if line == "" || targetX <= 0 {
		return 0
	}
	spans := woxui.GraphemeSpans(line)
	if len(spans) == 0 {
		return 0
	}
	runes := []rune(line)
	bestOffset, bestDist := 0, float32(1e9)
	consider := func(byteOffset int, width float32) {
		dist := width - targetX
		if dist < 0 {
			dist = -dist
		}
		if dist < bestDist {
			bestDist = dist
			bestOffset = byteOffset
		}
	}
	consider(0, 0)
	for index := 1; index <= len(spans); index++ {
		prefix := string(runes[:spans[index-1].End])
		consider(len(prefix), terminalMeasureWidth(window, prefix, style))
	}
	return bestOffset
}

// terminalOffsetAt maps a content-local point onto a grapheme-safe byte offset in value.
func terminalOffsetAt(value string, layout woxwidget.TextBlockLayout, window *woxui.Window, style woxui.TextStyle, point woxui.Point, lineHeight float32) int {
	if lineHeight <= 0 {
		lineHeight = terminalOutputLineHeight
	}
	ranges := terminalLineRanges(value, layout.Lines)
	if len(ranges) == 0 {
		return 0
	}
	lineIndex := int(math.Floor(float64(point.Y / lineHeight)))
	if lineIndex < 0 {
		return 0
	}
	if lineIndex >= len(ranges) {
		return len(value)
	}
	current := ranges[lineIndex]
	return TerminalClampUTF8Offset(value, current.start+terminalColumnAtX(current.text, point.X, window, style))
}

func isTerminalWordRune(current rune) bool {
	return unicode.IsLetter(current) || unicode.IsDigit(current) || unicode.IsMark(current) || current == '_'
}

// TerminalWordByteRange selects the word containing byteOffset, matching text-field double-click.
func TerminalWordByteRange(value string, byteOffset int) (int, int) {
	byteOffset = TerminalClampUTF8Offset(value, byteOffset)
	runes := []rune(value)
	if len(runes) == 0 {
		return 0, 0
	}
	runeOffset := utf8.RuneCountInString(value[:byteOffset])
	if runeOffset >= len(runes) {
		runeOffset = len(runes) - 1
	}
	start, end := runeOffset, runeOffset+1
	if isTerminalWordRune(runes[runeOffset]) {
		for start > 0 && isTerminalWordRune(runes[start-1]) {
			start--
		}
		for end < len(runes) && isTerminalWordRune(runes[end]) {
			end++
		}
	}
	return len(string(runes[:start])), len(string(runes[:end]))
}

// TerminalLineByteRange selects the newline-delimited line containing byteOffset.
func TerminalLineByteRange(value string, byteOffset int) (int, int) {
	byteOffset = TerminalClampUTF8Offset(value, byteOffset)
	if byteOffset >= len(value) && len(value) > 0 {
		byteOffset = TerminalClampUTF8Offset(value, len(value)-1)
	}
	start := byteOffset
	for start > 0 && value[start-1] != '\n' {
		start--
	}
	end := byteOffset
	for end < len(value) && value[end] != '\n' {
		end++
	}
	return start, end
}

func terminalRuneSelection(value string, start, end int) (int, int) {
	start = TerminalClampUTF8Offset(value, start)
	end = TerminalClampUTF8Offset(value, end)
	if start > end {
		start, end = end, start
	}
	return utf8.RuneCountInString(value[:start]), utf8.RuneCountInString(value[:end])
}
