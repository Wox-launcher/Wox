package test

import (
	"testing"
	"wox/plugin"
	"wox/util"
)

const calculatorPluginID = "bd723c38-f28d-4152-8621-76fd21d6456e"

func setCalculatorSeparators(t *testing.T, decimalMode string, thousandsMode string) {
	t.Helper()
	var calcInstance *plugin.Instance
	for _, instance := range plugin.GetPluginManager().GetPluginInstances() {
		if instance.Metadata.Id == calculatorPluginID {
			calcInstance = instance
			break
		}
	}
	if calcInstance == nil {
		t.Fatal("Calculator plugin instance not found")
	}
	if err := calcInstance.Setting.Set("DecimalSeparator", decimalMode); err != nil {
		t.Fatalf("Failed to set decimal separator mode: %v", err)
	}
	if err := calcInstance.Setting.Set("ThousandsSeparator", thousandsMode); err != nil {
		t.Fatalf("Failed to set thousands separator mode: %v", err)
	}
}

func TestCalculatorBasic(t *testing.T) {
	suite := NewTestSuite(t)
	setCalculatorSeparators(t, "Dot", "Comma")

	tests := []QueryTest{
		{
			Name:           "Simple addition",
			Query:          "1+2",
			ExpectedTitle:  "3",
			ExpectedAction: "Copy",
		},
		{
			Name:           "Complex expression",
			Query:          "1+2*3",
			ExpectedTitle:  "7",
			ExpectedAction: "Copy",
		},
		{
			Name:           "Parentheses",
			Query:          "(1+2)*3",
			ExpectedTitle:  "9",
			ExpectedAction: "Copy",
		},
		{
			Name:           "Division",
			Query:          "10/2",
			ExpectedTitle:  "5",
			ExpectedAction: "Copy",
		},
		{
			Name:           "Decimal result",
			Query:          "10/3",
			ExpectedTitle:  "3.3333333333333333",
			ExpectedAction: "Copy",
		},
		{
			Name:           "Leading dot decimal addition",
			Query:          "1 + .5",
			ExpectedTitle:  "1.5",
			ExpectedAction: "Copy",
		},
		{
			Name:           "Leading dot decimal multiplication",
			Query:          "1 * .1",
			ExpectedTitle:  "0.1",
			ExpectedAction: "Copy",
		},
		{
			Name:           "Leading dot both sides addition",
			Query:          ".1 + .2",
			ExpectedTitle:  "0.3",
			ExpectedAction: "Copy",
		},
		{
			Name:           "Leading dot multiplication",
			Query:          ".5 * .5",
			ExpectedTitle:  "0.25",
			ExpectedAction: "Copy",
		},
		{
			Name:           "Leading dot with unary minus",
			Query:          "-.5 + 1",
			ExpectedTitle:  "0.5",
			ExpectedAction: "Copy",
		},
		{
			Name:           "Leading dot inside parentheses",
			Query:          "(.5 + .25) * 2",
			ExpectedTitle:  "1.5",
			ExpectedAction: "Copy",
		},
		{
			Name:           "Leading dot in divisor",
			Query:          "1 / .5",
			ExpectedTitle:  "2",
			ExpectedAction: "Copy",
		},
	}

	suite.RunQueryTests(tests)
}

func TestCalculatorTrigonometric(t *testing.T) {
	suite := NewTestSuite(t)
	setCalculatorSeparators(t, "Dot", "Comma")

	tests := []QueryTest{
		{
			Name:           "Sin with addition",
			Query:          "sin(8) + 1",
			ExpectedTitle:  "1.9893582466233817",
			ExpectedAction: "Copy",
		},
		{
			Name:           "Sin with pi",
			Query:          "sin(pi/4)",
			ExpectedTitle:  "0.7071067811865475",
			ExpectedAction: "Copy",
		},
		{
			Name:           "Complex expression with pi",
			Query:          "2*pi + sin(pi/2)",
			ExpectedTitle:  "7.283185307179586",
			ExpectedAction: "Copy",
		},
		{
			Name:           "Cosine",
			Query:          "cos(0)",
			ExpectedTitle:  "1",
			ExpectedAction: "Copy",
		},
		{
			Name:           "Tangent",
			Query:          "tan(pi/4)",
			ExpectedTitle:  "1",
			ExpectedAction: "Copy",
		},
	}

	suite.RunQueryTests(tests)
}

