package calc

import (
	"errors"
	"strings"
)

func isOperator(r rune) bool { return strings.ContainsRune("+-*/^(),;", r) }

// numberPrefix parses a number string with given separators.
func NumberPrefix(chars []rune, i *int, n int, thousandsSep, decimalSep string) (string, error) {
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
		return "", errors.New("expected a number")
	}

	*i = current
	return sb.String(), nil
}

// NormalizeNumberSeparators makes equivalent Unicode grouping spaces consistent.
func NormalizeNumberSeparators(input string) string {
	replacer := strings.NewReplacer(
		"\u00A0", " ", // no-break space
		"\u202F", " ", // narrow no-break space
		"\u2009", " ", // thin space
		"\u2007", " ", // figure space
	)
	return replacer.Replace(input)
}
