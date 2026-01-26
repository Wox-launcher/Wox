package calculator

import (
	"strings"
	"wox/util/locale"
)

type DecimalSeparator string

const (
	DecimalSeparatorSystem DecimalSeparator = "System Locale"
	DecimalSeparatorDot    DecimalSeparator = "Dot"
	DecimalSeparatorComma  DecimalSeparator = "Comma"
)

type ThousandsSeparator string

const (
	ThousandsSeparatorSystem     ThousandsSeparator = "System Locale"
	ThousandsSeparatorComma      ThousandsSeparator = "Comma"
	ThousandsSeparatorDot        ThousandsSeparator = "Dot"
	ThousandsSeparatorSpace      ThousandsSeparator = "Space"
	ThousandsSeparatorApostrophe ThousandsSeparator = "Apostrophe"
	ThousandsSeparatorNone       ThousandsSeparator = "None"
)

func isCommaDecimalLocale() bool {
	_, loc := locale.GetLocale()
	// Common regions that use comma as decimal separator
	// This is a simplified list. In a real world scenario we might need a more comprehensive database.
	// Reference: https://en.wikipedia.org/wiki/Decimal_separator#Countries_using_Arabic_numerals_with_decimal_comma
	commaRegions := map[string]bool{
		"al": true, "ar": true, "at": true, "be": true, "bg": true, "bo": true, "br": true, "by": true,
		"cm": true, "cl": true, "co": true, "cr": true, "cu": true, "cy": true, "cz": true, "de": true,
		"dk": true, "ec": true, "ee": true, "es": true, "fi": true, "fr": true, "gr": true, "hr": true,
		"hu": true, "id": true, "is": true, "it": true, "kz": true, "lt": true, "lu": true, "lv": true,
		"mk": true, "mn": true, "mz": true, "nl": true, "no": true, "pe": true, "pl": true, "pt": true,
		"py": true, "ro": true, "rs": true, "ru": true, "se": true, "si": true, "sk": true, "tr": true,
		"ua": true, "uy": true, "uz": true, "ve": true, "vn": true, "za": true,
	}

	return commaRegions[strings.ToLower(loc)]
}

func GetDecimalSeparator(mode DecimalSeparator) string {
	switch mode {
	case DecimalSeparatorDot:
		return "."
	case DecimalSeparatorComma:
		return ","
	case DecimalSeparatorSystem:
		if isCommaDecimalLocale() {
			return ","
		}
		return "."
	default:
		return "."
	}
}

func GetThousandsSeparator(mode ThousandsSeparator, decimalSep string) string {
	switch mode {
	case ThousandsSeparatorComma:
		return ","
	case ThousandsSeparatorDot:
		return "."
	case ThousandsSeparatorSpace:
		return " "
	case ThousandsSeparatorApostrophe:
		return "'"
	case ThousandsSeparatorNone:
		return ""
	case ThousandsSeparatorSystem:
		_, loc := locale.GetLocale()
		loc = strings.ToLower(loc)
		apostropheRegions := map[string]bool{
			"ch": true, "li": true,
		}
		spaceRegions := map[string]bool{
			"fr": true, "ru": true, "se": true, "no": true, "fi": true, "dk": true, "pl": true, "cz": true,
			"sk": true, "hu": true, "ua": true, "ro": true, "bg": true, "rs": true, "si": true, "hr": true,
		}
		if apostropheRegions[loc] {
			return "'"
		}
		if spaceRegions[loc] {
			return " "
		}
		if decimalSep == "," {
			return "."
		}
		return ","
	default:
		if decimalSep == "," {
			return "."
		}
		return ","
	}
}