// calculator calculates should been done in < 10ms
func TestCalculatorShouldHandleQuick(t *testing.T) {
	suite := NewTestSuite(t)
	setCalculatorSeparators(t, "Dot", "Comma")

	tests := []QueryTest{
		{
			Name:           "Quick calculation 1",
			Query:          "123456789 + 987654321",
			ExpectedTitle:  "1,111,111,110",
			ExpectedAction: "Copy",
		},
	}

	suite.RunQueryTestsWithMaxDuration(tests, 10)
}

func TestCalculatorAdvanced(t *testing.T) {
	suite := NewTestSuite(t)
	setCalculatorSeparators(t, "Dot", "Comma")

	tests := []QueryTest{
		{
			Name:           "Exponential",
			Query:          "exp(2)",
			ExpectedTitle:  "7.38905609893065",
			ExpectedAction: "Copy",
		},
		{
			Name:           "Logarithm",
			Query:          "log2(8)",
			ExpectedTitle:  "3",
			ExpectedAction: "Copy",
		},
		{
			Name:           "Power",
			Query:          "pow(2,3)",
			ExpectedTitle:  "8",
			ExpectedAction: "Copy",
		},
		{
			Name:           "Square root",
			Query:          "sqrt(16)",
			ExpectedTitle:  "4",
			ExpectedAction: "Copy",
		},
		{
			Name:           "Absolute value",
			Query:          "abs(-42)",
			ExpectedTitle:  "42",
			ExpectedAction: "Copy",
		},
		{
			Name:           "Rounding",
			Query:          "round(3.7)",
			ExpectedTitle:  "4",
			ExpectedAction: "Copy",
		},
		{
			Name:           "Nested functions",
			Query:          "sqrt(pow(3,2) + pow(4,2))",
			ExpectedTitle:  "5",
			ExpectedAction: "Copy",
		},
		{
			Name:           "Natural logarithm",
			Query:          "log(e)",
			ExpectedTitle:  "1",
			ExpectedAction: "Copy",
		},
		{
			Name:           "Power operator basic",
			Query:          "2^3",
			ExpectedTitle:  "8",
			ExpectedAction: "Copy",
		},
		{
			Name:           "Power operator with parentheses",
			Query:          "(2+1)^2",
			ExpectedTitle:  "9",
			ExpectedAction: "Copy",
		},
		{
			Name:           "Power operator right associative",
			Query:          "2^3^2",
			ExpectedTitle:  "512",
			ExpectedAction: "Copy",
		},
		{
			Name:           "Power operator with decimal",
			Query:          "4^0.5",
			ExpectedTitle:  "2",
			ExpectedAction: "Copy",
		},
		{
			Name:           "Power operator negative base",
			Query:          "(-2)^3",
			ExpectedTitle:  "-8",
			ExpectedAction: "Copy",
		},
		{
			Name:           "Power operator zero exponent",
			Query:          "5^0",
			ExpectedTitle:  "1",
			ExpectedAction: "Copy",
		},
		{
			Name:           "Power operator complex expression",
			Query:          "2^3 + 3^2",
			ExpectedTitle:  "17",
			ExpectedAction: "Copy",
		},
		{
			Name:           "Power operator with multiplication",
			Query:          "2 * 3^2",
			ExpectedTitle:  "18",
			ExpectedAction: "Copy",
		},
		{
			Name:           "Power operator with division",
			Query:          "16 / 2^2",
			ExpectedTitle:  "4",
			ExpectedAction: "Copy",
		},
		{
			Name:           "Power operator precedence",
			Query:          "2 + 3^2 * 4",
			ExpectedTitle:  "38",
			ExpectedAction: "Copy",
		},
	}

	suite.RunQueryTests(tests)
}

