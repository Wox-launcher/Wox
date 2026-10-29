package component

import "testing"

func TestFindTextMatchesUnicodeAndLiteral(t *testing.T) {
	for _, test := range []struct {
		text, query string
		offsets     []int
	}{
		{"😀你好 HELLO hello", "hello", []int{4, 10}},
		{"你好你好", "你好", []int{0, 2}},
		{"a.*b .*", ".*", []int{1, 5}},
		{"aaaaa", "aa", []int{0, 2}},
		{"hello", "", nil}, {"hello", "missing", nil},
	} {
		matches := FindTextMatches(test.text, test.query)
		if len(matches) != len(test.offsets) {
			t.Fatalf("%q/%q: %v", test.text, test.query, matches)
		}
		for i, offset := range test.offsets {
			if matches[i].Start() != offset {
				t.Fatalf("%q: %v", test.text, matches)
			}
		}
	}
}
