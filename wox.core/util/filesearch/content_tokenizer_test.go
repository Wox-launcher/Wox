package filesearch

import (
	"strings"
	"testing"
)

func TestTokenizeForContentIndexEmpty(t *testing.T) {
	if got := TokenizeForContentIndex(""); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestTokenizeForContentIndexLatin(t *testing.T) {
	got := TokenizeForContentIndex("hello world foo")
	want := "hello world foo"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestTokenizeForContentIndexLatinMinChars(t *testing.T) {
	// Single-char tokens dropped for Latin.
	got := TokenizeForContentIndex("a go is fun")
	want := "go is fun"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestTokenizeForContentIndexCamelCase(t *testing.T) {
	got := TokenizeForContentIndex("getUserById")
	want := "get user by id"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestTokenizeForContentIndexSnakeCase(t *testing.T) {
	got := TokenizeForContentIndex("my_var_name")
	want := "my var name"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestTokenizeForContentIndexCJK(t *testing.T) {
	got := TokenizeForContentIndex("区块链")
	// unigrams: 区, 块, 链; bigrams: 区块, 块链
	// Order: 区, 区块, 块, 块链, 链
	want := "区 区块 块 块链 链"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestTokenizeForContentIndexCJKStoplist(t *testing.T) {
	// "的了" → both in stop-list → no unigrams, only bigram "的了"
	got := TokenizeForContentIndex("的了")
	want := "的了"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestTokenizeForContentIndexCJKFiveChars(t *testing.T) {
	got := TokenizeForContentIndex("区块链技术")
	// unigrams: 区, 块, 链, 技, 术
	// bigrams: 区块, 块链, 链技, 技术
	// emitCJKTokens order: for each position i: unigram[i], bigram[i:i+2]
	// i=0: 区, 区块; i=1: 块, 块链; i=2: 链, 链技; i=3: 技, 技术; i=4: 术
	want := "区 区块 块 块链 链 链技 技 技术 术"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestTokenizeForContentIndexMixed(t *testing.T) {
	got := TokenizeForContentIndex("hello 世界 test")
	// CJK: 世, 界, 世界
	// Latin: hello, test
	want := "hello 世 世界 界 test"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestTokenizeForContentIndexMaxBytes(t *testing.T) {
	// 65-byte token should be dropped.
	long := strings.Repeat("a", 65)
	got := TokenizeForContentIndex(long)
	if got != "" {
		t.Errorf("expected empty for 65-byte token, got %q", got)
	}

	// 64-byte token should pass.
	ok := strings.Repeat("a", 64)
	got = TokenizeForContentIndex(ok)
	if got != ok {
		t.Errorf("got %q, want %q", got, ok)
	}
}

func TestContentDefaultExtensions(t *testing.T) {
	exts := ContentDefaultExtensions()
	if len(exts) == 0 {
		t.Fatal("expected non-empty default extensions")
	}
	found := false
	for _, e := range exts {
		if e == "go" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'go' in default extensions")
	}
}

func TestContentExtensionsFromList(t *testing.T) {
	exts := ContentExtensionsFromList([]string{"txt", ".go", "MD", ""})
	if !exts["txt"] {
		t.Error("expected txt")
	}
	if !exts["go"] {
		t.Error("expected go")
	}
	if !exts["md"] {
		t.Error("expected md (lowercased)")
	}
	if len(exts) != 3 {
		t.Errorf("expected 3, got %d", len(exts))
	}
}

func TestIsContentSearchableExtension(t *testing.T) {
	exts := ContentExtensionsFromList([]string{"txt", "go", "md"})

	tests := []struct {
		path string
		want bool
	}{
		{"readme.txt", true},
		{"main.go", true},
		{"notes.md", true},
		{"image.png", false},
		{"noext", false},
		{".gitignore", false},
		{"readme.TXT", true}, // case-insensitive
	}

	for _, tt := range tests {
		got := IsContentSearchableExtension(tt.path, exts)
		if got != tt.want {
			t.Errorf("IsContentSearchableExtension(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestIsCJKStoplistUnigram(t *testing.T) {
	if !IsCJKStoplistUnigram('的') {
		t.Error("的 should be in stop-list")
	}
	if IsCJKStoplistUnigram('区') {
		t.Error("区 should not be in stop-list")
	}
}