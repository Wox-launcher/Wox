package filesearch

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	// contentMaxTokenBytes drops tokens longer than 64 bytes. Log/diff/json
	// files contain base64 blobs, hashes, URLs, and stack traces that would
	// bloat the FTS index without recall value.
	contentMaxTokenBytes = 64

	// contentMinTokenChars drops Latin/code tokens shorter than 2 characters
	// to reduce noise. CJK unigrams (single Han chars) are always kept.
	contentMinTokenChars = 2
)

// cjkStoplist contains single Han characters that are too frequent to be useful
// as unigram index terms. They are excluded from unigram emission but still
// appear inside bigrams (e.g. "的是" is indexed), so phrase recall is preserved
// while the ultra-high-frequency single-char postings that would balloon the
// index are eliminated.
var cjkStoplist = map[rune]bool{
	'的': true, '了': true, '是': true, '在': true, '有': true,
	'个': true, '这': true, '那': true, '之': true, '与': true,
	'和': true, '或': true, '也': true, '都': true, '就': true,
	'你': true, '我': true, '他': true, '她': true, '它': true,
	'们': true, '上': true, '下': true, '不': true, '为': true,
	'以': true, '及': true, '等': true, '把': true, '被': true,
	'对': true, '从': true, '到': true, '向': true, '里': true,
	'中': true, '可': true, '能': true, '要': true, '会': true,
	'着': true, '过': true, '地': true, '得': true, '说': true,
	'做': true, '看': true, '想': true, '已': true, '再': true,
	'只': true, '还': true, '又': true, '更': true, '最': true,
}

// IsCJKStoplistUnigram reports whether r is a CJK character excluded from
// unigram indexing. Query-time uses this to drop stop-list single-char tokens
// from the content query (they won't be in the index).
func IsCJKStoplistUnigram(r rune) bool {
	return cjkStoplist[r]
}

// TokenizeForContentIndex splits text into space-separated tokens suitable for
// FTS5 unicode61 tokenizer. CJK runs produce unigrams (single Han chars, minus
// stop-list) and bigrams (consecutive Han char pairs). Latin/code runs produce
// whitespace+punct split tokens with camelCase/snake_case splitting, lowercased,
// filtered by min length (2 chars for Latin) and max byte length (64).
//
// The result is a single string with tokens joined by spaces, ready to be
// inserted into an FTS5 unicode61 column.
func TokenizeForContentIndex(text string) string {
	if text == "" {
		return ""
	}

	builder := contentTokenBuilder{}
	builder.output.Grow(len(text))
	for _, r := range text {
		switch {
		case isContentCJK(r):
			builder.flushLatin()
			builder.appendCJK(r)
		case isContentTokenBoundary(r):
			builder.flushCJK()
			builder.flushLatin()
		default:
			builder.flushCJK()
			builder.appendLatin(r)
		}
	}
	builder.flushCJK()
	builder.flushLatin()
	return builder.output.String()
}

// contentTokenBuilder streams normalized tokens directly into the final FTS input.
type contentTokenBuilder struct {
	output           strings.Builder
	hasToken         bool
	latin            [contentMaxTokenBytes]byte
	latinBytes       int
	latinRunes       int
	latinTooLong     bool
	latinPrevIsLower bool
	cjk              rune
	hasCJK           bool
}

func (b *contentTokenBuilder) appendCJK(r rune) {
	if b.hasCJK {
		if !cjkStoplist[b.cjk] {
			b.writeRunes(b.cjk)
		}
		b.writeRunes(b.cjk, r)
	}
	b.cjk = r
	b.hasCJK = true
}

func (b *contentTokenBuilder) flushCJK() {
	if !b.hasCJK {
		return
	}
	if !cjkStoplist[b.cjk] {
		b.writeRunes(b.cjk)
	}
	b.cjk = 0
	b.hasCJK = false
}

