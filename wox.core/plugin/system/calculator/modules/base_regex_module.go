package modules

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"wox/plugin"
	"wox/plugin/system/calculator/core"
)

// PatternHandler represents a pattern and its corresponding handler
type patternHandler struct {
	Pattern     string                                                            // regex pattern
	Priority    int                                                               // pattern priority
	Handler     func(ctx context.Context, matches []string) (*core.Result, error) // handler function for the pattern
	Description string                                                            // description of what this pattern does
	regexp      *regexp.Regexp                                                    // compiled regexp
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
			FullMatch: true,
		})
	}
	return patterns
}

// CanHandle checks if this module can handle the given tokens
func (m *regexBaseModule) CanHandle(ctx context.Context, tokens []core.Token) bool {
	if len(tokens) == 0 {
		m.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("%s.CanHandle: no tokens", m.name))
		return false
	}

	inputStr := m.getInputString(tokens)
	m.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("%s.CanHandle: input=%s", m.name, inputStr))

	for _, handler := range m.patternHandlers {
		if handler.regexp.MatchString(inputStr) {
			m.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("%s.CanHandle: matched pattern %s", m.name, handler.Description))
			return true
		}
	}

	m.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("%s.CanHandle: no pattern matched", m.name))
	return false
}

// Parse parses the tokens using the registered patterns
func (m *regexBaseModule) Parse(ctx context.Context, tokens []core.Token) (*core.Result, error) {
	if len(tokens) == 0 {
		return nil, fmt.Errorf("no tokens to parse")
	}

	inputStr := m.getInputString(tokens)
	m.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("%s.Parse: input=%s", m.name, inputStr))

	for _, handler := range m.patternHandlers {
		if matches := handler.regexp.FindStringSubmatch(inputStr); len(matches) > 0 {
			m.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("Matched pattern %s with matches: %v", handler.Description, matches))
			return handler.Handler(ctx, matches)
		}
	}

	return nil, fmt.Errorf("unsupported format")
}

// Calculate performs the calculation using the Parse method
func (m *regexBaseModule) Calculate(ctx context.Context, tokens []core.Token) (*core.Result, error) {
	return m.Parse(ctx, tokens)
}

// Helper functions

func (m *regexBaseModule) getInputString(tokens []core.Token) string {
	var input strings.Builder
	for _, token := range tokens {
		input.WriteString(token.Str)
		input.WriteString(" ")
	}
	return strings.TrimSpace(strings.ToLower(input.String()))
}
