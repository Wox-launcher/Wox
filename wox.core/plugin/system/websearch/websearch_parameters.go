package websearch

import (
	"errors"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"wox/common"
	"wox/plugin"
)

type searchTemplateError struct {
	Field string
	Key   string
	Token string
}

func (e searchTemplateError) Error() string {
	if e.Token == "" {
		return e.Key
	}
	return e.Key + " " + e.Token
}

// ValidateSettingFields maps Title/URL template problems onto the matching editor fields.
func ValidateSettingFields(title string, urls []string) map[string]string {
	_, err := (webSearch{Title: title, Urls: urls}).parameters()
	if err == nil {
		return nil
	}
	var templateErr searchTemplateError
	if errors.As(err, &templateErr) {
		return map[string]string{templateErr.Field: templateErr.Key}
	}
	return map[string]string{"Urls": err.Error()}
}

var webSearchVariable = regexp.MustCompile(`\{wox:[^{}]*\}`)

// queryVariables declares only the environment values used by this entry.
func (s webSearch) queryVariables() []plugin.QueryVariable {
	text := s.Title + strings.Join(s.Urls, "")
	var variables []plugin.QueryVariable
	for _, variable := range []string{plugin.QueryVariableSelectedText, plugin.QueryVariableClipboardText} {
		if strings.Contains(text, variable) {
			variables = append(variables, variable)
		}
	}
	return variables
}

// webSearchParameter identifies input variables from the unified {wox:parameter?name=} form.
func webSearchParameter(token string) (string, bool) {
	ref, ok := plugin.ParseQueryVariable(token)
	if !ok || ref.Name != "parameter" {
		return "", false
	}
	name := ref.Params["name"]
	return name, isValidWebSearchParameterName(name)
}

// webSearchParameterHasInvalidName reports a parsed parameter whose name is not allowed.
func webSearchParameterHasInvalidName(token string) bool {
	ref, ok := plugin.ParseQueryVariable(token)
	return ok && ref.Name == "parameter" && !isValidWebSearchParameterName(ref.Params["name"])
}

