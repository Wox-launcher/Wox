package calculator

import (
	"fmt"
	"github.com/shopspring/decimal"
	"math"
)

var functions = map[string]interface{}{
	"abs":         math.Abs,
	"acos":        math.Acos,
	"acosh":       math.Acosh,
	"asin":        math.Asin,
	"asinh":       math.Asinh,
	"atan":        math.Atan,
	"atan2":       math.Atan2,
	"atanh":       math.Atanh,
	"cbrt":        math.Cbrt,
	"ceil":        math.Ceil,
	"copysign":    math.Copysign,
	"cos":         math.Cos,
	"cosh":        math.Cosh,
	"dim":         math.Dim,
	"erf":         math.Erf,
	"erfc":        math.Erfc,
	"erfcinv":     math.Erfcinv, // Go 1.10+
	"erfinv":      math.Erfinv,  // Go 1.10+
	"exp":         math.Exp,
	"exp2":        math.Exp2,
	"expm1":       math.Expm1,
	"fma":         math.FMA, // Go 1.14+
	"floor":       math.Floor,
	"gamma":       math.Gamma,
	"hypot":       math.Hypot,
	"j0":          math.J0,
	"j1":          math.J1,
	"log":         math.Log,
	"log10":       math.Log10,
	"log1p":       math.Log1p,
	"log2":        math.Log2,
	"logb":        math.Logb,
	"max":         math.Max,
	"min":         math.Min,
	"mod":         math.Mod,
	"nan":         math.NaN,
	"nextafter":   math.Nextafter,
	"pow":         math.Pow,
	"remainder":   math.Remainder,
	"round":       math.Round,       // Go 1.10+
	"roundtoeven": math.RoundToEven, // Go 1.10+
	"sin":         math.Sin,
	"sinh":        math.Sinh,
	"sqrt":        math.Sqrt,
	"tan":         math.Tan,
	"tanh":        math.Tanh,
	"trunc":       math.Trunc,
	"y0":          math.Y0,
	"y1":          math.Y1,
}

func call(funcName string, args []decimal.Decimal) (decimal.Decimal, error) {
	f, ok := functions[funcName]
	if !ok {
		return decimal.Zero, fmt.Errorf("unknown function %s", funcName)
	}
	switch f := f.(type) {
	case func() float64:
		return decimal.NewFromFloat(f()), nil
	case func(float64) float64:
		return decimal.NewFromFloat(f(args[0].InexactFloat64())), nil
	case func(float64, float64) float64:
		return decimal.NewFromFloat(f(args[0].InexactFloat64(), args[1].InexactFloat64())), nil
	case func(float64, float64, float64) float64:
		return decimal.NewFromFloat(f(args[0].InexactFloat64(), args[1].InexactFloat64(), args[2].InexactFloat64())), nil
	default:
		return decimal.Zero, fmt.Errorf("invalid function %s", funcName)
	}
}

func calculate(n *node) (decimal.Decimal, error) {
	switch n.kind {
	case addNode:
		left, err := calculate(n.left)
		if err != nil {
			return decimal.Zero, err
		}
		right, err := calculate(n.right)
		if err != nil {
			return decimal.Zero, err
		}
		return left.Add(right), nil
	case subNode:
		left, err := calculate(n.left)
		if err != nil {
			return decimal.Zero, err
		}
		right, err := calculate(n.right)
		if err != nil {
			return decimal.Zero, err
		}
		return left.Sub(right), nil
	case mulNode:
		left, err := calculate(n.left)
		if err != nil {
			return decimal.Zero, err
		}
		right, err := calculate(n.right)
		if err != nil {
			return decimal.Zero, err
		}
		return left.Mul(right), nil
	case divNode:
		left, err := calculate(n.left)
		if err != nil {
			return decimal.Zero, err
		}
		right, err := calculate(n.right)
		if err != nil {
			return decimal.Zero, err
		}
		return left.Div(right), nil
	case numNode:
		return n.val, nil
	case funcNode:
		var args []decimal.Decimal
		for _, arg := range n.args {
			val, err := calculate(arg)
			if err != nil {
				return decimal.Zero, err
			}
			args = append(args, val)
		}
		return call(n.funcName, args)
	}
	return decimal.Zero, fmt.Errorf("unknown node type: %s", n.kind)
}

func Calculate(expr string) (decimal.Decimal, error) {
	tokens, err := tokenize(expr)
	if err != nil {
		return decimal.Zero, err
	}
	p := newParser(tokens)
	n, err := p.parse()
	if err != nil {
		return decimal.Zero, err
	}
	return calculate(n)
}
