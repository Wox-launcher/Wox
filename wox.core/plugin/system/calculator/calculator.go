package calculator

import (
	"fmt"
	"math"
	"math/big"
	"strings"
	"wox/util/calc"

	"github.com/shopspring/decimal"
)

var functions = calc.Functions

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
		return calc.Arithmetic("+", left, right)
	case subNode:
		left, err := calculate(n.left)
		if err != nil {
			return nil, err
		}
		right, err := calculate(n.right)
		if err != nil {
			return nil, err
		}
		return calc.Arithmetic("-", left, right)
	case mulNode:
		left, err := calculate(n.left)
		if err != nil {
			return nil, err
		}
		right, err := calculate(n.right)
		if err != nil {
			return nil, err
		}
		return calc.Arithmetic("*", left, right)
	case divNode:
		left, err := calculate(n.left)
		if err != nil {
			return nil, err
		}
		right, err := calculate(n.right)
		if err != nil {
			return nil, err
		}
		return calc.Arithmetic("/", left, right)
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
		return calc.RatFromFloat(math.Pow(leftFloat, rightFloat))
	case numNode:
		return calc.RatFromDecimal(n.val)
	case funcNode:
		var args []*big.Rat
		for _, arg := range n.args {
			val, err := calculate(arg)
			if err != nil {
				return nil, err
			}
			args = append(args, val)
		}
		return calc.Call(n.funcName, args)
	}
	return nil, fmt.Errorf("unknown node type: %s", n.kind)
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
	return calc.DecimalFromRat(result)
}
