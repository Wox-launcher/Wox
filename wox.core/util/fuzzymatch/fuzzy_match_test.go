package fuzzymatch

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFuzzyMatchExact(t *testing.T) {
	// Exact match should have highest score
	result := FuzzyMatch("Terminal", "Terminal", false)
	assert.True(t, result.IsMatch)
	assert.Greater(t, result.Score, int64(100))
}

func TestFuzzyMatchPrefix(t *testing.T) {
	// Prefix match should have high score
	result := FuzzyMatch("Terminal", "Term", false)
	assert.True(t, result.IsMatch)
	assert.Greater(t, result.Score, int64(50))

	// "term" should match "Terminal" (case insensitive)
	result = FuzzyMatch("Terminal", "term", false)
	assert.True(t, result.IsMatch)
}

func TestFuzzyMatchCamelCase(t *testing.T) {
	// CamelCase matching
	result := FuzzyMatch("moduleNameResolver", "mnr", false)
	assert.True(t, result.IsMatch)

	result = FuzzyMatch("getProcessById", "gpb", false)
	assert.True(t, result.IsMatch)

	result = FuzzyMatch("XMLHttpRequest", "xhr", false)
	assert.True(t, result.IsMatch)
}

func TestFuzzyMatchBoundary(t *testing.T) {
	// Boundary matching (after delimiters)
	result := FuzzyMatch("my-awesome-plugin", "map", false)
	assert.True(t, result.IsMatch)

	result = FuzzyMatch("user_profile_settings", "ups", false)
	assert.True(t, result.IsMatch)

	result = FuzzyMatch("file.name.extension", "fne", false)
	assert.True(t, result.IsMatch)
}

func TestFuzzyMatchDiacritics(t *testing.T) {
	// Diacritics should be normalized
	result := FuzzyMatch("café", "cafe", false)
	assert.True(t, result.IsMatch)

	result = FuzzyMatch("naïve", "naive", false)
	assert.True(t, result.IsMatch)

	result = FuzzyMatch("Müller", "muller", false)
	assert.True(t, result.IsMatch)

	result = FuzzyMatch("Björk", "bjork", false)
	assert.True(t, result.IsMatch)

	result = FuzzyMatch("São Paulo", "sao", false)
	assert.True(t, result.IsMatch)

	// Search with diacritics should also work
	result = FuzzyMatch("resume", "résumé", false)
	assert.True(t, result.IsMatch)
}

func TestFuzzyMatchNoMatch(t *testing.T) {
	// These should NOT match
	result := FuzzyMatch("Terminal", "xyz", false)
	assert.False(t, result.IsMatch)

	result = FuzzyMatch("hello", "world", false)
	assert.False(t, result.IsMatch)

	// Pattern longer than text
	result = FuzzyMatch("abc", "abcdef", false)
	assert.False(t, result.IsMatch)
}

func TestFuzzxyMatchPinyinPolyphonicCharacter(t *testing.T) {
	result := FuzzyMatch("这是一个多音字测试, 两行字", "hang", true)
	assert.True(t, result.IsMatch)

	result = FuzzyMatch("这是一个多音字测试, 行走", "xing", true)
	assert.True(t, result.IsMatch)
}

func TestFuzzyMatchScoreComparison(t *testing.T) {
	// Prefix match should score higher than substring match
	prefixResult := FuzzyMatch("Terminal", "term", false)
	substringResult := FuzzyMatch("myTerminal", "term", false)
	assert.Greater(t, prefixResult.Score, substringResult.Score)

	// Exact match should score higher than prefix match
	exactResult := FuzzyMatch("term", "term", false)
	assert.Greater(t, exactResult.Score, prefixResult.Score)

	// Consecutive matches should score higher than scattered matches
	consecutiveResult := FuzzyMatch("abcdef", "abc", false)
	scatteredResult := FuzzyMatch("aXbXcXdef", "abc", false)
	assert.Greater(t, consecutiveResult.Score, scatteredResult.Score)
}

func TestFuzzyMatchPinyinAdvanced(t *testing.T) {
	// Full pinyin match
	assert.True(t, FuzzyMatch("微信", "weixin", true).IsMatch)
	assert.True(t, FuzzyMatch("支付宝", "zhifubao", true).IsMatch)

	// First letter pinyin match
	assert.True(t, FuzzyMatch("微信", "wx", true).IsMatch)
	assert.True(t, FuzzyMatch("支付宝", "zfb", true).IsMatch)

	// Partial pinyin match
	assert.True(t, FuzzyMatch("网易云音乐", "wangyiyun", true).IsMatch)
}

func TestFuzzyMatchEdgeCases(t *testing.T) {
	// Empty pattern should match everything
	result := FuzzyMatch("anything", "", false)
	assert.True(t, result.IsMatch)

	// Empty text should not match non-empty pattern
	result = FuzzyMatch("", "abc", false)
	assert.False(t, result.IsMatch)

	// Both empty
	result = FuzzyMatch("", "", false)
	assert.True(t, result.IsMatch)

	// Single character match
	result = FuzzyMatch("a", "a", false)
	assert.True(t, result.IsMatch)

	// Unicode characters
	result = FuzzyMatch("日本語テスト", "日本", true)
	assert.True(t, result.IsMatch)
}

