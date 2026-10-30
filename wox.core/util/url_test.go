package util

import (
	"strings"
	"testing"
)

// TestIsUrlPreservesRegexpSemantics covers the fast rejection of long non-URLs.
func TestIsUrlPreservesRegexpSemantics(t *testing.T) {
	for _, value := range []string{
		"example.com", " https://example.com/path ", "127.0.0.1:8080/path", "localhost",
		"http://localhost/path.with.dot", "hello world. example", "example.com/a\nb",
		strings.Repeat("a", 128*1024), strings.Repeat("a", 128*1024) + ".com",
	} {
		want := urlPattern.MatchString(strings.TrimSpace(value))
		if got := IsUrl(value); got != want {
			t.Fatalf("IsUrl differs from the original regexp for input of length %d: got %v, want %v", len(value), got, want)
		}
	}
}