func TestCalculatorSeparators(t *testing.T) {
	suite := NewTestSuite(t)
	setCalculatorSeparators(t, "Dot", "Comma")
	calculatorId := "bd723c38-f28d-4152-8621-76fd21d6456e"

	// Find the plugin instance to ensure we update the correct setting store
	var calcInstance *plugin.Instance
	for _, instance := range plugin.GetPluginManager().GetPluginInstances() {
		if instance.Metadata.Id == calculatorId {
			calcInstance = instance
			break
		}
	}
	if calcInstance == nil {
		t.Fatal("Calculator plugin instance not found")
	}

	// Helper to set separator modes
	setModes := func(decimalMode string, thousandsMode string) {
		// Clear platform-specific overrides so tests use the values we set here.
		platformSuffix := "@" + util.GetCurrentPlatform()
		_ = calcInstance.Setting.Delete("DecimalSeparator" + platformSuffix)
		_ = calcInstance.Setting.Delete("ThousandsSeparator" + platformSuffix)
		if err := calcInstance.Setting.Set("DecimalSeparator", decimalMode); err != nil {
			t.Fatalf("Failed to set decimal separator mode: %v", err)
		}
		if err := calcInstance.Setting.Set("ThousandsSeparator", thousandsMode); err != nil {
			t.Fatalf("Failed to set thousands separator mode: %v", err)
		}
	}

	// Test Dot Mode (US: 1,234.56)
	// Thousands: ,
	// Decimal: .
	t.Run("Dot Mode (US)", func(t *testing.T) {
		setModes("Dot", "Comma")
		tests := []QueryTest{
			{
				Name:           "Dot Mode - Simple Addition",
				Query:          "1.5 + 2.5",
				ExpectedTitle:  "4",
				ExpectedAction: "Copy",
			},
			{
				Name:           "Dot Mode - Thousands Separator",
				Query:          "1,000 + 200", // 1000 + 200
				ExpectedTitle:  "1,200",
				ExpectedAction: "Copy",
			},
			{
				Name:           "Dot Mode - Output Format",
				Query:          "1234.56 * 1",
				ExpectedTitle:  "1,234.56",
				ExpectedAction: "Copy",
			},
			{
				Name:           "Dot Mode - Argument Separator",
				Query:          "max(1, 2)", // Comma as argument separator (since decimal is Dot)
				ExpectedTitle:  "2",
				ExpectedAction: "Copy",
			},
		}
		suite.RunQueryTests(tests)
	})

	// Test Comma Mode (European: 1.234,56)
	// Thousands: .
	// Decimal: ,
	t.Run("Comma Mode (EU)", func(t *testing.T) {
		setModes("Comma", "Dot")
		tests := []QueryTest{
			{
				Name:           "Comma Mode - Simple Addition",
				Query:          "1,5 + 2,5", // 1.5 + 2.5
				ExpectedTitle:  "4",
				ExpectedAction: "Copy",
			},
			{
				Name:           "Comma Mode - Thousands Separator",
				Query:          "1.000 + 200", // 1000 + 200
				ExpectedTitle:  "1.200",
				ExpectedAction: "Copy",
			},
			{
				Name:           "Comma Mode - Output Format",
				Query:          "1234,56 * 1",
				ExpectedTitle:  "1.234,56",
				ExpectedAction: "Copy",
			},
			{
				Name:           "Comma Mode - Argument Separator",
				Query:          "max(1; 2)", // Semicolon as separator (since comma is decimal)
				ExpectedTitle:  "2",
				ExpectedAction: "Copy",
			},
		}
		suite.RunQueryTests(tests)
	})

	// Test Space thousands (SI: 1 234,56)
	// Thousands: space
	// Decimal: ,
	t.Run("Space Thousands (SI)", func(t *testing.T) {
		setModes("Comma", "Space")
		tests := []QueryTest{
			{
				Name:           "Space Thousands - Simple Addition",
				Query:          "1 000 + 200",
				ExpectedTitle:  "1 200",
				ExpectedAction: "Copy",
			},
			{
				Name:           "Space Thousands - Output Format",
				Query:          "1234,56 * 1",
				ExpectedTitle:  "1 234,56",
				ExpectedAction: "Copy",
			},
		}
		suite.RunQueryTests(tests)
	})

	// Test Apostrophe thousands (Swiss: 1'234.56)
	// Thousands: '
	// Decimal: .
	t.Run("Apostrophe Thousands (CH)", func(t *testing.T) {
		setModes("Dot", "Apostrophe")
		tests := []QueryTest{
			{
				Name:           "Apostrophe Thousands - Simple Addition",
				Query:          "1'000 + 200",
				ExpectedTitle:  "1'200",
				ExpectedAction: "Copy",
			},
			{
				Name:           "Apostrophe Thousands - Output Format",
				Query:          "1234.56 * 1",
				ExpectedTitle:  "1'234.56",
				ExpectedAction: "Copy",
			},
		}
		suite.RunQueryTests(tests)
	})
}
