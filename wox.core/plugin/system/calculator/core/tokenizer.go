package core

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/shopspring/decimal"
)

type Tokenizer struct {
	patterns []TokenPattern
}

func NewTokenizer(patterns []TokenPattern) *Tokenizer {
	// Sort patterns by priority
	sorted := make([]TokenPattern, len(patterns))
	copy(sorted, patterns)
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i].Priority < sorted[j].Priority {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	return &Tokenizer{patterns: sorted}
}

type invalidTokenError struct {
	input    string
	position int
}

func (e *invalidTokenError) Error() string {
	curr := ""
	pos := e.position
	for _, line := range strings.Split(e.input, "\n") {
		len := len(line)
		curr += line + "\n"
		if pos < len {
			return curr + strings.Repeat(" ", pos) + "^ invalid token"
		}
		pos -= len + 1
	}
	return ""
}

func (t *Tokenizer) Tokenize(ctx context.Context, input string) ([]Token, error) {
	chars := []rune(input)
	i := 0
	n := len(chars)
	tokens := []Token{}

	for i < n {
		char := chars[i]
		if unicode.IsSpace(char) {
			i++
			continue
		}

		// Try to match each pattern
		matched := false
		for _, pattern := range t.patterns {
			var re *regexp.Regexp
			var match string

			if pattern.FullMatch {
				// For full match patterns, try to match the entire remaining input
				re = regexp.MustCompile("^" + pattern.Pattern + "$")
				match = re.FindString(strings.TrimSpace(string(chars[i:])))
				if match != "" {
					tokens = append(tokens, Token{Kind: pattern.Type, Str: match})
					i = n // Move to the end
					matched = true
					break
				}
			} else {
				// For partial match patterns, match from current position
				re = regexp.MustCompile("^" + pattern.Pattern)
				match = re.FindString(string(chars[i:]))
				if match != "" {
					if pattern.Type == NumberToken {
						val, err := strconv.ParseFloat(match, 64)
						if err != nil {
							return nil, err
						}
						tokens = append(tokens, Token{Kind: NumberToken, Val: decimal.NewFromFloat(val)})
					} else {
						tokens = append(tokens, Token{Kind: pattern.Type, Str: match})
					}
					i += len([]rune(match))
					matched = true
					break
				}
			}
		}

		if !matched {
			// Special handling for operators
			if strings.ContainsRune("+-*/(),", char) {
				tokens = append(tokens, Token{Kind: ReservedToken, Str: string(char)})
				i++
				matched = true
			}
		}

		if !matched {
			return nil, &invalidTokenError{input: input, position: i}
		}
	}

	tokens = append(tokens, Token{Kind: EosToken})
	return tokens, nil
}
