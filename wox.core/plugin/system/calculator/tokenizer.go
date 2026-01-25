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

const operators = "+-*/^(),;"

func isOperator(char rune) bool {
	for _, op := range operators {
		if char == op {
			return true
		}
	}
	return false
}

// numberPrefix parses a number string with given separators
func numberPrefix(chars []rune, i *int, n int, thousandsSep, decimalSep string) (float64, error) {
	current := *i
	seenDecimal := false
	var sb strings.Builder

	for current < n {
		char := chars[current]

		// 1. Check for Digit
		if char >= '0' && char <= '9' {
			sb.WriteRune(char)
			current++
			continue
		}

		// 2. Check for Decimal Separator
		if !seenDecimal && decimalSep != "" && strings.HasPrefix(string(chars[current:]), decimalSep) {
			// If decimalSep is also an operator (like ','), only consume it if valid as decimal part
			// Heuristic: require following digit if it's an ambiguous separator
			shouldConsume := true
			isAmbiguous := isOperator(rune(decimalSep[0]))

			if isAmbiguous {
				sepLen := len(decimalSep)
				if current+sepLen < n {
					nextChar := chars[current+sepLen]
					// Check if next char is digit
					if nextChar < '0' || nextChar > '9' {
						shouldConsume = false
					}
				} else {
					// EOF after separator? "1,". Treat as valid.
				}
			}

			if shouldConsume {
				current += len(decimalSep)
				sb.WriteRune('.')
				seenDecimal = true
				continue
			}
		}

		// 3. Check for Thousands Separator
		// Only allowed before decimal point
		if !seenDecimal && thousandsSep != "" && strings.HasPrefix(string(chars[current:]), thousandsSep) {
			sepLen := len(thousandsSep)
			// Must be followed by 3 digits
			if current+sepLen+3 <= n {
				validGroup := true
				for k := 0; k < 3; k++ {
					if chars[current+sepLen+k] < '0' || chars[current+sepLen+k] > '9' {
						validGroup = false
						break
					}
				}
				if validGroup {
					current += sepLen // consume separator
					// Don't consume it (skip it)
					continue
				}
			}
		}

		// Nothing matched, break
		break
	}

	if sb.Len() == 0 || (sb.Len() == 1 && sb.String() == ".") {
		return 0, errors.New("expected a number")
	}

	val, err := strconv.ParseFloat(sb.String(), 64)
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

func tokenize(input string, thousandsSep, decimalSep string) ([]token, error) {
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

		if val, err := numberPrefix(chars, &i, n, thousandsSep, decimalSep); err == nil {
			tokens = append(tokens, token{kind: numberToken, val: decimal.NewFromFloat(val)})
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
