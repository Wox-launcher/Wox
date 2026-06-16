package filesearch

import (
	"strings"
	"unicode"

	"wox/util/fuzzymatch"
)

func buildPinyinFields(input string) (string, string) {
	if !containsHanRune(input) {
		// Optimization: ASCII-only file names are already indexed by normalized
		// name, name_key, and path FTS. Treating every ASCII name as "pinyin"
		// filled two extra FTS tables for tens of thousands of code files without
		// helping Chinese pinyin recall, so only names that actually contain Han
		// characters get pinyin payloads. Mixed names still keep their ASCII
		// letters below so queries like "xiangmu2" can match "项目2".
		return "", ""
	}

	var full strings.Builder
	var initials strings.Builder

	for _, r := range input {
		switch {
		case unicode.Is(unicode.Han, r):
			pinyins, ok := fuzzymatch.PinyinDict[int(r)]
			if !ok || len(pinyins) == 0 {
				continue
			}

			pinyin := strings.ToLower(strings.TrimSpace(pinyins[0]))
			if pinyin == "" {
				continue
			}

			full.WriteString(pinyin)
			initials.WriteByte(pinyin[0])
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			lower := strings.ToLower(string(r))
			full.WriteString(lower)
			initials.WriteString(lower)
		}
	}

	return full.String(), initials.String()
}

func containsHanRune(input string) bool {
	for _, r := range input {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}
