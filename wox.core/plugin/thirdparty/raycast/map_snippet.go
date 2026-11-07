package raycast

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"

	"wox/plugin"
	"wox/plugin/system/snippets/dateformat"
)

const snippetKeywordPrefix = "Keyword: "

// snippetBody is the note text for one snippet. Keeping the keyword changes the copied body.
func snippetBody(text, keyword string, keep bool) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	if !keep || strings.TrimSpace(keyword) == "" {
		return text
	}
	return snippetKeywordPrefix + strings.TrimSpace(keyword) + "\n\n" + text
}

var snippetPlaceholder = regexp.MustCompile(`\{[^{}]*\}`)
var snippetSupportedDate = regexp.MustCompile(`^\{date format="[^"]*"\}$`)

// snippetHasUnsupported reports source placeholders that the destination cannot resolve.
func snippetHasUnsupported(text string) bool {
	depth := 0
	for _, c := range text {
		switch c {
		case '{':
			depth++
			if depth > 1 {
				return true
			}
		case '}':
			if depth > 0 {
				depth--
			}
		}
	}
	for _, token := range snippetPlaceholder.FindAllString(text, -1) {
		if token == "{date}" || token == "{clipboard}" || token == "{uuid}" {
			continue
		}
		if snippetSupportedDate.MatchString(token) {
			format := strings.TrimSuffix(strings.TrimPrefix(token, `{date format="`), `"}`)
			if dateformat.Supported(format) && !strings.ContainsAny(format, "&{}") {
				continue
			}
		}
		return true
	}
	return false
}

// snippetFingerprint identifies one source snippet and the body option, independent of catalog order.
func snippetFingerprint(sourceID, title, text, keyword string, keep bool) string {
	keepText := "0"
	if keep {
		keepText = "1"
	}
	sum := sha256.Sum256([]byte(strings.Join([]string{sourceID, title, text, keyword, keepText}, "\n")))
	return hex.EncodeToString(sum[:])
}

// convertSnippetVariables translates supported Raycast placeholders to Wox syntax.
func convertSnippetVariables(text string) string {
	var output strings.Builder
	for pos := 0; pos < len(text); {
		start := strings.IndexByte(text[pos:], '{')
		if start < 0 {
			output.WriteString(text[pos:])
			break
		}
		start += pos
		output.WriteString(text[pos:start])
		depth, end, nested := 0, start, false
		for ; end < len(text); end++ {
			switch text[end] {
			case '{':
				depth++
				nested = nested || depth > 1
			case '}':
				depth--
			}
			if depth == 0 {
				end++
				break
			}
		}
		token := text[start:end]
		if depth == 0 && !nested {
			switch token {
			case "{date}":
				token = "{wox:date}"
			case "{clipboard}":
				token = "{wox:clipboard_text}"
			case "{uuid}":
				token = "{wox:uuid}"
			default:
				if snippetSupportedDate.MatchString(token) {
					format := strings.TrimSuffix(strings.TrimPrefix(token, `{date format="`), `"}`)
					if dateformat.Supported(format) && !strings.ContainsAny(format, "&{}") {
						token = plugin.FormatQueryVariable("date", map[string]string{"format": format})
					}
				}
			}
		}
		output.WriteString(token)
		pos = end
	}
	return output.String()
}
