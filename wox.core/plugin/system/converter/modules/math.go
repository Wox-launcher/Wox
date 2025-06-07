package modules

import (
	"context"
	"fmt"
	"strings"
	"wox/plugin"
	"wox/plugin/system/converter/core"

	"github.com/shopspring/decimal"
)

type MathModule struct {
	*regexBaseModule
}

func NewMathModule(ctx context.Context, api plugin.API) *MathModule {
	m := &MathModule{}

	// Define patterns for math operations
	handlers := []*patternHandler{
		{
			Pattern:     `([0-9]+(?:\.[0-9]+)?)\s*%\s+of\s+\$([0-9]+(?:\.[0-9]+)?)`,
			Priority:    1300,
			Description: "Handle percentage of currency (e.g., 12% of $321)",
			Handler:     m.handlePercentageOfCurrency,
		},
		{
			Pattern:     `([0-9]+(?:\.[0-9]+)?)\s*%\s+of\s+([0-9]+(?:\.[0-9]+)?)\s*(?i)(usd|eur|gbp|jpy|cny|aud|cad|btc|eth|usdt|bnb)`,
			Priority:    1250,
			Description: "Handle percentage of currency/crypto with unit (e.g., 12% of 321 USD, 12% of 1 BTC)",
			Handler:     m.handlePercentageOfCurrencyOrCrypto,
		},
		{
			Pattern:     `([0-9]+(?:\.[0-9]+)?)\s*%`,
			Priority:    1200,
			Description: "Handle percentage (e.g., 12%)",
			Handler:     m.handlePercentage,
		},
		{
			Pattern:     `([0-9]+(?:\.[0-9]+)?)\s*%\s+of\s+([0-9]+(?:\.[0-9]+)?)`,
			Priority:    1100,
			Description: "Handle percentage of number (e.g., 12% of 321)",
			Handler:     m.handlePercentageOfNumber,
		},
	}

	m.regexBaseModule = NewRegexBaseModule(api, "math", handlers)
	return m
}

func (m *MathModule) Convert(ctx context.Context, value core.Result, toUnit core.Unit) (core.Result, error) {
	// Math module doesn't support unit conversion
	return value, fmt.Errorf("math module doesn't support unit conversion")
}

// Helper functions

func (m *MathModule) handlePercentage(ctx context.Context, matches []string) (core.Result, error) {
	percentage, err := decimal.NewFromString(matches[1])
	if err != nil {
		return core.Result{}, fmt.Errorf("invalid percentage: %s", matches[1])
	}

	// Convert percentage to decimal (12% = 0.12)
	result := percentage.Div(decimal.NewFromInt(100))

	return core.Result{
		DisplayValue: fmt.Sprintf("%s%%", percentage.String()),
		RawValue:     result,
		Unit:         core.Unit{Name: "percentage", Type: core.UnitTypeNumber},
		Module:       m,
	}, nil
}

func (m *MathModule) handlePercentageOfCurrencyOrCrypto(ctx context.Context, matches []string) (core.Result, error) {
	percentage, err := decimal.NewFromString(matches[1])
	if err != nil {
		return core.Result{}, fmt.Errorf("invalid percentage: %s", matches[1])
	}

	amount, err := decimal.NewFromString(matches[2])
	if err != nil {
		return core.Result{}, fmt.Errorf("invalid amount: %s", matches[2])
	}

	unit := strings.ToUpper(matches[3])

	// Calculate percentage of amount
	result := percentage.Div(decimal.NewFromInt(100)).Mul(amount)

	// Determine if it's crypto or currency
	cryptoUnits := map[string]bool{"BTC": true, "ETH": true, "USDT": true, "BNB": true}
	var unitType core.UnitType
	var displayValue string

	if cryptoUnits[unit] {
		unitType = core.UnitTypeCrypto
		displayValue = fmt.Sprintf("%s %s", result.Round(8).String(), unit)
	} else {
		unitType = core.UnitTypeCurrency
		displayValue = fmt.Sprintf("%s %s", result.Round(2).String(), unit)
	}

	return core.Result{
		DisplayValue: displayValue,
		RawValue:     result,
		Unit:         core.Unit{Name: unit, Type: unitType},
		Module:       m,
	}, nil
}

func (m *MathModule) handlePercentageOfCurrency(ctx context.Context, matches []string) (core.Result, error) {
	percentage, err := decimal.NewFromString(matches[1])
	if err != nil {
		return core.Result{}, fmt.Errorf("invalid percentage: %s", matches[1])
	}

	amount, err := decimal.NewFromString(matches[2])
	if err != nil {
		return core.Result{}, fmt.Errorf("invalid amount: %s", matches[2])
	}

	// Calculate percentage of amount
	result := percentage.Div(decimal.NewFromInt(100)).Mul(amount)

	return core.Result{
		DisplayValue: fmt.Sprintf("$%s", result.Round(2).String()),
		RawValue:     result,
		Unit:         core.Unit{Name: "USD", Type: core.UnitTypeCurrency},
		Module:       m,
	}, nil
}

func (m *MathModule) handlePercentageOfNumber(ctx context.Context, matches []string) (core.Result, error) {
	percentage, err := decimal.NewFromString(matches[1])
	if err != nil {
		return core.Result{}, fmt.Errorf("invalid percentage: %s", matches[1])
	}

	amount, err := decimal.NewFromString(matches[2])
	if err != nil {
		return core.Result{}, fmt.Errorf("invalid amount: %s", matches[2])
	}

	// Calculate percentage of amount
	result := percentage.Div(decimal.NewFromInt(100)).Mul(amount)

	return core.Result{
		DisplayValue: result.Round(2).String(),
		RawValue:     result,
		Unit:         core.Unit{Name: "number", Type: core.UnitTypeNumber},
		Module:       m,
	}, nil
}

func (m *MathModule) TokenPatterns() []core.TokenPattern {
	return []core.TokenPattern{
		{
			Pattern:   `([0-9]+(?:\.[0-9]+)?)\s*%\s+of\s+\$([0-9]+(?:\.[0-9]+)?)`,
			Type:      core.IdentToken,
			Priority:  1300,
			FullMatch: false,
			Module:    m,
		},
		{
			Pattern:   `([0-9]+(?:\.[0-9]+)?)\s*%\s+of\s+([0-9]+(?:\.[0-9]+)?)\s*(?i)(usd|eur|gbp|jpy|cny|aud|cad|btc|eth|usdt|bnb)`,
			Type:      core.IdentToken,
			Priority:  1250,
			FullMatch: false,
			Module:    m,
		},
		{
			Pattern:   `([0-9]+(?:\.[0-9]+)?)\s*%`,
			Type:      core.IdentToken,
			Priority:  1200,
			FullMatch: false,
			Module:    m,
		},
		{
			Pattern:   `([0-9]+(?:\.[0-9]+)?)\s*%\s+of\s+([0-9]+(?:\.[0-9]+)?)`,
			Type:      core.IdentToken,
			Priority:  1100,
			FullMatch: false,
			Module:    m,
		},
	}
}
