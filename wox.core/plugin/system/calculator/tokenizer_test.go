package calculator

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func TestTokenize(t *testing.T) {
	tests := []struct {
		input    string
		expected []token
		hasError bool
	}{
		{
			input: "0.01",
			expected: []token{
				{kind: numberToken, val: decimal.NewFromFloat(0.01)},
				{kind: eosToken},
			},
			hasError: false,
		},
		{
			input: ".01",
			expected: []token{
				{kind: numberToken, val: decimal.NewFromFloat(0.01)},
				{kind: eosToken},
			},
			hasError: false,
		},
		{
			input: "1 + .5",
			expected: []token{
				{kind: numberToken, val: decimal.NewFromFloat(1)},
				{kind: reservedToken, str: "+"},
				{kind: numberToken, val: decimal.NewFromFloat(0.5)},
				{kind: eosToken},
			},
			hasError: false,
		},
	}

	for _, test := range tests {
		tokens, err := tokenize(test.input)
		if test.hasError {
			assert.Error(t, err, "Input: %s", test.input)
		} else {
			assert.NoError(t, err, "Input: %s", test.input)
			if err == nil {
				assert.Equal(t, len(test.expected), len(tokens), "Input: %s - Token count mismatch", test.input)
				for i, expectedToken := range test.expected {
					if i >= len(tokens) {
						break
					}
					actualToken := tokens[i]
					assert.Equal(t, expectedToken.kind, actualToken.kind, "Input: %s - Token %d kind mismatch", test.input, i)
					assert.Equal(t, expectedToken.str, actualToken.str, "Input: %s - Token %d str mismatch", test.input, i)
					if expectedToken.kind == numberToken {
						assert.True(t, expectedToken.val.Equal(actualToken.val), "Input: %s - Token %d val mismatch. Expected %s, got %s", test.input, i, expectedToken.val, actualToken.val)
					}
				}
			}
		}
	}
}
