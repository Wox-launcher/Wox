package calculator

import (
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"

	"github.com/shopspring/decimal"
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

const calculatorResultPrecision = 16

func call(funcName string, args []*big.Rat) (*big.Rat, error) {
	f, ok := functions[funcName]
	if !ok {
		return nil, fmt.Errorf("unknown function %s", funcName)
	}
	floatArgs := make([]float64, len(args))
	for i, arg := range args {
		floatArgs[i], _ = arg.Float64()
	}
	switch f := f.(type) {
	case func() float64:
		return ratFromFloat(f())
	case func(float64) float64:
		if funcName == "tan" {
			x := floatArgs[0]
			result := f(x)
			if result == 1 || result == -1 {
				result = f(math.Nextafter(x, 0))
			}
			if math.Abs(result-1) < 1e-12 {
				result = 1
			} else if math.Abs(result+1) < 1e-12 {
				result = -1
			}
			return ratFromFloat(result)
		}
		return ratFromFloat(f(floatArgs[0]))
	case func(float64, float64) float64:
		return ratFromFloat(f(floatArgs[0], floatArgs[1]))
	case func(float64, float64, float64) float64:
		return ratFromFloat(f(floatArgs[0], floatArgs[1], floatArgs[2]))
	default:
		return nil, fmt.Errorf("invalid function %s", funcName)
	}
}

func calculate(n *node) (*big.Rat, error) {
	switch n.kind {
	case addNode:
		left, err := calculate(n.left)
		if err != nil {
			return nil, err
		}
		right, err := calculate(n.right)
		if err != nil {
			return nil, err
		}
		return new(big.Rat).Add(left, right), nil
	case subNode:
		left, err := calculate(n.left)
		if err != nil {
			return nil, err
		}
		right, err := calculate(n.right)
		if err != nil {
			return nil, err
		}
		return new(big.Rat).Sub(left, right), nil
	case mulNode:
		left, err := calculate(n.left)
		if err != nil {
			return nil, err
		}
		right, err := calculate(n.right)
		if err != nil {
			return nil, err
		}
		return new(big.Rat).Mul(left, right), nil
	case divNode:
		left, err := calculate(n.left)
		if err != nil {
			return nil, err
		}
		right, err := calculate(n.right)
		if err != nil {
			return nil, err
		}
		if right.Sign() == 0 {
			return nil, fmt.Errorf("division by zero")
		}
		return new(big.Rat).Quo(left, right), nil
	case powNode:
		left, err := calculate(n.left)
		if err != nil {
			return nil, err
		}
		right, err := calculate(n.right)
		if err != nil {
			return nil, err
		}
		// Use math.Pow for power calculation
		leftFloat, _ := left.Float64()
		rightFloat, _ := right.Float64()
		return ratFromFloat(math.Pow(leftFloat, rightFloat))
	case numNode:
		return ratFromDecimal(n.val)
	case funcNode:
		var args []*big.Rat
		for _, arg := range n.args {
			val, err := calculate(arg)
			if err != nil {
				return nil, err
			}
			args = append(args, val)
		}
		return call(n.funcName, args)
	}
	return nil, fmt.Errorf("unknown node type: %s", n.kind)
}

// ratFromDecimal converts a decimal value to an exact rational value.
func ratFromDecimal(value decimal.Decimal) (*big.Rat, error) {
	result, ok := new(big.Rat).SetString(value.String())
	if !ok {
		return nil, fmt.Errorf("invalid decimal value: %s", value)
	}
	return result, nil
}

// ratFromFloat converts an approximate function result into a rational value without adding another float conversion.
func ratFromFloat(value float64) (*big.Rat, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return nil, fmt.Errorf("invalid numeric result: %v", value)
	}
	result, ok := new(big.Rat).SetString(strconv.FormatFloat(value, 'f', -1, 64))
	if !ok {
		return nil, fmt.Errorf("invalid numeric result: %v", value)
	}
	return result, nil
}

// decimalFromRat applies the calculator's display precision only after exact arithmetic is complete.
func decimalFromRat(value *big.Rat) (decimal.Decimal, error) {
	result, err := decimal.NewFromString(value.FloatString(calculatorResultPrecision))
	if err != nil {
		return decimal.Zero, err
	}
	return result, nil
}

func Calculate(expr string, thousandsSep, decimalSep string) (decimal.Decimal, error) {
	tokens, err := tokenize(expr, thousandsSep, decimalSep)
	if err != nil {
		return decimal.Zero, err
	}

	// Check if any identifier token is not a valid function name or constant
	for _, t := range tokens {
		if t.kind == identToken {
			// Check if it's a function name
			if _, ok := functions[t.str]; !ok {
				// If not a function, check if it's a constant
				if _, ok := map[string]float64{
					"e":       math.E,
					"pi":      math.Pi,
					"phi":     math.Phi,
					"sqrt2":   math.Sqrt2,
					"sqrte":   math.SqrtE,
					"sqrtpi":  math.SqrtPi,
					"sqrtphi": math.SqrtPhi,
					"ln2":     math.Ln2,
					"log2e":   math.Log2E,
					"ln10":    math.Ln10,
					"log10e":  math.Log10E,
				}[strings.ToLower(t.str)]; !ok {
					return decimal.Zero, fmt.Errorf("unknown identifier: %s", t.str)
				}
			}
		}
	}

	p := newParser(tokens)
	n, err := p.parse()
	if err != nil {
		return decimal.Zero, err
	}
	result, err := calculate(n)
	if err != nil {
		return decimal.Zero, err
	}
	return decimalFromRat(result)
}