func TestFuzzyMatchSpecialCharacters(t *testing.T) {
	// Special characters in text
	result := FuzzyMatch("C++ Programming", "cpro", false)
	assert.True(t, result.IsMatch)

	result = FuzzyMatch("user@example.com", "user", false)
	assert.True(t, result.IsMatch)

	result = FuzzyMatch("path/to/file.txt", "ptf", false)
	assert.True(t, result.IsMatch)

	// Test that searching for actual content works
	result = FuzzyMatch("C++ Programming", "prog", false)
	assert.True(t, result.IsMatch)
}

func TestStringMatcherPinyin(t *testing.T) {
	// All first letters match
	assert.True(t, FuzzyMatch("有道词典", "yd", true).IsMatch)
	assert.True(t, FuzzyMatch("有道词典", "ydcd", true).IsMatch)
	assert.True(t, FuzzyMatch("网易云音乐", "wyyy", true).IsMatch)
	assert.True(t, FuzzyMatch("腾讯qq", "tx", true).IsMatch)
	assert.True(t, FuzzyMatch("你好", "nh", true).IsMatch)
	assert.True(t, FuzzyMatch("你好", "n", true).IsMatch)

	// All full pinyin match
	assert.True(t, FuzzyMatch("QQ音乐.app", "yinyue", true).IsMatch, "QQ音乐.app should match yinyue")
	assert.True(t, FuzzyMatch("你好", "nihao", true).IsMatch)
	assert.True(t, FuzzyMatch("你好", "ni", true).IsMatch)
	assert.True(t, FuzzyMatch("你好", "niha", true).IsMatch)
	assert.True(t, FuzzyMatch("网易云音乐", "wangyiyinyue", true).IsMatch)

	// Mixed mode should NOT match (first letter + partial pinyin)
	cases := []struct {
		pattern string
		text    string
		match   bool
	}{
		{"", "", true},
		{"", "a", true},
		{"a", "", false},
		{"a", "a", true},
		{"a", "b", false},
		{"nh", "你好", true},
		{"h", "你好", true},
		{"ha", "你好", false},
		{"ii", "Ii", true},
	}

	for _, c := range cases {
		result := FuzzyMatch(c.text, c.pattern, true)
		if result.IsMatch != c.match {
			t.Errorf("Test failed, pattern: %s, text: %s, expected: %v, got: %v", c.pattern, c.text, c.match, result.IsMatch)
		}
	}
}

func TestStringMatcher(t *testing.T) {

	cases := []struct {
		pattern string
		text    string
		match   bool
	}{
		{"", "", true},
		{"", "a", true},
		{"a", "", false},
		{"a", "a", true},
		{"a", "b", false},
	}

	for _, c := range cases {
		result := FuzzyMatch(c.text, c.pattern, false)
		if result.IsMatch != c.match {
			t.Errorf("Test failed, pattern: %s, text: %s, expected: %v, got: %v", c.pattern, c.text, c.match, result.IsMatch)
		}
	}
}

func TestIsStringMatchScore(t *testing.T) {
	cases := []struct {
		pattern string
		text    string
		match   bool
	}{
		{"", "", true},
		{"", "a", true},
		{"a", "", false},
		{"a", "a", true},
		{"a", "b", false},
	}

	for _, c := range cases {
		result := FuzzyMatch(c.text, c.pattern, false)
		if result.IsMatch != c.match {
			t.Errorf("Test failed, pattern: %s, text: %s, expected: %v, got: %v", c.pattern, c.text, c.match, result.IsMatch)
		}
	}
}

// cpu: Apple M1 Max
// BenchmarkIsStringMatchScore-10    	 1959001	       618.8 ns/op	       0 B/op	       0 allocs/op
func BenchmarkIsStringMatchScore(b *testing.B) {
	for i := 0; i < b.N; i++ {
		FuzzyMatch("刚好今天和老婆去超市 有道词典 Microsoft Word - Document.docx ", "超市", true)
	}
}

// cpu: Apple M1 Max
// BenchmarkFuzzyMatchNoMatch-10    	11877062	        97.95 ns/op	       0 B/op	       0 allocs/op
func BenchmarkFuzzyMatchNoMatch(b *testing.B) {
	// Scenario: Searching "git" in a long list of unrelated items
	// This should be zero allocations with the optimization
	text := "Microsoft Word - Document.docx - Final Version - 2024"
	pattern := "git"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		FuzzyMatch(text, pattern, true)
	}
}

// cpu: Apple M1 Max
// BenchmarkIsStringMatchScorePinyin-10    	  697035	      1687 ns/op	       0 B/op	       0 allocs/op
func BenchmarkIsStringMatchScorePinyin(b *testing.B) {
	// Scenario: Searching "yd" (YouDao) in the text, forcing Pinyin logic
	text := "刚好今天和老婆去超市 之前的优化中存在一个过于激进的策略：当句子超过 10 个字时，会强制丢弃多音字的变体以节省计算。这导致在长句中 \"行\" 只保留了 \"hang\" 而丢失了 \"xing\" 有道词典 Microsoft Word - Document.docx "
	pattern := "yd"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		FuzzyMatch(text, pattern, true)
	}
}
