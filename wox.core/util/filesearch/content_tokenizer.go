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

	tokens := tokenizeContent(text)
	return strings.Join(tokens, " ")
}

// tokenizeContent is the core tokenizer that returns a slice of tokens.
// Exported via TokenizeForContentIndex (joined string) for FTS5 insertion.
func tokenizeContent(text string) []string {
	var tokens []string
	var cjkRun []rune
	var latinRun strings.Builder

	flushCJK := func() {
		if len(cjkRun) == 0 {
			return
		}
		tokens = emitCJKTokens(tokens, cjkRun)
		cjkRun = cjkRun[:0]
	}

	flushLatin := func() {
		if latinRun.Len() == 0 {
			return
		}
		s := latinRun.String()
		latinRun.Reset()
		for _, sub := range splitIdentifier(s) {
			tokens = appendLatinTokenIfValid(tokens, sub)
		}
	}

	for _, r := range text {
		switch {
		case isContentCJK(r):
			flushLatin()
			cjkRun = append(cjkRun, r)
		case isContentTokenBoundary(r):
			flushCJK()
			flushLatin()
		default:
			flushCJK()
			latinRun.WriteRune(r)
		}
	}
	flushCJK()
	flushLatin()

	return tokens
}

// emitCJKTokens emits unigrams (minus stop-list) and bigrams for a CJK run.
// CJK tokens bypass the min-char filter (single Han chars are valid unigrams);
// only the byte cap applies (which they never hit).
func emitCJKTokens(tokens []string, run []rune) []string {
	for i, r := range run {
		if !cjkStoplist[r] {
			tokens = appendCJKTokenIfValid(tokens, string(r))
		}
		if i+1 < len(run) {
			bigram := string(run[i : i+2])
			tokens = appendCJKTokenIfValid(tokens, bigram)
		}
	}
	return tokens
}

// splitIdentifier applies camelCase and snake_case splitting to a raw token,
// then lowercases each sub-token. Example: "getUserById" -> ["get","user","by","id"],
// "my_var_name" -> ["my","var","name"], "config.json" -> ["config","json"].
func splitIdentifier(s string) []string {
	if s == "" {
		return nil
	}

	var subs []string
	var current strings.Builder
	prevIsLower := false

	for i, r := range s {
		if r == '_' || r == '-' || r == '.' {
			if current.Len() > 0 {
				subs = append(subs, current.String())
				current.Reset()
			}
			prevIsLower = false
			continue
		}

		isUpper := unicode.IsUpper(r)
		if i > 0 && isUpper && prevIsLower {
			if current.Len() > 0 {
				subs = append(subs, current.String())
				current.Reset()
			}
		}
		current.WriteRune(unicode.ToLower(r))
		prevIsLower = !isUpper
	}
	if current.Len() > 0 {
		subs = append(subs, current.String())
	}

	return subs
}

// appendLatinTokenIfValid appends a lowercased Latin/code token if it passes
// length filters: min 2 chars, max 64 bytes.
func appendLatinTokenIfValid(tokens []string, token string) []string {
	if token == "" {
		return tokens
	}
	lower := strings.ToLower(token)
	if utf8.RuneCountInString(lower) < contentMinTokenChars {
		return tokens
	}
	if len(lower) > contentMaxTokenBytes {
		return tokens
	}
	return append(tokens, lower)
}

// appendCJKTokenIfValid appends a lowercased CJK token if it passes the byte
// cap only. The min-char filter does not apply — single Han chars are valid
// unigrams.
func appendCJKTokenIfValid(tokens []string, token string) []string {
	if token == "" {
		return tokens
	}
	lower := strings.ToLower(token)
	if len(lower) > contentMaxTokenBytes {
		return tokens
	}
	return append(tokens, lower)
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