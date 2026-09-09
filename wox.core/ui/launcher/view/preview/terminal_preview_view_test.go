package preview

import (
	"reflect"
	"testing"
)

// TestTerminalHighlightSegments preserves byte offsets across wrapped lines and skips inactive search.
func TestTerminalHighlightSegments(t *testing.T) {
	value := "first\n中文 match\nlast"
	lines := []string{"first", "中文", "match", "last"}
	if got := terminalHighlightSegments(value, lines, nil); got != nil {
		t.Fatalf("inactive search prepared highlights: %#v", got)
	}
	got := terminalHighlightSegments(value, lines, []TerminalMatch{{Start: 6, End: 18}})
	want := []terminalHighlightSegment{{line: 1, start: 0, end: 6}, {line: 2, start: 0, end: 5}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("highlight segments = %#v, want %#v", got, want)
	}
}
