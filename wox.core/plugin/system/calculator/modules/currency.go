package modules

import (
	"context"
	"fmt"
	"strings"
	"wox/plugin"
	"wox/plugin/system/calculator/core"

	"github.com/shopspring/decimal"
)

type CurrencyModule struct {
	*regexBaseModule
	rates map[string]float64
}

func NewCurrencyModule(ctx context.Context, api plugin.API) *CurrencyModule {
	// Initialize with some common currencies
	// In real world application, these rates should be fetched from an API
	rates := map[string]float64{
		"USD": 1.0,
		"EUR": 0.853,
		"GBP": 0.79,
		"JPY": 142.35,
		"CNY": 7.14,
		"AUD": 1.47,
		"CAD": 1.32,
	}

	m := &CurrencyModule{
		rates: rates,
	}

	const (
		currencyPattern = `(usd|eur|gbp|jpy|cny|aud|cad)`
		numberPattern   = `([0-9]+(?:\.[0-9]+)?)`
	)

	// Initialize pattern handlers
	handlers := []*patternHandler{
		{
			Pattern:     numberPattern + `\s*` + currencyPattern + `\s+in\s+` + currencyPattern,
			Priority:    1000,
			Description: "Convert currency using 'in' format (e.g., 10 USD in EUR)",
			Handler:     m.handleConversion,
		},
		{
			Pattern:     numberPattern + `\s*` + currencyPattern + `\s*=\s*\?\s*` + currencyPattern,
			Priority:    900,
			Description: "Convert currency using '=?' format (e.g., 10USD=?EUR)",
			Handler:     m.handleConversion,
		},
		{
			Pattern:     numberPattern + `\s*` + currencyPattern + `\s+to\s+` + currencyPattern,
			Priority:    800,
			Description: "Convert currency using 'to' format (e.g., 10 USD to EUR)",
			Handler:     m.handleConversion,
		},
	}

	m.regexBaseModule = NewRegexBaseModule(api, "currency", handlers)
	return m
}

func (m *CurrencyModule) Convert(ctx context.Context, value *core.Result, toUnit string) (*core.Result, error) {
	fromCurrency := value.Unit
	toCurrency := strings.ToUpper(toUnit)

	// Check if currencies are supported
	if _, ok := m.rates[fromCurrency]; !ok {
		return nil, fmt.Errorf("unsupported currency: %s", fromCurrency)
	}
	if _, ok := m.rates[toCurrency]; !ok {
		return nil, fmt.Errorf("unsupported currency: %s", toCurrency)
	}

	// Convert to USD first (as base currency), then to target currency
	amountFloat, _ := value.RawValue.Float64()
	amountInUSD := amountFloat / m.rates[fromCurrency]
	result := amountInUSD * m.rates[toCurrency]
	resultDecimal := decimal.NewFromFloat(result)

	return &core.Result{
		DisplayValue: m.formatWithCurrencySymbol(resultDecimal, toCurrency),
		RawValue:     &resultDecimal,
		Unit:         toCurrency,
	}, nil
}

func (m *CurrencyModule) CanConvertTo(unit string) bool {
	_, ok := m.rates[strings.ToUpper(unit)]
	return ok
}

// Helper functions

func (m *CurrencyModule) handleConversion(ctx context.Context, matches []string) (*core.Result, error) {
	// matches[0] is the full match
	// matches[1] is the amount
	// matches[2] is the source currency
	// matches[3] is the target currency
	amount, err := decimal.NewFromString(matches[1])
	if err != nil {
		return nil, fmt.Errorf("invalid amount: %s", matches[1])
	}

	fromCurrency := strings.ToUpper(matches[2])
	toCurrency := strings.ToUpper(matches[3])

	// Check if currencies are supported
	if _, ok := m.rates[fromCurrency]; !ok {
		return nil, fmt.Errorf("unsupported currency: %s", fromCurrency)
	}
	if _, ok := m.rates[toCurrency]; !ok {
		return nil, fmt.Errorf("unsupported currency: %s", toCurrency)
	}

	// Convert to USD first (as base currency), then to target currency
	amountFloat, _ := amount.Float64()
	amountInUSD := amountFloat / m.rates[fromCurrency]
	result := amountInUSD * m.rates[toCurrency]
	resultDecimal := decimal.NewFromFloat(result)

	return &core.Result{
		DisplayValue: m.formatWithCurrencySymbol(resultDecimal, toCurrency),
		RawValue:     &resultDecimal,
		Unit:         toCurrency,
	}, nil
}

func (m *CurrencyModule) formatWithCurrencySymbol(amount decimal.Decimal, currency string) string {
	var symbol string
	switch currency {
	case "USD":
		symbol = "$"
	case "EUR":
		symbol = "€"
	case "GBP":
		symbol = "£"
	case "JPY":
		symbol = "¥"
	case "CNY":
		symbol = "¥"
	case "AUD":
		symbol = "A$"
	case "CAD":
		symbol = "C$"
	default:
		symbol = ""
	}

	// Format with exactly 2 decimal places
	return fmt.Sprintf("%s%s", symbol, amount)
}
