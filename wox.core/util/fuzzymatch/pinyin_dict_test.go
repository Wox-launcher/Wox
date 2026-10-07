package fuzzymatch

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"testing"
)

// TestPinyinDictionaryContent preserves every original character and the order
// of its pronunciations when changing the dictionary's storage representation.
func TestPinyinDictionaryContent(t *testing.T) {
	dictionary := loadPinyinDict()
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

var benchmarkPinyinDictionary map[int][]string

// BenchmarkLoadPinyinDictionary includes decompression in release-overlay builds.
func BenchmarkLoadPinyinDictionary(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		benchmarkPinyinDictionary = loadPinyinDict()
	}
}
