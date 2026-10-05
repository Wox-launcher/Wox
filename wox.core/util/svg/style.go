package svg

import (
	"bytes"
	"encoding/xml"
	"sort"
	"strings"
)

// cssRule is one simple selector from an author <style> element.
type cssRule struct {
	selector    cssSelector
	properties  map[string]string
	specificity int
	order       int
}

// cssSelector is a type, id, and class list with no combinators.
// Descendant selectors and pseudo-classes are rejected so a rule cannot match
// more elements than its text literally names.
type cssSelector struct {
	tag       string
	id        string
	classes   []string
	universal bool
}

// collectStyleRules reads every <style> element before shapes are parsed.
// Desktop icons such as Code - OSS keep their colors in class rules
// (`.st2{fill:#167abf}`) instead of presentation attributes. The block is
// allowed to follow the paths it colors. XML comments inside <style> are
// kept because older exporters hide the sheet from legacy HTML parsers that way.
func collectStyleRules(data []byte) []cssRule {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	depth := 0
	var sheet strings.Builder
	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}
		switch element := token.(type) {
		case xml.StartElement:
			if element.Name.Local == "style" {
				depth++
			}
		case xml.EndElement:
			if element.Name.Local == "style" && depth > 0 {
				depth--
			}
		case xml.CharData:
			if depth > 0 {
				sheet.Write(element)
			}
		case xml.Comment:
			if depth > 0 {
				sheet.Write(element)
			}
		}
	}
	return parseStyleSheet(sheet.String())
}

// parseStyleSheet reads the class and type rules exported by icon editors.
// At-rules such as @media are skipped, and a declaration this renderer cannot
// apply is dropped so one unsupported value does not reject the icon.
func parseStyleSheet(input string) []cssRule {
	text := stripCSSComments(input)
	rules := []cssRule{}
	order := 0
	for i := 0; i < len(text); {
		i = skipCSSSpace(text, i)
		if i >= len(text) {
			break
		}
		if text[i] == '@' {
			i = skipCSSAtRule(text, i)
			continue
		}
		brace := indexUnquoted(text, i, '{')
		if brace < 0 {
			break
		}
		selectorText := text[i:brace]
		end := indexUnquoted(text, brace+1, '}')
		if end < 0 {
			break
		}
		properties := supportedStyleProperties(parseDeclarations(text[brace+1 : end]))
		if len(properties) > 0 {
			for _, part := range strings.Split(selectorText, ",") {
				selector, ok := parseSimpleSelector(part)
				if !ok {
					continue
				}
				rules = append(rules, cssRule{
					selector:    selector,
					properties:  properties,
					specificity: selector.specificity(),
					order:       order,
				})
				order++
			}
		}
		i = end + 1
	}
	return rules
}

func matchingStyleProperties(tag string, attributes map[string]string, rules []cssRule) map[string]string {
	if len(rules) == 0 {
		return nil
	}
	id := attributes["id"]
	classes := strings.Fields(attributes["class"])
	matched := make([]cssRule, 0, 4)
	for _, rule := range rules {
		if rule.selector.matches(tag, id, classes) {
			matched = append(matched, rule)
		}
	}
	if len(matched) == 0 {
		return nil
	}
	sort.SliceStable(matched, func(i, j int) bool {
		if matched[i].specificity != matched[j].specificity {
			return matched[i].specificity < matched[j].specificity
		}
		return matched[i].order < matched[j].order
	})
	properties := make(map[string]string)
	for _, rule := range matched {
		for key, value := range rule.properties {
			properties[key] = value
		}
	}
	return properties
}

