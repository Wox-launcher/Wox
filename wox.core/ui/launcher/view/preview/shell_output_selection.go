package preview

import (
	"unicode"
	"unicode/utf8"

	woxwidget "wox/ui/widget"
)

// TerminalOutputKey keeps output selection, copy, and the context menu on one retained identity.
func TerminalOutputKey(sessionID string) woxwidget.Key {
	return woxwidget.Key("terminal-preview-output-" + sessionID)
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
