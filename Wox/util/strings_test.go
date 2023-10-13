package util

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestStringMatcherPinyin(t *testing.T) {
	assert.True(t, StringMatch("有道词典", "yd", true))
	assert.True(t, StringMatch("网易云音乐", "yyy", true))
	assert.True(t, StringMatch("腾讯qq", "tx", true))
	assert.True(t, StringMatch("QQ音乐.app", "yinyue", true))
	assert.True(t, StringMatch("Cursor", "cursor", true))
}

func TestStringMatcher(t *testing.T) {
	testcase(t, "OverLeaf-Latex: An online LaTeX editor", "exce", false)
	testcase(t, "Windows Terminal", "term", true)
	testcase(t, "Microsoft SQL Server Management Studio", "mssms", true)
}

func testcase(t *testing.T, term string, search string, expected bool) {
	assert.Equal(t, StringMatch(term, search, false), expected)
}

func TestMultiplyTerms(t *testing.T) {
	terms := [][]string{{"1", "2"}}
	n := []string{"3", "4"}
	expected := [][]string{{"1", "2", "3"}, {"1", "2", "4"}}
	assert.Equal(t, expected, multiplyTerms(terms, n))
}

func TestGetPinYin(t *testing.T) {
	assert.Equal(t, []string{"qqyinle", "qqyinyue", "qqyl", "qqyy"}, getPinYin("QQ音乐"))
}
