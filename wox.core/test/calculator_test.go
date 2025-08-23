package test

import (
	"testing"
)

func TestCalculatorBasic(t *testing.T) {
	suite := NewTestSuite(t)

	tests := []QueryTest{
		{
			Name:           "Simple addition",
			Query:          "1+2",
			ExpectedTitle:  "3",
			ExpectedAction: "Copy result",
		},
		{
			Name:           "Complex expression",
			Query:          "1+2*3",
			ExpectedTitle:  "7",
			ExpectedAction: "Copy result",
		},
		{
			Name:           "Parentheses",
			Query:          "(1+2)*3",
			ExpectedTitle:  "9",
			ExpectedAction: "Copy result",
		},
		{
			Name:           "Division",
			Query:          "10/2",
			ExpectedTitle:  "5",
			ExpectedAction: "Copy result",
		},
		{
			Name:           "Decimal result",
			Query:          "10/3",
			ExpectedTitle:  "3.3333333333333333",
			ExpectedAction: "Copy result",
		},
	}

	suite.RunQueryTests(tests)
}

func TestCalculatorTrigonometric(t *testing.T) {
	suite := NewTestSuite(t)

	tests := []QueryTest{
		{
			Name:           "Sin with addition",
			Query:          "sin(8) + 1",
			ExpectedTitle:  "1.9893582466233817",
			ExpectedAction: "Copy result",
		},
		{
			Name:           "Sin with pi",
			Query:          "sin(pi/4)",
			ExpectedTitle:  "0.7071067811865475",
			ExpectedAction: "Copy result",
		},
		{
			Name:           "Complex expression with pi",
			Query:          "2*pi + sin(pi/2)",
			ExpectedTitle:  "7.283185307179586",
			ExpectedAction: "Copy result",
		},
		{
			Name:           "Cosine",
			Query:          "cos(0)",
			ExpectedTitle:  "1",
			ExpectedAction: "Copy result",
		},
		{
			Name:           "Tangent",
			Query:          "tan(pi/4)",
			ExpectedTitle:  "0.9999999999999998",
			ExpectedAction: "Copy result",
		},
	}

	suite.RunQueryTests(tests)
}

func TestCalculatorAdvanced(t *testing.T) {
	suite := NewTestSuite(t)

	tests := []QueryTest{
		{
			Name:           "Exponential",
			Query:          "exp(2)",
			ExpectedTitle:  "7.38905609893065",
			ExpectedAction: "Copy result",
		},
		{
			Name:           "Logarithm",
			Query:          "log2(8)",
			ExpectedTitle:  "3",
			ExpectedAction: "Copy result",
		},
		{
			Name:           "Power",
			Query:          "pow(2,3)",
			ExpectedTitle:  "8",
			ExpectedAction: "Copy result",
		},
		{
			Name:           "Square root",
			Query:          "sqrt(16)",
			ExpectedTitle:  "4",
			ExpectedAction: "Copy result",
		},
		{
			Name:           "Absolute value",
			Query:          "abs(-42)",
			ExpectedTitle:  "42",
			ExpectedAction: "Copy result",
		},
		{
			Name:           "Rounding",
			Query:          "round(3.7)",
			ExpectedTitle:  "4",
			ExpectedAction: "Copy result",
		},
		{
			Name:           "Nested functions",
			Query:          "sqrt(pow(3,2) + pow(4,2))",
			ExpectedTitle:  "5",
			ExpectedAction: "Copy result",
		},
		{
			Name:           "Natural logarithm",
			Query:          "log(e)",
			ExpectedTitle:  "1",
			ExpectedAction: "Copy result",
		},
		{
			Name:           "Power operator basic",
			Query:          "2^3",
			ExpectedTitle:  "8",
			ExpectedAction: "Copy result",
		},
		{
			Name:           "Power operator with parentheses",
			Query:          "(2+1)^2",
			ExpectedTitle:  "9",
			ExpectedAction: "Copy result",
		},
		{
			Name:           "Power operator right associative",
			Query:          "2^3^2",
			ExpectedTitle:  "512",
			ExpectedAction: "Copy result",
		},
		{
			Name:           "Power operator with decimal",
			Query:          "4^0.5",
			ExpectedTitle:  "2",
			ExpectedAction: "Copy result",
		},
		{
			Name:           "Power operator negative base",
			Query:          "(-2)^3",
			ExpectedTitle:  "-8",
			ExpectedAction: "Copy result",
		},
		{
			Name:           "Power operator zero exponent",
			Query:          "5^0",
			ExpectedTitle:  "1",
			ExpectedAction: "Copy result",
		},
		{
			Name:           "Power operator complex expression",
			Query:          "2^3 + 3^2",
			ExpectedTitle:  "17",
			ExpectedAction: "Copy result",
		},
		{
			Name:           "Power operator with multiplication",
			Query:          "2 * 3^2",
			ExpectedTitle:  "18",
			ExpectedAction: "Copy result",
		},
		{
			Name:           "Power operator with division",
			Query:          "16 / 2^2",
			ExpectedTitle:  "4",
			ExpectedAction: "Copy result",
		},
		{
			Name:           "Power operator precedence",
			Query:          "2 + 3^2 * 4",
			ExpectedTitle:  "38",
			ExpectedAction: "Copy result",
		},
	}

	suite.RunQueryTests(tests)
}
