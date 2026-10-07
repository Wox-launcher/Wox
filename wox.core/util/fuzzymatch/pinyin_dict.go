package fuzzymatch

import (
	"embed"
	"strings"
	"unicode/utf8"
	"wox/internal/assetfs"
)

//go:embed pinyin_dict.txt
var rawPinyinFS embed.FS

var pinyinFS = assetfs.New(rawPinyinFS)

// loadPinyinDict keeps the dictionary as compressible data instead of generating
// thousands of map-initialization instructions. Decoded strings belong to this
// map, so releasing the dictionary also releases its backing text.
func loadPinyinDict() map[int][]string {
	data := string(pinyinFS.MustReadFile("pinyin_dict.txt"))
	dictionary := make(map[int][]string, strings.Count(data, "\n"))
	for line := range strings.SplitSeq(strings.TrimSuffix(data, "\n"), "\n") {
		character, size := utf8.DecodeRuneInString(line)
		if character == utf8.RuneError || len(line) <= size+1 || line[size] != '\t' {
			panic("invalid embedded pinyin dictionary row")
		}
		dictionary[int(character)] = strings.Split(line[size+1:], ",")
	}
	return dictionary
}
