package calculator

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func TestTokenize(t *testing.T) {
	tests := []struct {
		input        string
		thousandsSep string
		decimalSep   string
		expected     []token
		hasError     bool
	}{
		{
			input:        "0.01",
			thousandsSep: "",
			decimalSep:   ".",
			expected: []token{
				{kind: numberToken, val: decimal.NewFromFloat(0.01)},
				{kind: eosToken},
			},
			hasError: false,
		},
		{
			input:        ".01",
			thousandsSep: "",
			decimalSep:   ".",
			expected: []token{
				{kind: numberToken, val: decimal.NewFromFloat(0.01)},
				{kind: eosToken},
			},
			hasError: false,
		},
		{
			input:        "1 + .5",
			thousandsSep: "",
			decimalSep:   ".",
			expected: []token{
				{kind: numberToken, val: decimal.NewFromFloat(1)},
				{kind: reservedToken, str: "+"},
				{kind: numberToken, val: decimal.NewFromFloat(0.5)},
				{kind: eosToken},
			},
			hasError: false,
		},
		// Test US/Standard format (Comma thousands, Dot decimal)
		{
			input:        "1,234.56",
			thousandsSep: ",",
			decimalSep:   ".",
			expected: []token{
				{kind: numberToken, val: decimal.NewFromFloat(1234.56)},
				{kind: eosToken},
			},
			hasError: false,
		},
		// Test European format (Dot thousands, Comma decimal)
		{
			input:        "1.234,56",
			thousandsSep: ".",
			decimalSep:   ",",
			expected: []token{
				{kind: numberToken, val: decimal.NewFromFloat(1234.56)},
				{kind: eosToken},
			},
			hasError: false,
		},
		// Test SI format (Space thousands, Comma decimal)
		{
			input:        "1 234,56",
			thousandsSep: " ",
			decimalSep:   ",",
			expected: []token{
				{kind: numberToken, val: decimal.NewFromFloat(1234.56)},
				{kind: eosToken},
			},
			hasError: false,
		},
		// Test SI format with narrow no-break space
		{
			input:        "1\u202F234,56",
			thousandsSep: " ",
			decimalSep:   ",",
			expected: []token{
				{kind: numberToken, val: decimal.NewFromFloat(1234.56)},
				{kind: eosToken},
			},
			hasError: false,
		},
		// Test Swiss format (Apostrophe thousands, Dot decimal)
		{
			input:        "1'234.56",
			thousandsSep: "'",
			decimalSep:   ".",
			expected: []token{
				{kind: numberToken, val: decimal.NewFromFloat(1234.56)},
				{kind: eosToken},
			},
			hasError: false,
		},
		// Test Argument Separation Conflict: max(1,2) with Comma Decimal
		// Should parse as one number 1.2
		{
			input:        "max(1,2)",
			thousandsSep: ".",
			decimalSep:   ",",
			expected: []token{
				{kind: identToken, str: "max"},
				{kind: reservedToken, str: "("},
				{kind: numberToken, val: decimal.NewFromFloat(1.2)},
				{kind: reservedToken, str: ")"},
				{kind: eosToken},
			},
			hasError: false,
		},
		// Test Argument Separation with Semicolon: max(1;2) with Comma Decimal
		{
			input:        "max(1;2)",
			thousandsSep: ".",
			decimalSep:   ",",
			expected: []token{
				{kind: identToken, str: "max"},
				{kind: reservedToken, str: "("},
				{kind: numberToken, val: decimal.NewFromFloat(1)},
				{kind: reservedToken, str: ";"},
				{kind: numberToken, val: decimal.NewFromFloat(2)},
				{kind: reservedToken, str: ")"},
				{kind: eosToken},
			},
			hasError: false,
		},
		// Test Argument Separation Conflict: max(1, 2) with Comma Decimal (Space separates)
		// Should parse as 1, separator, 2. But wait, token logic consumes comma as decimal if followed by digit.
		// Space breaks the "followed by digit" check? No, space is after comma.
		// If input is "1, 2". "1" is parsed. "," is checked. Is next char digit? No, space.
		// So "," is NOT consumed as decimal. It becomes reservedToken.
		{
			input:        "max(1, 2)",
			thousandsSep: ".",
			decimalSep:   ",",
			expected: []token{
				{kind: identToken, str: "max"},
				{kind: reservedToken, str: "("},
				{kind: numberToken, val: decimal.NewFromFloat(1)},
				{kind: reservedToken, str: ","},
				{kind: numberToken, val: decimal.NewFromFloat(2)},
				{kind: reservedToken, str: ")"},
				{kind: eosToken},
			},
			hasError: false,
		},
	}

	for _, test := range tests {
		tokens, err := tokenize(test.input, test.thousandsSep, test.decimalSep)
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
