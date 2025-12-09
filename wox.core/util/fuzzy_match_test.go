package util

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
	assert.True(t, IsStringMatch("微信", "weixin", true))
	assert.True(t, IsStringMatch("支付宝", "zhifubao", true))

	// First letter pinyin match
	assert.True(t, IsStringMatch("微信", "wx", true))
	assert.True(t, IsStringMatch("支付宝", "zfb", true))

	// Partial pinyin match
	assert.True(t, IsStringMatch("网易云音乐", "wangyiyun", true))
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

func TestFuzzyMatchPerformance(t *testing.T) {
	// Long strings should complete in reasonable time
	longText := "This is a very long text that contains many words and should still be matched quickly even though it has a lot of characters in it and we want to make sure the algorithm is efficient"

	start := GetSystemTimestamp()
	for i := 0; i < 1000; i++ {
		FuzzyMatch(longText, "quickly", false)
	}
	elapsed := GetSystemTimestamp() - start
	assert.Less(t, elapsed, int64(1000)) // Should complete 1000 iterations in less than 1 second
}

// Tests migrated from strings_test.go

func TestStringMatcherPinyin(t *testing.T) {
	// All first letters match
	assert.True(t, IsStringMatch("有道词典", "yd", true))
	assert.True(t, IsStringMatch("有道词典", "ydcd", true))
	assert.True(t, IsStringMatch("网易云音乐", "wyyy", true))
	assert.True(t, IsStringMatch("腾讯qq", "tx", true))
	assert.True(t, IsStringMatch("你好", "nh", true))
	assert.True(t, IsStringMatch("你好", "n", true))

	// All full pinyin match
	assert.True(t, IsStringMatch("QQ音乐.app", "yinyue", true), "QQ音乐.app should match yinyue")
	assert.True(t, IsStringMatch("你好", "nihao", true))
	assert.True(t, IsStringMatch("你好", "ni", true))
	assert.True(t, IsStringMatch("你好", "niha", true))
	assert.True(t, IsStringMatch("网易云音乐", "wangyiyinyue", true))

	// Mixed mode should NOT match (first letter + partial pinyin)
	assert.False(t, IsStringMatch("你好", "nhao", true), "你好 should NOT match nhao (mixed mode)")
	assert.False(t, IsStringMatch("你好", "nih", true), "你好 should NOT match nih (mixed mode)")

	// Partial full pinyin IS allowed (typing in progress)
	assert.True(t, IsStringMatch("有道词典", "youdao", true)) // covers first 2 chars fully in pinyin mode

	// Scattered syllables should NOT match (too many skipped syllables between matches)
	assert.False(t, IsStringMatch("J道:解惑授道-国际软件架构前沿", "daoyan", true), "scattered pinyin should not match")

	// Non-Chinese text should not use pinyin matching
	assert.False(t, IsStringMatch("Microsoft Remote Desktop", "test", true))
}

func TestStringMatcher(t *testing.T) {
	testcase(t, "OverLeaf-Latex: An online LaTeX editor", "exce", false)
	testcase(t, "Windows Terminal", "term", true)
	testcase(t, "Microsoft SQL Server Management Studio", "mssms", true)
}

func testcase(t *testing.T, term string, search string, expected bool) {
	assert.Equal(t, expected, IsStringMatch(term, search, false))
}

func TestIsStringMatchScore(t *testing.T) {
	match, score := IsStringMatchScore("有道词典", "有", true)
	assert.True(t, match)
	assert.GreaterOrEqual(t, score, int64(1))

	match, score = IsStringMatchScore("Share with AirDrop", "air", true)
	assert.True(t, match)
	assert.GreaterOrEqual(t, score, int64(1))
}

func TestIsStringMatchScoreLong(t *testing.T) {
	start := GetSystemTimestamp()
	longText := `X 上的 Johnny Bi："好多推友关注清迈的物价，刚好今天和老婆去超市，随手拍了一些价格，给小伙伴们分享一下。 今天去的是Makro，是杭东这边比较大的超市，也是我们最经常去的超市，价格一般，比BigC便宜，但是和各种市场比起来偏贵。… https:/2OP" / X htt198644`
	IsStringMatchScore(longText, "github", true)
	elapsed := GetSystemTimestamp() - start
	assert.Less(t, elapsed, int64(1000))
}
