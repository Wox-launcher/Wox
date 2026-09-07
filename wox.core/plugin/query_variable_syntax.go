package plugin

import (
	"regexp"
	"sort"
	"strings"
)

var queryVariableName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// QueryVariableRef is one {wox:name} or {wox:name?key=value} placeholder.
type QueryVariableRef struct {
	Name   string
	Params map[string]string
}

// ParseQueryVariable reads the unified {wox:name} or {wox:name?key=value} form.
func ParseQueryVariable(token string) (QueryVariableRef, bool) {
	if !strings.HasPrefix(token, "{wox:") || !strings.HasSuffix(token, "}") {
		return QueryVariableRef{}, false
	}
	body := token[len("{wox:") : len(token)-1]
	name, query, hasQuery := strings.Cut(body, "?")
	if !queryVariableName.MatchString(name) {
		return QueryVariableRef{}, false
	}
	if !hasQuery {
		return QueryVariableRef{Name: name}, true
	}
	params := make(map[string]string)
	for _, part := range strings.Split(query, "&") {
		key, value, ok := strings.Cut(part, "=")
		if !ok || !queryVariableName.MatchString(key) {
			return QueryVariableRef{}, false
		}
		params[key] = value
	}
	return QueryVariableRef{Name: name, Params: params}, true
}

// FormatQueryVariable writes {wox:name} or {wox:name?key=value} with name first.
func FormatQueryVariable(name string, params map[string]string) string {
	if len(params) == 0 {
		return "{wox:" + name + "}"
	}
	keys := make([]string, 0, len(params))
	for key := range params {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i] == "name" {
			return true
		}
		if keys[j] == "name" {
			return false
		}
		return keys[i] < keys[j]
	})
	var output strings.Builder
	output.WriteString("{wox:")
	output.WriteString(name)
	output.WriteByte('?')
	for index, key := range keys {
		if index > 0 {
			output.WriteByte('&')
		}
		output.WriteString(key)
		output.WriteByte('=')
		output.WriteString(params[key])
	}
	output.WriteByte('}')
	return output.String()
}

// ParameterQueryVariable is the canonical Web Search input placeholder.
func ParameterQueryVariable(name string) string {
	return FormatQueryVariable("parameter", map[string]string{"name": name})
}