// isValidWebSearchParameterName allows words in any language, plus digits, underscores and spaces.
func isValidWebSearchParameterName(name string) bool {
	if name == "" {
		return false
	}
	runes := []rune(name)
	if unicode.IsSpace(runes[0]) || unicode.IsDigit(runes[0]) {
		return false
	}
	if unicode.IsSpace(runes[len(runes)-1]) {
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

// parameters derives stable input order from URLs; titles can only reference those inputs.
func (s webSearch) parameters() ([]string, error) {
	var names []string
	if err := validateWebSearchTemplateSyntax(s.Urls, "Urls"); err != nil {
		return nil, err
	}
	if err := validateWebSearchTemplateSyntax([]string{s.Title}, "Title"); err != nil {
		return nil, err
	}
	for _, template := range s.Urls {
		parsed, err := url.Parse(webSearchVariable.ReplaceAllString(template, "value"))
		if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return nil, searchTemplateError{Field: "Urls", Key: "i18n:plugin_websearch_error_invalid_url"}
		}
		authorityStart := strings.Index(template, "://") + 3
		authorityEnd := strings.IndexAny(template[authorityStart:], "/?#")
		if authorityEnd < 0 {
			authorityEnd = len(template)
		} else {
			authorityEnd += authorityStart
		}
		for _, match := range webSearchVariable.FindAllStringIndex(template, -1) {
			if match[0] < authorityEnd {
				return nil, searchTemplateError{Field: "Urls", Key: "i18n:plugin_websearch_error_variable_in_host"}
			}
			token := template[match[0]:match[1]]
			if name, ok := webSearchParameter(token); ok {
				if !slices.Contains(names, name) {
					names = append(names, name)
				}
			} else if webSearchParameterHasInvalidName(token) {
				return nil, searchTemplateError{Field: "Urls", Key: "i18n:plugin_websearch_error_invalid_parameter_name", Token: token}
			} else if token != plugin.QueryVariableSelectedText && token != plugin.QueryVariableClipboardText {
				return nil, searchTemplateError{Field: "Urls", Key: "i18n:plugin_websearch_error_unknown_variable", Token: token}
			}
		}
	}
	for _, token := range webSearchVariable.FindAllString(s.Title, -1) {
		name, ok := webSearchParameter(token)
		if ok && !slices.Contains(names, name) {
			return nil, searchTemplateError{Field: "Title", Key: "i18n:plugin_websearch_error_unknown_title_variable", Token: token}
		}
		if !ok && webSearchParameterHasInvalidName(token) {
			return nil, searchTemplateError{Field: "Title", Key: "i18n:plugin_websearch_error_invalid_parameter_name", Token: token}
		}
		if !ok && token != plugin.QueryVariableSelectedText && token != plugin.QueryVariableClipboardText {
			return nil, searchTemplateError{Field: "Title", Key: "i18n:plugin_websearch_error_unknown_title_variable", Token: token}
		}
	}
	return names, nil
}

// validateWebSearchTemplateSyntax rejects unclosed {wox: tokens and removed {query} aliases.
func validateWebSearchTemplateSyntax(templates []string, field string) error {
	for _, template := range templates {
		if strings.Contains(webSearchVariable.ReplaceAllString(template, ""), "{wox:") {
			return searchTemplateError{Field: field, Key: "i18n:plugin_websearch_error_unclosed_variable"}
		}
		if strings.Contains(template, "{query}") || strings.Contains(template, "{lower_query}") || strings.Contains(template, "{upper_query}") {
			return searchTemplateError{Field: field, Key: "i18n:plugin_websearch_error_unknown_variable"}
		}
	}
	return nil
}

// queryHint uses namespaced IDs so a parameter named "command" cannot collide with the core prefix.
func (s webSearch) queryHint(names []string) *common.QueryHint {
	if len(names) == 0 {
		return nil
	}
	hint := &common.QueryHint{}
	for i, name := range names {
		if i > 0 {
			hint.Elements = append(hint.Elements, common.QueryElement{Id: "separator:" + name, Kind: common.QueryElementText, Text: " "})
		}
		hint.Elements = append(hint.Elements, common.QueryElement{Id: "parameter:" + name, Kind: common.QueryElementArgument, Placeholder: common.I18nString(name), Required: true})
	}
	return hint
}

// parameterValues keeps single-input compatibility without guessing boundaries in multi-input plain text.
func (s webSearch) parameterValues(query plugin.Query, names []string) (map[string]string, bool) {
	values := make(map[string]string, len(names))
	for _, name := range names {
		value := ""
		if query.QueryHint != nil {
			value = query.QueryHint.Argument("parameter:" + name)
		} else if len(names) == 1 {
			_, value, _ = strings.Cut(query.RawQuery, " ")
		}
		if strings.TrimSpace(value) == "" {
			return values, false
		}
		values[plugin.ParameterQueryVariable(name)] = value
	}
	return values, true
}

// renderWebSearchTemplate substitutes original tokens once; inserted text is never interpreted as another variable.
func renderWebSearchTemplate(template string, values map[string]string, isURL bool) string {
	var output strings.Builder
	previous := 0
	for _, match := range webSearchVariable.FindAllStringIndex(template, -1) {
		output.WriteString(template[previous:match[0]])
		token := template[match[0]:match[1]]
		value := values[token]
		if ref, ok := plugin.ParseQueryVariable(token); ok && ref.Name == "parameter" {
			value = values[plugin.ParameterQueryVariable(ref.Params["name"])]
			switch ref.Params["case"] {
			case "lower":
				value = strings.ToLower(value)
			case "upper":
				value = strings.ToUpper(value)
			}
		}
		if isURL {
			prefix := template[:match[0]]
			if strings.Contains(prefix, "?") && !strings.Contains(prefix, "#") {
				value = url.QueryEscape(value)
			} else {
				value = url.PathEscape(value)
			}
		}
		output.WriteString(value)
		previous = match[1]
	}
	output.WriteString(template[previous:])
	return output.String()
}
