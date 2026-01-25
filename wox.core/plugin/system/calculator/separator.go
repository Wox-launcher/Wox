package calculator

import (
	"strings"
	"wox/util/locale"
)

type SeparatorMode string

const (
	SeparatorModeSystem SeparatorMode = "System Locale"
	SeparatorModeDot    SeparatorMode = "Dot"   // e.g., 1,234.567.89
	SeparatorModeComma  SeparatorMode = "Comma" // e.g., 1.234,567,89
)

// GetSeparators returns the thousands separator and decimal separator based on the mode
func GetSeparators(mode SeparatorMode) (thousandsSep string, decimalSep string) {
	switch mode {
	case SeparatorModeDot:
		return ",", "."
	case SeparatorModeComma:
		return ".", ","
	case SeparatorModeSystem:
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

		if commaRegions[strings.ToLower(loc)] {
			return ".", ","
		}
		// Default to US/UK style
		return ",", "."
	default:
		// Default to Comma style (US standard) if unknown
		return ",", "."
	}
}