func (b *contentTokenBuilder) appendLatin(r rune) {
	if r == '_' || r == '-' || r == '.' {
		b.flushLatin()
		return
	}

	isUpper := unicode.IsUpper(r)
	if isUpper && b.latinPrevIsLower {
		b.flushLatin()
	}
	lower := unicode.ToLower(r)
	b.latinRunes++
	runeBytes := utf8.RuneLen(lower)
	if runeBytes < 0 {
		runeBytes = utf8.RuneLen(unicode.ReplacementChar)
		lower = unicode.ReplacementChar
	}
	if b.latinBytes+runeBytes <= len(b.latin) {
		b.latinBytes += utf8.EncodeRune(b.latin[b.latinBytes:], lower)
	} else {
		b.latinTooLong = true
	}
	b.latinPrevIsLower = !isUpper
}

func (b *contentTokenBuilder) flushLatin() {
	if b.latinRunes >= contentMinTokenChars && !b.latinTooLong {
		b.writeBytes(b.latin[:b.latinBytes])
	}
	b.latinBytes = 0
	b.latinRunes = 0
	b.latinTooLong = false
	b.latinPrevIsLower = false
}

func (b *contentTokenBuilder) beginToken() {
	if b.hasToken {
		b.output.WriteByte(' ')
	} else {
		b.hasToken = true
	}
}

func (b *contentTokenBuilder) writeBytes(token []byte) {
	if len(token) == 0 {
		return
	}
	b.beginToken()
	b.output.Write(token)
}

func (b *contentTokenBuilder) writeRunes(runes ...rune) {
	byteCount := 0
	for _, r := range runes {
		byteCount += utf8.RuneLen(r)
	}
	if byteCount <= 0 || byteCount > contentMaxTokenBytes {
		return
	}
	b.beginToken()
	for _, r := range runes {
		b.output.WriteRune(unicode.ToLower(r))
	}
}

func isContentCJK(r rune) bool {
	return unicode.Is(unicode.Han, r)
}

// isContentTokenBoundary reports whether r should split Latin/code tokens.
func isContentTokenBoundary(r rune) bool {
	if r == '_' || r == '-' || r == '.' {
		return false
	}
	if unicode.IsLetter(r) || unicode.IsDigit(r) {
		return false
	}
	return true
}

// ContentDefaultExtensions is the whitelist of file extensions eligible for
// content indexing. Binary files are excluded by omission.
func ContentDefaultExtensions() []string {
	return []string{
		"txt", "md", "json", "yaml", "yml", "xml", "csv", "tsv",
		"docx", "pptx", "xlsx", "pdf",
		"go", "py", "js", "ts", "tsx", "jsx", "rs", "c", "cpp", "h", "hpp",
		"java", "rb", "php", "sh", "bat", "ps1",
		"toml", "ini", "cfg", "conf",
		"html", "css", "scss", "less", "vue", "svelte",
		"sql", "lua", "pl", "kt", "swift", "dart", "r", "m",
		"tex", "bib", "log", "diff", "patch",
	}
}

// ContentExtensionsFromList builds a set from a list of extension strings
// (without leading dot, case-insensitive).
func ContentExtensionsFromList(exts []string) map[string]bool {
	set := make(map[string]bool, len(exts))
	for _, e := range exts {
		e = strings.TrimSpace(e)
		e = strings.TrimPrefix(e, ".")
		e = strings.ToLower(e)
		if e != "" {
			set[e] = true
		}
	}
	return set
}

// ContentDefaultMaxReadBytes is the default maximum bytes read from each file.
const ContentDefaultMaxReadBytes = 256 * 1024 // 256 KB

// IsContentSearchableExtension reports whether a path's extension is in the
// given extension whitelist.
func IsContentSearchableExtension(path string, extensions map[string]bool) bool {
	if len(extensions) == 0 {
		return false
	}
	ext := contentNormalizeExtension(path)
	if ext == "" {
		return false
	}
	return extensions[ext]
}

func contentNormalizeExtension(path string) string {
	dotIdx := strings.LastIndexByte(path, '.')
	if dotIdx < 0 || dotIdx == len(path)-1 {
		return ""
	}
	// Don't treat dotfiles (like .gitignore) as having an extension.
	slashIdx := strings.LastIndexAny(path, "/\\")
	if slashIdx == dotIdx-1 {
		return ""
	}
	return strings.ToLower(path[dotIdx+1:])
}
