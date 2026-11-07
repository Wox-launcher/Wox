package snippets

import (
	"strings"
	"time"

	"github.com/google/uuid"

	"wox/plugin"
	"wox/plugin/system/snippets/dateformat"
)

// expandTemplate resolves supported variables once per expansion and leaves unknown tokens intact.
func expandTemplate(text string, now time.Time, readClipboard func() (string, error)) (string, error) {
	var clipboard string
	var clipboardRead bool
	var clipboardErr error
	resolve := func(token string) string {
		ref, ok := plugin.ParseQueryVariable(token)
		if !ok {
			return token
		}
		switch ref.Name {
		case "uuid":
			if len(ref.Params) == 0 {
				return uuid.NewString()
			}
		case "clipboard_text":
			if len(ref.Params) != 0 {
				return token
			}
			if !clipboardRead {
				clipboard, clipboardErr = readClipboard()
				clipboardRead = true
			}
			return clipboard
		case "date":
			if len(ref.Params) == 0 {
				return now.Format("2006-01-02")
			}
			format, ok := ref.Params["format"]
			if !ok || len(ref.Params) != 1 || !dateformat.Supported(format) {
				return token
			}
			// Format each token separately so literal digits do not become Go layout directives.
			return strings.NewReplacer("yyyy", now.Format("2006"), "MM", now.Format("01"), "dd", now.Format("02"),
				"HH", now.Format("15"), "mm", now.Format("04"), "ss", now.Format("05")).Replace(format)
		}
		return token
	}
	var result strings.Builder
	for pos := 0; pos < len(text); {
		start := strings.IndexByte(text[pos:], '{')
		if start < 0 {
			result.WriteString(text[pos:])
			break
		}
		start += pos
		result.WriteString(text[pos:start])
		depth, end := 0, start
		nested := false
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
			result.WriteString(resolve(token))
		} else {
			result.WriteString(token)
		}
		pos = end
	}
	if clipboardErr != nil {
		return "", clipboardErr
	}
	return result.String(), nil
}
