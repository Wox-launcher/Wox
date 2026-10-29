package component

import (
	"strings"

	woxui "wox/ui/runtime"
)

// TextFieldFind paints search results independently of the editing selection.
type TextFieldFind struct {
	Query  string
	Active int // Rune offset of the current match, or -1 for other fields.
}

// NoteFindTarget addresses one text segment or table cell without moving editor focus.
type NoteFindTarget struct{ Block, Row, Column, Offset int }

func noteFieldFind(query string, target *NoteFindTarget, block, row, column int) *TextFieldFind {
	active := -1
	if target != nil && target.Block == block && target.Row == row && target.Column == column {
		active = target.Offset
	}
	return &TextFieldFind{Query: query, Active: active}
}

// FindTextMatches returns non-overlapping literal matches using rune offsets.
func FindTextMatches(text, query string) []woxui.TextSelection {
	if query == "" {
		return nil
	}
	source, needle := []rune(text), []rune(query)
	var matches []woxui.TextSelection
	for i := 0; i+len(needle) <= len(source); {
		if strings.EqualFold(string(source[i:i+len(needle)]), query) {
			matches = append(matches, woxui.TextSelection{Anchor: i, Focus: i + len(needle)})
			i += len(needle)
		} else {
			i++
		}
	}
	return matches
}
