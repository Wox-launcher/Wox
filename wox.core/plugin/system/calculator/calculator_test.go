package calculator

import "testing"

// TestCalculateExactRationalArithmetic verifies that intermediate division is not rounded.
func TestCalculateExactRationalArithmetic(t *testing.T) {
	tests := []struct {
		expression string
		expected   string
	}{
		{expression: "2/3*6", expected: "4"},
		{expression: "1/3*3", expected: "1"},
		{expression: "1/3", expected: "0.3333333333333333"},
	}

	for _, test := range tests {
		result, err := Calculate(test.expression, ",", ".")
		if err != nil {
			t.Fatalf("Calculate(%q) returned an error: %v", test.expression, err)
		}
		if result.String() != test.expected {
			t.Errorf("Calculate(%q) = %s, expected %s", test.expression, result, test.expected)
		}
	}
}
