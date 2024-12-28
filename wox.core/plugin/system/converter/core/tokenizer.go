package core

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/shopspring/decimal"
)

type TokenKind int

const (
	UnknownToken    TokenKind = iota // For error handling
	NumberToken                      // For numbers (e.g., 100, 3.14)
	IdentToken                       // For identifiers and keywords (e.g., USD, in, to)
	OperationToken                   // For operators and special characters (e.g., +, -, *, /)
	ConversionToken                  // For conversion directives (e.g., in EUR, to USD)
	EosToken                         // End of stream token
)

func (t TokenKind) String() string {
	switch t {
	case NumberToken:
		return "NumberToken"
	case IdentToken:
		return "IdentToken"
	case OperationToken:
		return "OperationToken"
	case ConversionToken:
		return "ConversionToken"
	case EosToken:
		return "EosToken"
	}
	return "UnknownToken"
}

type Token struct {
	Kind   TokenKind
	Val    decimal.Decimal // Only used for NumberToken
	Str    string          // Original string representation
	Module Module          // The module that parsed this token
}

func (t *Token) String() string {
	return t.Str
}

type TokenPattern struct {
	Pattern   string    // Regex pattern for matching
	Type      TokenKind // Type of token this pattern produces
	Priority  int       // Higher priority patterns are matched first
	FullMatch bool      // Whether this pattern should match the entire input
	Module    Module    // The module that owns this pattern
}

type Tokenizer struct {
	patterns []TokenPattern
}

func NewTokenizer(patterns []TokenPattern) *Tokenizer {
	// Sort patterns by priority (highest first)
	for i := 0; i < len(patterns)-1; i++ {
		for j := i + 1; j < len(patterns); j++ {
			if patterns[i].Priority < patterns[j].Priority {
				patterns[i], patterns[j] = patterns[j], patterns[i]
			}
		}
	}

	return &Tokenizer{patterns: patterns}
}

func (t *Tokenizer) Tokenize(ctx context.Context, input string) ([]Token, error) {
	var tokens []Token
	input = strings.TrimSpace(input)

	// Try full match patterns first
	for _, pattern := range t.patterns {
		if !pattern.FullMatch {
			continue
		}
		re := regexp.MustCompile(`^` + pattern.Pattern + `$`)
		if re.MatchString(input) {
			// For full match patterns, we create a single token with the entire input
			token := Token{
				Kind:   pattern.Type,
				Str:    input,
				Module: pattern.Module,
			}
			if pattern.Type == NumberToken {
				// Only parse decimal value for number tokens
				if val, err := decimal.NewFromString(input); err == nil {
					token.Val = val
				}
			}
			return []Token{token, {Kind: EosToken}}, nil
		}
	}

	// If no full match, tokenize normally
	for len(input) > 0 {
		input = strings.TrimSpace(input)
		if len(input) == 0 {
			break
		}

		matched := false
		for _, pattern := range t.patterns {
			if pattern.FullMatch {
				continue
			}

			re := regexp.MustCompile(`^` + pattern.Pattern)
			if matches := re.FindStringSubmatch(input); len(matches) > 0 {
				// Create a single token for the entire match
				token := Token{
					Kind:   pattern.Type,
					Str:    matches[0],
					Module: pattern.Module,
				}

				// If it's a number token, parse the value
				if pattern.Type == NumberToken {
					if val, err := decimal.NewFromString(matches[0]); err == nil {
						token.Val = val
					}
				}

				tokens = append(tokens, token)
				input = input[len(matches[0]):]
				matched = true
				break
			}
		}

		if !matched {
			// Try to match operators
			if strings.HasPrefix(input, "+") || strings.HasPrefix(input, "-") ||
				strings.HasPrefix(input, "*") || strings.HasPrefix(input, "/") {
				tokens = append(tokens, Token{
					Kind:   OperationToken,
					Str:    input[0:1],
					Module: nil,
				})
				input = input[1:]
				continue
			}

			return nil, fmt.Errorf("invalid token at: %s", input)
		}
	}

	tokens = append(tokens, Token{Kind: EosToken})
	return tokens, nil
}
