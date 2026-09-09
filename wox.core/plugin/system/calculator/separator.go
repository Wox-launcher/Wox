package calculator

import "wox/util/calc"

type DecimalSeparator = calc.DecimalSeparator
type ThousandsSeparator = calc.ThousandsSeparator

const (
	DecimalSeparatorSystem       = calc.DecimalSeparatorSystem
	DecimalSeparatorDot          = calc.DecimalSeparatorDot
	DecimalSeparatorComma        = calc.DecimalSeparatorComma
	ThousandsSeparatorSystem     = calc.ThousandsSeparatorSystem
	ThousandsSeparatorComma      = calc.ThousandsSeparatorComma
	ThousandsSeparatorDot        = calc.ThousandsSeparatorDot
	ThousandsSeparatorSpace      = calc.ThousandsSeparatorSpace
	ThousandsSeparatorApostrophe = calc.ThousandsSeparatorApostrophe
	ThousandsSeparatorNone       = calc.ThousandsSeparatorNone
)

var GetDecimalSeparator = calc.GetDecimalSeparator
var GetThousandsSeparator = calc.GetThousandsSeparator
