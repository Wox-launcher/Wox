package calculator

import (
	"strings"
	"unicode"
	"wox/util/calc"

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

const operators = "+-*/^(),;"

func isOperator(char rune) bool {
	for _, op := range operators {
		if char == op {
			return true
		}
	}
	return false
}

func isAlpha(char rune) bool {
	return (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z')
}

func isAlNum(char rune) bool {
	return isAlpha(char) || (char >= '0' && char <= '9')
}

func tokenize(input string, thousandsSep, decimalSep string) ([]token, error) {
	input = calc.NormalizeNumberSeparators(input)
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

		if number, err := calc.NumberPrefix(chars, &i, n, thousandsSep, decimalSep); err == nil {
			val, parseErr := decimal.NewFromString(number)
			if parseErr != nil {
				return nil, parseErr
			}
			tokens = append(tokens, token{kind: numberToken, val: val})
			continue
		}

		if isOperator(char) {
			tokens = append(tokens, token{kind: reservedToken, str: string(char)})
			i++
			continue
		}

		return nil, &invalidTokenError{input: input, position: i}
	}
	tokens = append(tokens, token{kind: eosToken})
	return tokens, nil
}
