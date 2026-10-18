package fuzzymatch

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"
)

// TestPinyinDictionaryContent preserves every original character and the order
// of its pronunciations when changing the dictionary's storage representation.
func TestPinyinDictionaryContent(t *testing.T) {
	compact := loadPinyinDict()
	dictionary := make(map[int][]string)
	for line := range strings.SplitSeq(strings.TrimSpace(string(pinyinFS.MustReadFile("pinyin_dict.txt"))), "\n") {
		r, _ := utf8.DecodeRuneInString(line)
		syllables, initials, ok := compact.lookup(r)
		if !ok || len(syllables) != len(initials) {
			t.Fatalf("missing reading for %U", r)
		}
		for i, syllable := range syllables {
			if initials[i] != toLowerASCII(rune(syllable[0])) {
				t.Fatalf("initial changed for %U", r)
			}
		}
		dictionary[int(r)] = syllables
	}
	if len(dictionary) != 8105 {
		t.Fatalf("dictionary entries = %d, want 8105", len(dictionary))
	}
	data, err := json.Marshal(dictionary)
	if err != nil {
		t.Fatal(err)
	}
	const want = "b4ee6ecdcf975b29a5038f4c4560723575841aa118c684cfc602134c3e64e80c"
	if got := fmt.Sprintf("%x", sha256.Sum256(data)); got != want {
		t.Fatalf("dictionary fingerprint = %s, want %s", got, want)
	}
}

// TestPinyinDictionaryBorrowedReadingsSurviveRelease checks owned asset bytes
// after the table is unpublished and collected, including supplementary Han.
func TestPinyinDictionaryBorrowedReadingsSurviveRelease(t *testing.T) {
	for _, r := range []rune{'重', '项', '𬺓'} {
		readings, initials, ok := lookupPinyinReading(r)
		if !ok {
			t.Fatalf("missing %U", r)
		}
		want := strings.Join(readings, ",")
		ReleasePinyinDictionary()
		runtime.GC()
		if strings.Join(readings, ",") != want || len(initials) != len(readings) {
			t.Fatalf("borrowed readings lost for %U", r)
		}
		for i, reading := range readings {
			if initials[i] != toLowerASCII(rune(reading[0])) {
				t.Fatalf("borrowed initial lost for %U", r)
			}
		}
	}
	for _, r := range []rune{-1, 0, 'A', unicode.MaxRune, unicode.MaxRune + 1} {
		if syllables, _, ok := lookupPinyinReading(r); ok || syllables != nil {
			t.Fatalf("unexpected reading for %U", r)
		}
	}
}

var benchmarkPinyinDictionary *pinyinDictionary

// BenchmarkLoadPinyinDictionary includes decompression in release-overlay builds.
func BenchmarkLoadPinyinDictionary(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		benchmarkPinyinDictionary = loadPinyinDict()
	}
}
