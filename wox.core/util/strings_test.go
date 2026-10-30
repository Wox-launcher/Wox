package util

import (
	"strings"
	"testing"
)

// TestEllipsisMiddlePreservesRuneBoundaries compares the bounded allocation path with the original behavior.
func TestEllipsisMiddlePreservesRuneBoundaries(t *testing.T) {
	for _, value := range []string{"", "short", strings.Repeat("abcDEF", 20), strings.Repeat("中文🙂é", 20), "\xff" + strings.Repeat("内容", 20)} {
		for limit := 4; limit <= 35; limit++ {
			runes := []rune(value)
			want := value
			if len(runes) > limit {
				half := limit / 2
				want = string(runes[:half-1]) + "..." + string(runes[len(runes)-half+2:])
			}
			if got := EllipsisMiddle(value, limit); got != want {
				t.Fatalf("limit %d: got %q, want %q", limit, got, want)
			}
		}
	}
}
