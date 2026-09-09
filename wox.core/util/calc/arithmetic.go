package calc

import (
	"fmt"
	"math/big"
)

// Arithmetic returns a new exact value, leaving operands reusable by both parsers.
func Arithmetic(op string, left, right *big.Rat) (*big.Rat, error) {
	switch op {
	case "+":
		return new(big.Rat).Add(left, right), nil
	case "-":
		return new(big.Rat).Sub(left, right), nil
	case "*":
		return new(big.Rat).Mul(left, right), nil
	case "/":
		if right.Sign() == 0 {
			return nil, fmt.Errorf("division by zero")
		}
		return new(big.Rat).Quo(left, right), nil
	default:
		return nil, fmt.Errorf("unsupported arithmetic operator %s", op)
	}
}
