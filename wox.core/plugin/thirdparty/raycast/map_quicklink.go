package raycast

import (
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"unicode"
)

// quicklinkTarget prefers an explicit filesystem path over the link field.
func quicklinkTarget(link Quicklink) string {
	if path := strings.TrimSpace(link.Path); path != "" {
		return path
	}
	return strings.TrimSpace(link.Link)
}

// quicklinkLocalPath reports a file or folder target. Web links stay on the search path.
func quicklinkLocalPath(link string) (string, bool) {
	link = strings.TrimSpace(link)
	if link == "" {
		return "", false
	}
	if strings.HasPrefix(strings.ToLower(link), "file:") {
		parsed, err := url.Parse(link)
		if err != nil {
			return "", false
		}
		decoded, err := url.PathUnescape(parsed.Path)
		if err != nil {
			decoded = parsed.Path
		}
		if runtime.GOOS == "windows" {
			decoded = strings.TrimPrefix(decoded, "/")
		}
		decoded = filepath.Clean(filepath.FromSlash(decoded))
		if filepath.IsAbs(decoded) {
			return decoded, true
		}
		return "", false
	}
	if strings.Contains(link, "://") {
		return "", false
	}
	if strings.HasPrefix(link, "~/") || strings.HasPrefix(link, `~\`) || link == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", false
		}
		if link == "~" {
			link = home
		} else {
			link = filepath.Join(home, link[2:])
		}
	}
	cleaned := filepath.Clean(link)
	if filepath.IsAbs(cleaned) {
		return cleaned, true
	}
	return "", false
}

func quicklinkPathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

var (
	quicklinkNamedPlaceholder = regexp.MustCompile(`(?i)\{(?:query|argument)\s+name="([^"]*)"\}`)
	quicklinkPlainPlaceholder = regexp.MustCompile(`(?i)\{(?:query|argument)\}`)
	quicklinkAnyPlaceholder   = regexp.MustCompile(`\{[^{}]+\}`)
)

// rewriteQuicklinkLink turns Raycast argument placeholders into Web Search parameters.
func rewriteQuicklinkLink(link string) (rewritten string, ok bool) {
	rewritten = quicklinkNamedPlaceholder.ReplaceAllStringFunc(link, func(token string) string {
		name := quicklinkNamedPlaceholder.FindStringSubmatch(token)
		if len(name) != 2 || !parameterNameAllowed(name[1]) {
			return token
		}
		return "{wox:parameter?name=" + name[1] + "}"
	})
	rewritten = quicklinkPlainPlaceholder.ReplaceAllString(rewritten, "{wox:parameter?name=query}")
	for _, token := range quicklinkAnyPlaceholder.FindAllString(rewritten, -1) {
		if strings.HasPrefix(token, "{wox:parameter?name=") && strings.HasSuffix(token, "}") {
			name := strings.TrimSuffix(strings.TrimPrefix(token, "{wox:parameter?name="), "}")
			if parameterNameAllowed(name) {
				continue
			}
		}
		return rewritten, false
	}
	probe := quicklinkAnyPlaceholder.ReplaceAllString(rewritten, "value")
	parsed, err := url.Parse(probe)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return rewritten, false
	}
	return rewritten, true
}

// parameterNameAllowed matches Web Search parameter names: letters, digits, underscores, and spaces.
func parameterNameAllowed(name string) bool {
	if name == "" {
		return false
	}
	runes := []rune(name)
	if unicode.IsSpace(runes[0]) || unicode.IsDigit(runes[0]) || unicode.IsSpace(runes[len(runes)-1]) {
		return false
	}
	for _, current := range runes {
		switch current {
		case '{', '}', '&', '=', '?':
			return false
		}
		if current != ' ' && current != '_' && !unicode.IsLetter(current) && !unicode.IsDigit(current) {
			return false
		}
	}
	return true
}

// quicklinkKeyword builds a trigger keyword. Spaces are not legal trigger characters, so they are removed.
func quicklinkKeyword(name string) (string, bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	name = strings.Join(strings.Fields(name), " ")
	var builder strings.Builder
	for _, current := range name {
		if unicode.IsLetter(current) || unicode.IsDigit(current) || current == '_' {
			builder.WriteRune(current)
		}
	}
	keyword := builder.String()
	if keyword == "" || keyword == "*" || strings.ContainsFunc(keyword, unicode.IsSpace) {
		return keyword, false
	}
	return keyword, true
}

func browserForPath(path string) (id string, recognized bool) {
	base := strings.ToLower(path)
	switch {
	case strings.Contains(base, "chrome"):
		return "chrome", true
	case strings.Contains(base, "msedge"), strings.Contains(base, "edge"):
		return "edge", true
	case strings.Contains(base, "firefox"):
		return "firefox", true
	case strings.Contains(base, "brave"):
		return "brave", true
	case strings.Contains(base, "opera"):
		return "opera", true
	case strings.Contains(base, "chromium"):
		return "chromium", true
	case strings.Contains(base, "safari"):
		return "safari", true
	default:
		return "", false
	}
}
