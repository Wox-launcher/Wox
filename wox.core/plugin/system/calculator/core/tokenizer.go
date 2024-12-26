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
	UnknownToken  TokenKind = iota // For error handling
	NumberToken                    // For numbers (e.g., 100, 3.14)
	IdentToken                     // For identifiers and keywords (e.g., USD, in, to)
	ReservedToken                  // For operators and special characters (e.g., +, -, *, /)
	EosToken                       // End of stream token
)

type Token struct {
	Kind TokenKind
	Val  decimal.Decimal // Only used for NumberToken
	Str  string          // Original string representation
}

func (t *Token) String() string {
	return t.Str
}

type TokenPattern struct {
	Pattern   string    // Regex pattern for matching
	Type      TokenKind // Type of token this pattern produces
	Priority  int       // Higher priority patterns are matched first
	FullMatch bool      // Whether this pattern should match the entire input
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
			token := Token{Kind: pattern.Type, Str: input}
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
			if matches := re.FindString(input); matches != "" {
				token := Token{Kind: pattern.Type, Str: matches}
				if pattern.Type == NumberToken {
					// Only parse decimal value for number tokens
					if val, err := decimal.NewFromString(matches); err == nil {
						token.Val = val
					}
				}
				tokens = append(tokens, token)
				input = input[len(matches):]
				matched = true
				break
			}
		}

		if !matched {
			return nil, fmt.Errorf("invalid token at: %s", input)
		}
	}

	tokens = append(tokens, Token{Kind: EosToken})
	return tokens, nil
}
