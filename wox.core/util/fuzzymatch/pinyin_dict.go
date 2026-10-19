package fuzzymatch

import (
	"embed"
	"strings"
	"unicode"
	"unicode/utf8"
	"unsafe"
	"wox/internal/assetfs"
)

//go:embed pinyin_dict.txt
var rawPinyinFS embed.FS

var pinyinFS = assetfs.New(rawPinyinFS)

// pinyinDictionary uses a sparse two-level rune table and flat reading arrays.
// Only populated Unicode pages need row slots. Strings borrow one decoded asset,
// preserving pronunciation order without per-character maps or slice objects.
type pinyinDictionary struct {
	pages     []uint16
	rows      []uint16
	starts    []uint16
	syllables []string
	initials  []rune
}

// lookup lends immutable slices while the caller owns their backing dictionary.
func (d *pinyinDictionary) lookup(r rune) ([]string, []rune, bool) {
	if r < 0 || uint32(r)>>8 >= uint32(len(d.pages)) {
		return nil, nil, false
	}
	page := d.pages[uint32(r)>>8]
	if page == 0 {
		return nil, nil, false
	}
	row := d.rows[int(page)*256+int(r&255)]
	if row == 0 {
		return nil, nil, false
	}
	start, end := d.starts[row-1], d.starts[row]
	return d.syllables[start:end], d.initials[start:end], true
}

// loadPinyinDict counts storage once, then fills a small number of flat arrays.
// The source remains ordinary compressible text in both dev and packed builds.
func loadPinyinDict() *pinyinDictionary {
	bytes := pinyinFS.MustReadFile("pinyin_dict.txt")
	// Each resource read owns its bytes. Immutable reading strings can borrow
	// that allocation instead of copying the whole dictionary a second time.
	data := unsafe.String(unsafe.SliceData(bytes), len(bytes))
	var used [unicode.MaxRune/256 + 1]bool
	entries, readings, pages, maxPage := 0, 0, 0, 0
	for line := range strings.SplitSeq(strings.TrimSuffix(data, "\n"), "\n") {
		character, size := utf8.DecodeRuneInString(line)
		if character == utf8.RuneError || len(line) <= size+1 || line[size] != '\t' {
			panic("invalid embedded pinyin dictionary row")
		}
		page := int(character) >> 8
		if !used[page] {
			used[page] = true
			pages++
		}
		maxPage = max(maxPage, page)
		entries++
		readings += strings.Count(line[size+1:], ",") + 1
	}
	if entries >= 1<<16 || readings >= 1<<16 {
		panic("embedded pinyin dictionary exceeds compact offsets")
	}
	dictionary := &pinyinDictionary{pages: make([]uint16, maxPage+1), rows: make([]uint16, (pages+1)*256), starts: make([]uint16, entries+1), syllables: make([]string, 0, readings), initials: make([]rune, 0, readings)}
	pageID := uint16(0)
	for page, present := range used[:maxPage+1] {
		if present {
			pageID++
			dictionary.pages[page] = pageID
		}
	}
	entry := 0
	for line := range strings.SplitSeq(strings.TrimSuffix(data, "\n"), "\n") {
		character, size := utf8.DecodeRuneInString(line)
		page := dictionary.pages[uint32(character)>>8]
		row := int(page)*256 + int(character&255)
		if dictionary.rows[row] != 0 {
			panic("duplicate embedded pinyin character")
		}
		dictionary.rows[row] = uint16(entry + 1)
		for syllable := range strings.SplitSeq(line[size+1:], ",") {
			if syllable == "" {
				panic("empty embedded pinyin reading")
			}
			dictionary.syllables = append(dictionary.syllables, syllable)
			dictionary.initials = append(dictionary.initials, toLowerASCII(rune(syllable[0])))
		}
		entry++
		dictionary.starts[entry] = uint16(len(dictionary.syllables))
	}
	return dictionary
}