func (selector cssSelector) matches(tag string, id string, classes []string) bool {
	if selector.id != "" && selector.id != id {
		return false
	}
	if selector.tag != "" && selector.tag != tag {
		return false
	}
	for _, class := range selector.classes {
		found := false
		for _, candidate := range classes {
			if candidate == class {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return selector.universal || selector.tag != "" || selector.id != "" || len(selector.classes) > 0
}

func (selector cssSelector) specificity() int {
	score := len(selector.classes) * 10
	if selector.id != "" {
		score += 100
	}
	if selector.tag != "" {
		score++
	}
	return score
}

func parseSimpleSelector(selector string) (cssSelector, bool) {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return cssSelector{}, false
	}
	if strings.ContainsAny(selector, " \t\n\r\f>+~:[]()") || strings.Contains(selector, "\\") {
		return cssSelector{}, false
	}
	var parsed cssSelector
	for i := 0; i < len(selector); {
		switch selector[i] {
		case '.':
			name, next, ok := readCSSIdent(selector, i+1)
			if !ok {
				return cssSelector{}, false
			}
			parsed.classes = append(parsed.classes, name)
			i = next
		case '#':
			if parsed.id != "" {
				return cssSelector{}, false
			}
			name, next, ok := readCSSIdent(selector, i+1)
			if !ok {
				return cssSelector{}, false
			}
			parsed.id = name
			i = next
		case '*':
			if parsed.tag != "" || parsed.universal {
				return cssSelector{}, false
			}
			parsed.universal = true
			i++
		default:
			if parsed.tag != "" || parsed.universal {
				return cssSelector{}, false
			}
			name, next, ok := readCSSIdent(selector, i)
			if !ok {
				return cssSelector{}, false
			}
			parsed.tag = name
			i = next
		}
	}
	if parsed.tag == "" && parsed.id == "" && len(parsed.classes) == 0 && !parsed.universal {
		return cssSelector{}, false
	}
	return parsed, true
}

func readCSSIdent(value string, start int) (string, int, bool) {
	if start >= len(value) {
		return "", start, false
	}
	i := start
	first := value[i]
	if first == '-' {
		if i+1 >= len(value) || !isCSSIdentStart(value[i+1]) {
			return "", start, false
		}
		i += 2
	} else if !isCSSIdentStart(first) {
		return "", start, false
	} else {
		i++
	}
	for i < len(value) && isCSSIdentContinue(value[i]) {
		i++
	}
	return value[start:i], i, true
}

func isCSSIdentStart(char byte) bool {
	return char == '_' || (char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z')
}

func isCSSIdentContinue(char byte) bool {
	return isCSSIdentStart(char) || (char >= '0' && char <= '9') || char == '-'
}

func parseDeclarations(body string) map[string]string {
	properties := map[string]string{}
	for _, part := range strings.Split(body, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		key, value, ok := strings.Cut(part, ":")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.TrimSpace(value)
		if strings.HasSuffix(strings.ToLower(value), "!important") {
			value = strings.TrimSpace(value[:len(value)-len("!important")])
		}
		if key == "" || value == "" {
			continue
		}
		properties[key] = value
	}
	return properties
}

// supportedStyleProperties drops declarations this renderer cannot apply.
func supportedStyleProperties(properties map[string]string) map[string]string {
	if len(properties) == 0 {
		return nil
	}
	kept := make(map[string]string, len(properties))
	for key, value := range properties {
		if _, err := applyProperties(defaultPathStyle(), map[string]string{key: value}); err != nil {
			continue
		}
		kept[key] = value
	}
	if len(kept) == 0 {
		return nil
	}
	return kept
}

func stripCSSComments(input string) string {
	var builder strings.Builder
	builder.Grow(len(input))
	for i := 0; i < len(input); {
		if i+1 < len(input) && input[i] == '/' && input[i+1] == '*' {
			end := strings.Index(input[i+2:], "*/")
			if end < 0 {
				break
			}
			i += end + 4
			builder.WriteByte(' ')
			continue
		}
		builder.WriteByte(input[i])
		i++
	}
	return builder.String()
}

func skipCSSSpace(text string, i int) int {
	for i < len(text) {
		switch text[i] {
		case ' ', '\t', '\n', '\r', '\f':
			i++
		default:
			return i
		}
	}
	return i
}

func skipCSSAtRule(text string, i int) int {
	quote := byte(0)
	for ; i < len(text); i++ {
		char := text[i]
		if quote != 0 {
			if char == '\\' && i+1 < len(text) {
				i++
				continue
			}
			if char == quote {
				quote = 0
			}
			continue
		}
		switch char {
		case '"', '\'':
			quote = char
		case ';':
			return i + 1
		case '{':
			return skipCSSBlock(text, i)
		}
	}
	return i
}

func skipCSSBlock(text string, i int) int {
	depth := 0
	quote := byte(0)
	for ; i < len(text); i++ {
		char := text[i]
		if quote != 0 {
			if char == '\\' && i+1 < len(text) {
				i++
				continue
			}
			if char == quote {
				quote = 0
			}
			continue
		}
		switch char {
		case '"', '\'':
			quote = char
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i + 1
			}
		}
	}
	return i
}

func indexUnquoted(text string, start int, target byte) int {
	quote := byte(0)
	for i := start; i < len(text); i++ {
		char := text[i]
		if quote != 0 {
			if char == '\\' && i+1 < len(text) {
				i++
				continue
			}
			if char == quote {
				quote = 0
			}
			continue
		}
		if char == '"' || char == '\'' {
			quote = char
			continue
		}
		if char == target {
			return i
		}
	}
	return -1
}
