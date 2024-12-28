package modules

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"wox/plugin"
	"wox/plugin/system/converter/core"
)

// PatternHandler represents a pattern and its corresponding handler
type patternHandler struct {
	Pattern     string                                                           // regex pattern
	Priority    int                                                              // pattern priority
	Handler     func(ctx context.Context, matches []string) (core.Result, error) // handler function for the pattern
	Description string                                                           // description of what this pattern does
	FullMatch   bool                                                             // whether the pattern is a full match
	regexp      *regexp.Regexp                                                   // compiled regexp
}

// regexBaseModule is a base module that provides regex-based pattern matching functionality
type regexBaseModule struct {
	api             plugin.API
	name            string
	patternHandlers []*patternHandler
}

// NewregexBaseModule creates a new regexBaseModule
func NewRegexBaseModule(api plugin.API, name string, handlers []*patternHandler) *regexBaseModule {
	m := &regexBaseModule{
		api:             api,
		name:            name,
		patternHandlers: handlers,
	}

	// Compile all regexps
	for _, handler := range handlers {
		handler.regexp = regexp.MustCompile(handler.Pattern)
	}

	return m
}

// Name returns the name of the module
func (m *regexBaseModule) Name() string {
	return m.name
}

// TokenPatterns returns the token patterns for this module
func (m *regexBaseModule) TokenPatterns() []core.TokenPattern {
	patterns := make([]core.TokenPattern, 0, len(m.patternHandlers))
	for _, handler := range m.patternHandlers {
		patterns = append(patterns, core.TokenPattern{
			Pattern:   handler.Pattern,
			Type:      core.IdentToken,
			Priority:  handler.Priority,
			FullMatch: handler.FullMatch,
			Module:    m,
		})
	}
	return patterns
}

// Calculate performs the calculation using the Parse method
func (m *regexBaseModule) Calculate(ctx context.Context, token core.Token) (core.Result, error) {
	if token.Kind == core.EosToken {
		return core.Result{}, fmt.Errorf("cannot calculate EOS token")
	}

	m.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("Processing in module %s", m.name))

	for _, handler := range m.patternHandlers {
		if matches := handler.regexp.FindStringSubmatch(strings.ToLower(token.Str)); len(matches) > 0 {
			result, err := handler.Handler(ctx, matches)
			if err != nil {
				m.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("	=> Pattern '%s': %v", handler.Description, err))
				continue
			}

			//log matches, but ignore the first match which is the full match
			m.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("	=> Pattern '%s' matched: %v", handler.Description, strings.Join(matches[1:], ", ")))
			m.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("	=> matched result: value=%s, raw=%s, unit=%s", result.DisplayValue, result.RawValue, result.Unit.Name))
			return result, nil
		}
	}

	m.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("	=> No matching patterns found for token '%s'", token.Str))
	return core.Result{}, fmt.Errorf("unsupported format")
}

func (m *regexBaseModule) Convert(ctx context.Context, value core.Result, toUnit core.Unit) (core.Result, error) {
	return core.Result{}, fmt.Errorf("conversion not supported")
}

func (m *regexBaseModule) getInputString(tokens []core.Token) string {
	var input strings.Builder
	for _, token := range tokens {
		input.WriteString(token.Str)
		input.WriteString(" ")
	}
	return strings.TrimSpace(strings.ToLower(input.String()))
}
