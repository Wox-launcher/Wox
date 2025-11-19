package calculator

import (
	"errors"
	"strconv"
	"strings"
	"unicode"

	"github.com/shopspring/decimal"
)

type tokenKind string

const (
	reservedToken tokenKind = "reserved"
	numberToken   tokenKind = "number"
	identToken    tokenKind = "ident"
	eosToken      tokenKind = "eos"
)

type token struct {
	kind tokenKind
	val  decimal.Decimal
	str  string
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

const operators = "+-*/^(),"

func isOperator(char rune) bool {
	for _, op := range operators {
		if char == op {
			return true
		}
	}
	return false
}

func numberPrefix(chars []rune, i *int, n int) (float64, error) {
	start := *i
	current := *i

	// Consume digits
	for current < n && (chars[current] >= '0' && chars[current] <= '9') {
		current++
	}

	// Consume dot and subsequent digits
	if current < n && chars[current] == '.' {
		current++
		for current < n && (chars[current] >= '0' && chars[current] <= '9') {
			current++
		}
	}

	if current == start {
		return 0, errors.New("expected a number")
	}

	candidate := string(chars[start:current])
	if candidate == "." {
		return 0, errors.New("expected a number")
	}

	val, err := strconv.ParseFloat(candidate, 64)
	if err != nil {
		return 0, err
	}

	*i = current
	return val, nil
}

func isAlpha(char rune) bool {
	return (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z')
}

func isAlNum(char rune) bool {
	return isAlpha(char) || (char >= '0' && char <= '9')
}

func tokenize(input string) ([]token, error) {
	chars := []rune(input)
	i := 0
	n := len(chars)
	tokens := []token{}
	for i < n {
		char := chars[i]
		if unicode.IsSpace(char) {
			i++
			continue
		}

		if isAlpha(char) {
			start := i
			i++
			for i < n && isAlNum(chars[i]) {
				i++
			}
			tokens = append(tokens,
				token{kind: identToken, str: string(chars[start:i])})
			continue
		}

		if isOperator(char) {
			tokens = append(tokens, token{kind: reservedToken, str: string(char)})
			i++
			continue
		}

		if val, err := numberPrefix(chars, &i, n); err == nil {
			tokens = append(tokens, token{kind: numberToken, val: decimal.NewFromFloat(val)})
			continue
		}

		return nil, &invalidTokenError{input: input, position: i}
	}
	tokens = append(tokens, token{kind: eosToken})
	return tokens, nil
}
