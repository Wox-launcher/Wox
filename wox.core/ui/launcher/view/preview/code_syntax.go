package preview

import (
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// CodeTokenKind classifies syntax spans rendered in the code file preview.
type CodeTokenKind uint8

const (
	CodeTokenPlain CodeTokenKind = iota
	CodeTokenComment
	CodeTokenKeyword
	CodeTokenString
	CodeTokenNumber
)

// CodeToken is a single syntax-highlighted slice of source text.
type CodeToken struct {
	Kind  CodeTokenKind
	Value string
}

type syntaxProfile struct {
	lineComment       string
	altLineComment    string
	blockCommentStart string
	blockCommentEnd   string
	backtickString    bool
	backtickRaw       bool
	tripleQuotes      bool
	singleQuoteChar   bool
	multilineQuotes   bool
	markupTags        bool
	atKeyword         bool
	dashInIdent       bool
	caseInsensitive   bool
	remComment        bool
	keywords          map[string]struct{}
}

func keywordSet(words ...string) map[string]struct{} {
	set := make(map[string]struct{}, len(words))
	for _, w := range words {
		set[w] = struct{}{}
	}
	return set
}

var (
	goProfile = syntaxProfile{
		lineComment: "//", blockCommentStart: "/*", blockCommentEnd: "*/",
		backtickString: true, backtickRaw: true, singleQuoteChar: true,
		keywords: keywordSet(
			"break", "case", "chan", "const", "continue", "default", "defer", "else",
			"fallthrough", "for", "func", "go", "goto", "if", "import", "interface",
			"map", "package", "range", "return", "select", "struct", "switch", "type",
			"var", "true", "false", "nil", "iota",
		),
	}
	jsProfile = syntaxProfile{
		lineComment: "//", blockCommentStart: "/*", blockCommentEnd: "*/",
		backtickString: true,
		keywords: keywordSet(
			"as", "async", "await", "break", "case", "catch", "class", "const",
			"continue", "debugger", "default", "delete", "do", "else", "enum", "export",
			"extends", "false", "finally", "for", "from", "function", "if", "implements",
			"import", "in", "instanceof", "interface", "let", "new", "null", "of",
			"package", "private", "protected", "public", "readonly", "return", "static",
			"super", "switch", "this", "throw", "true", "try", "type", "typeof",
			"undefined", "var", "void", "while", "with", "yield",
		),
	}
	cLikeProfile = syntaxProfile{
		lineComment: "//", blockCommentStart: "/*", blockCommentEnd: "*/",
		singleQuoteChar: true,
		keywords: keywordSet(
			"abstract", "as", "async", "auto", "await", "bool", "boolean", "break",
			"byte", "case", "catch", "char", "class", "const", "constexpr", "continue",
			"default", "defer", "delete", "do", "double", "else", "enum", "explicit",
			"export", "extends", "extern", "false", "final", "finally", "float", "fn",
			"for", "foreach", "func", "fun", "goto", "guard", "if", "impl", "implements",
			"import", "in", "inline", "int", "interface", "internal", "is", "let", "long",
			"match", "mod", "mut", "namespace", "new", "nil", "noexcept", "null", "nullptr",
			"object", "operator", "override", "package", "private", "protected", "pub",
			"public", "readonly", "return", "sealed", "self", "short", "signed", "sizeof",
			"static", "struct", "super", "switch", "template", "this", "throw", "throws",
			"trait", "true", "try", "typedef", "typename", "uint", "union", "unsafe",
			"unsigned", "use", "using", "val", "var", "virtual", "void", "volatile",
			"where", "while", "yield",
		),
	}
	pythonProfile = syntaxProfile{
		lineComment: "#", tripleQuotes: true,
		keywords: keywordSet(
			"False", "None", "True", "and", "as", "assert", "async", "await", "break",
			"class", "continue", "def", "del", "elif", "else", "except", "finally", "for",
			"from", "global", "if", "import", "in", "is", "lambda", "match", "nonlocal",
			"not", "or", "pass", "raise", "return", "try", "while", "with", "yield",
		),
	}
	shellProfile = syntaxProfile{
		lineComment: "#", backtickString: true, multilineQuotes: true,
		keywords: keywordSet(
			"break", "case", "continue", "do", "done", "elif", "else", "esac", "export",
			"fi", "for", "function", "if", "in", "local", "readonly", "return", "select",
			"then", "until", "while", "true", "false",
		),
	}
	powershellProfile = syntaxProfile{
		lineComment: "#", blockCommentStart: "<#", blockCommentEnd: "#>", multilineQuotes: true,
		keywords: keywordSet(
			"begin", "break", "catch", "class", "continue", "data", "do", "dynamicparam",
			"else", "elseif", "end", "enum", "exit", "filter", "finally", "for", "foreach",
			"from", "function", "if", "in", "param", "process", "return", "switch", "throw",
			"trap", "try", "until", "using", "while", "true", "false", "null",
		),
	}
	rubyProfile = syntaxProfile{
		lineComment: "#", backtickString: true, multilineQuotes: true,
		keywords: keywordSet(
			"BEGIN", "END", "alias", "and", "begin", "break", "case", "class", "def",
			"defined?", "do", "else", "elsif", "end", "ensure", "false", "for", "if",
			"in", "module", "next", "nil", "not", "or", "redo", "rescue", "retry",
			"return", "self", "super", "then", "true", "undef", "unless", "until",
			"when", "while", "yield",
		),
	}
	jsonProfile = syntaxProfile{
		lineComment: "//", blockCommentStart: "/*", blockCommentEnd: "*/",
		keywords: keywordSet("true", "false", "null"),
	}
	configProfile = syntaxProfile{
		lineComment: "#", altLineComment: ";",
		keywords: keywordSet("true", "false", "null", "yes", "no", "on", "off"),
	}
	markupProfile = syntaxProfile{
		blockCommentStart: "<!--", blockCommentEnd: "-->",
		markupTags: true,
	}
	cssProfile = syntaxProfile{
		lineComment: "//", blockCommentStart: "/*", blockCommentEnd: "*/",
		atKeyword: true, dashInIdent: true,
		keywords: keywordSet("important", "inherit", "initial", "unset", "revert", "none", "auto"),
	}
	sqlProfile = syntaxProfile{
		lineComment: "--", blockCommentStart: "/*", blockCommentEnd: "*/",
		caseInsensitive: true, multilineQuotes: true,
		keywords: keywordSet(
			"add", "all", "alter", "and", "as", "asc", "between", "by", "case", "cast",
			"check", "column", "commit", "constraint", "create", "cross", "default",
			"delete", "desc", "distinct", "drop", "else", "end", "exists", "false",
			"foreign", "from", "full", "group", "having", "in", "index", "inner",
			"insert", "into", "is", "join", "key", "left", "like", "limit", "not",
			"null", "offset", "on", "or", "order", "outer", "primary", "references",
			"returning", "right", "rollback", "select", "set", "table", "then", "true",
			"union", "unique", "update", "values", "view", "when", "where", "with",
		),
	}
	luaProfile = syntaxProfile{
		lineComment: "--",
		keywords: keywordSet(
			"and", "break", "do", "else", "elseif", "end", "false", "for", "function",
			"goto", "if", "in", "local", "nil", "not", "or", "repeat", "return",
			"then", "true", "until", "while",
		),
	}
	batchProfile = syntaxProfile{
		lineComment: "::", remComment: true, caseInsensitive: true,
		keywords: keywordSet(
			"call", "cd", "copy", "del", "do", "echo", "else", "endlocal", "equ",
			"exist", "exit", "for", "goto", "if", "in", "neq", "not", "off", "on",
			"pause", "pushd", "popd", "set", "setlocal", "shift", "start",
		),
	}
	dockerMakeProfile = syntaxProfile{
		lineComment: "#", caseInsensitive: true,
		keywords: keywordSet(
			"arg", "as", "cmd", "copy", "add", "entrypoint", "env", "expose", "from",
			"healthcheck", "label", "maintainer", "onbuild", "run", "shell", "stopsignal",
			"user", "volume", "workdir", "ifeq", "ifneq", "ifdef", "ifndef", "else",
			"endif", "include", "define", "endef", "export",
		),
	}
)

var codeFileExtensions = map[string]syntaxProfile{
	".go":         goProfile,
	".js":         jsProfile,
	".mjs":        jsProfile,
	".cjs":        jsProfile,
	".jsx":        jsProfile,
	".ts":         jsProfile,
	".mts":        jsProfile,
	".cts":        jsProfile,
	".tsx":        jsProfile,
	".c":          cLikeProfile,
	".h":          cLikeProfile,
	".cc":         cLikeProfile,
	".cpp":        cLikeProfile,
	".cxx":        cLikeProfile,
	".hpp":        cLikeProfile,
	".hh":         cLikeProfile,
	".m":          cLikeProfile,
	".mm":         cLikeProfile,
	".cs":         cLikeProfile,
	".java":       cLikeProfile,
	".kt":         cLikeProfile,
	".kts":        cLikeProfile,
	".swift":      cLikeProfile,
	".rs":         cLikeProfile,
	".dart":       cLikeProfile,
	".scala":      cLikeProfile,
	".zig":        cLikeProfile,
	".proto":      cLikeProfile,
	".groovy":     cLikeProfile,
	".gradle":     cLikeProfile,
	".php":        cLikeProfile,
	".py":         pythonProfile,
	".pyw":        pythonProfile,
	".pyi":        pythonProfile,
	".sh":         shellProfile,
	".bash":       shellProfile,
	".zsh":        shellProfile,
	".fish":       shellProfile,
	".ps1":        powershellProfile,
	".psm1":       powershellProfile,
	".psd1":       powershellProfile,
	".rb":         rubyProfile,
	".pl":         shellProfile,
	".pm":         shellProfile,
	".r":          shellProfile,
	".ex":         shellProfile,
	".exs":        shellProfile,
	".nix":        shellProfile,
	".tf":         shellProfile,
	".tfvars":     shellProfile,
	".hcl":        shellProfile,
	".mk":         dockerMakeProfile,
	".cmake":      dockerMakeProfile,
	".dockerfile": dockerMakeProfile,
	".json":       jsonProfile,
	".jsonc":      jsonProfile,
	".json5":      jsonProfile,
	".yaml":       configProfile,
	".yml":        configProfile,
	".toml":       configProfile,
	".ini":        configProfile,
	".cfg":        configProfile,
	".conf":       configProfile,
	".properties": configProfile,
	".env":        configProfile,
	".html":       markupProfile,
	".htm":        markupProfile,
	".xml":        markupProfile,
	".xhtml":      markupProfile,
	".plist":      markupProfile,
	".csproj":     markupProfile,
	".xaml":       markupProfile,
	".vue":        markupProfile,
	".svelte":     markupProfile,
	".css":        cssProfile,
	".scss":       cssProfile,
	".sass":       cssProfile,
	".less":       cssProfile,
	".sql":        sqlProfile,
	".lua":        luaProfile,
	".bat":        batchProfile,
	".cmd":        batchProfile,
	".graphql":    shellProfile,
	".gql":        shellProfile,
	".hs":         luaProfile,
	".clj":        configProfile,
	".cljs":       configProfile,
	".el":         configProfile,
	".vim":        configProfile,
	".diff":       configProfile,
	".patch":      configProfile,
}

var codeFileNames = map[string]syntaxProfile{
	"makefile":       dockerMakeProfile,
	"gnumakefile":    dockerMakeProfile,
	"dockerfile":     dockerMakeProfile,
	"containerfile":  dockerMakeProfile,
	"cmakelists.txt": dockerMakeProfile,
	"go.mod":         goProfile,
	"go.sum":         configProfile,
	".gitignore":     configProfile,
	".env":           configProfile,
}

func lookupSyntaxProfile(filename string) (syntaxProfile, bool) {
	base := strings.ToLower(filepath.Base(strings.TrimSpace(filename)))
	if base == "" {
		return syntaxProfile{}, false
	}
	if prof, ok := codeFileNames[base]; ok {
		return prof, true
	}
	ext := strings.ToLower(filepath.Ext(base))
	prof, ok := codeFileExtensions[ext]
	return prof, ok
}

// IsCodePreviewFile reports whether filename should be rendered with syntax-highlighted code preview.
func IsCodePreviewFile(filename string) bool {
	_, ok := lookupSyntaxProfile(filename)
	return ok
}

// TokenizeCode splits source into coalesced syntax tokens whose values concatenate back to source.
func TokenizeCode(filename string, source string) []CodeToken {
	if source == "" {
		return nil
	}
	prof, ok := lookupSyntaxProfile(filename)
	if !ok {
		return []CodeToken{{Kind: CodeTokenPlain, Value: source}}
	}

	var tokens []CodeToken
	emit := func(kind CodeTokenKind, val string) {
		if val == "" {
			return
		}
		if len(tokens) > 0 && tokens[len(tokens)-1].Kind == kind {
			tokens[len(tokens)-1].Value += val
			return
		}
		tokens = append(tokens, CodeToken{Kind: kind, Value: val})
	}

	inTag := false
	n := len(source)
	i := 0
	for i < n {
		if prof.lineComment != "" && strings.HasPrefix(source[i:], prof.lineComment) && (!prof.markupTags || !inTag) {
			end := scanLineEnd(source, i+len(prof.lineComment))
			emit(CodeTokenComment, source[i:end])
			i = end
			continue
		}
		if prof.altLineComment != "" && strings.HasPrefix(source[i:], prof.altLineComment) {
			end := scanLineEnd(source, i+len(prof.altLineComment))
			emit(CodeTokenComment, source[i:end])
			i = end
			continue
		}
		if prof.remComment && isLineStart(source, i) && hasCaseInsensitiveREM(source[i:]) {
			end := scanLineEnd(source, i+3)
			emit(CodeTokenComment, source[i:end])
			i = end
			continue
		}
		if prof.blockCommentStart != "" && strings.HasPrefix(source[i:], prof.blockCommentStart) {
			end := i + len(prof.blockCommentStart)
			if closeIdx := strings.Index(source[end:], prof.blockCommentEnd); closeIdx >= 0 {
				end += closeIdx + len(prof.blockCommentEnd)
			} else {
				end = n
			}
			emit(CodeTokenComment, source[i:end])
			i = end
			continue
		}

		b := source[i]
		if prof.markupTags {
			if b == '<' {
				tagStart := i + 1
				if tagStart < n && source[tagStart] == '/' {
					tagStart++
				}
				if tagStart < n && isASCIIAlpha(source[tagStart]) {
					tagEnd := tagStart + 1
					for tagEnd < n && (isIdentContinue(source[tagEnd]) || source[tagEnd] == '-' || source[tagEnd] == ':') {
						tagEnd++
					}
					emit(CodeTokenPlain, source[i:tagStart])
					emit(CodeTokenKeyword, source[tagStart:tagEnd])
					inTag = true
					i = tagEnd
					continue
				}
			} else if b == '>' && inTag {
				inTag = false
				emit(CodeTokenPlain, source[i:i+1])
				i++
				continue
			}
		}

		if prof.tripleQuotes && (strings.HasPrefix(source[i:], `"""`) || strings.HasPrefix(source[i:], `'''`)) {
			delim := source[i : i+3]
			end := scanTripleQuoted(source, i+3, delim)
			emit(CodeTokenString, source[i:end])
			i = end
			continue
		}

		if prof.backtickString && b == '`' {
			end := scanQuoted(source, i+1, '`', !prof.backtickRaw, true)
			emit(CodeTokenString, source[i:end])
			i = end
			continue
		}

		if b == '"' && (!prof.markupTags || inTag) {
			end := scanQuoted(source, i+1, '"', true, prof.multilineQuotes)
			emit(CodeTokenString, source[i:end])
			i = end
			continue
		}

		if b == '\'' {
			if prof.singleQuoteChar {
				if end, ok := scanCharLiteral(source, i); ok {
					emit(CodeTokenString, source[i:end])
					i = end
					continue
				}
			} else if !prof.markupTags || inTag {
				end := scanQuoted(source, i+1, '\'', true, prof.multilineQuotes)
				emit(CodeTokenString, source[i:end])
				i = end
				continue
			}
		}

		if prof.atKeyword && b == '@' && i+1 < n && isASCIIAlpha(source[i+1]) {
			end := scanIdent(source, i+1, true)
			emit(CodeTokenKeyword, source[i:end])
			i = end
			continue
		}

		if isIdentStart(b) && len(prof.keywords) > 0 {
			end := scanIdent(source, i+1, prof.dashInIdent)
			word := source[i:end]
			lookup := word
			if prof.caseInsensitive {
				lookup = strings.ToLower(word)
			}
			if _, isKW := prof.keywords[lookup]; isKW {
				emit(CodeTokenKeyword, word)
			} else {
				emit(CodeTokenPlain, word)
			}
			i = end
			continue
		}

		if isASCIIDigit(b) && (!prof.markupTags || inTag) {
			end := scanNumber(source, i)
			emit(CodeTokenNumber, source[i:end])
			i = end
			continue
		}

		_, size := utf8.DecodeRuneInString(source[i:])
		if size <= 0 {
			size = 1
		}
		emit(CodeTokenPlain, source[i:i+size])
		i += size
	}

	return tokens
}

func scanLineEnd(s string, i int) int {
	if idx := strings.IndexByte(s[i:], '\n'); idx >= 0 {
		return i + idx
	}
	return len(s)
}

func isLineStart(s string, i int) bool {
	for j := i - 1; j >= 0; j-- {
		if s[j] == '\n' {
			return true
		}
		if s[j] != ' ' && s[j] != '\t' {
			return false
		}
	}
	return true
}

func hasCaseInsensitiveREM(s string) bool {
	if len(s) < 3 || !strings.EqualFold(s[:3], "rem") {
		return false
	}
	return len(s) == 3 || s[3] == ' ' || s[3] == '\t' || s[3] == '\r' || s[3] == '\n'
}

func scanTripleQuoted(s string, i int, delim string) int {
	for i < len(s) {
		if s[i] == '\\' && i+1 < len(s) {
			i += 2
			continue
		}
		if strings.HasPrefix(s[i:], delim) {
			return i + 3
		}
		i++
	}
	return len(s)
}

func scanQuoted(s string, i int, quote byte, escape bool, multiline bool) int {
	for i < len(s) {
		c := s[i]
		if !multiline && c == '\n' {
			return i
		}
		if escape && c == '\\' && i+1 < len(s) {
			i += 2
			continue
		}
		if c == quote {
			return i + 1
		}
		i++
	}
	return len(s)
}

// scanCharLiteral matches 'x' or '\n' style literals without misclassifying Rust lifetimes like 'a.
func scanCharLiteral(s string, i int) (int, bool) {
	j := i + 1
	if j >= len(s) || s[j] == '\n' || s[j] == '\'' {
		return 0, false
	}
	if s[j] == '\\' {
		j += 2
		for count := 0; j < len(s) && count < 10; count++ {
			if s[j] == '\n' {
				return 0, false
			}
			if s[j] == '\'' {
				return j + 1, true
			}
			j++
		}
		return 0, false
	}
	_, size := utf8.DecodeRuneInString(s[j:])
	if size <= 0 {
		return 0, false
	}
	j += size
	if j < len(s) && s[j] == '\'' {
		return j + 1, true
	}
	return 0, false
}

func scanIdent(s string, i int, allowDash bool) int {
	for i < len(s) {
		c := s[i]
		if isIdentContinue(c) || (allowDash && c == '-') {
			i++
			continue
		}
		break
	}
	return i
}

func scanNumber(s string, i int) int {
	n := len(s)
	if i+1 < n && s[i] == '0' {
		switch s[i+1] {
		case 'x', 'X':
			j := i + 2
			for j < n && (isHexDigit(s[j]) || s[j] == '_') {
				j++
			}
			if j > i+2 {
				return j
			}
		case 'b', 'B':
			j := i + 2
			for j < n && (s[j] == '0' || s[j] == '1' || s[j] == '_') {
				j++
			}
			if j > i+2 {
				return j
			}
		case 'o', 'O':
			j := i + 2
			for j < n && ((s[j] >= '0' && s[j] <= '7') || s[j] == '_') {
				j++
			}
			if j > i+2 {
				return j
			}
		}
	}

	j := i
	for j < n && (isASCIIDigit(s[j]) || s[j] == '_') {
		j++
	}
	if j+1 < n && s[j] == '.' && isASCIIDigit(s[j+1]) {
		j += 2
		for j < n && (isASCIIDigit(s[j]) || s[j] == '_') {
			j++
		}
	}
	if j < n && (s[j] == 'e' || s[j] == 'E') {
		k := j + 1
		if k < n && (s[k] == '+' || s[k] == '-') {
			k++
		}
		if k < n && isASCIIDigit(s[k]) {
			j = k + 1
			for j < n && (isASCIIDigit(s[j]) || s[j] == '_') {
				j++
			}
		}
	}
	return j
}

func isASCIIAlpha(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

func isASCIIDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

func isHexDigit(b byte) bool {
	return isASCIIDigit(b) || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')
}

func isIdentStart(b byte) bool {
	return isASCIIAlpha(b) || b == '_'
}

func isIdentContinue(b byte) bool {
	return isASCIIAlpha(b) || isASCIIDigit(b) || b == '_'
}
