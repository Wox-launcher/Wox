package modules

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
	"wox/plugin"
	"wox/plugin/system/calculator/core"
	"wox/util"

	"github.com/PuerkitoBio/goquery"
	"github.com/shopspring/decimal"
)

type CurrencyModule struct {
	*regexBaseModule
	rates map[string]float64
}

func NewCurrencyModule(ctx context.Context, api plugin.API) *CurrencyModule {
	m := &CurrencyModule{
		rates: make(map[string]float64),
	}

	const (
		currencyPattern = `(?i)(usd|eur|gbp|jpy|cny|aud|cad)`
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

	// Add debug logging for pattern matching
	for _, h := range handlers {
		pattern := h.Pattern
		originalHandler := h.Handler
		h.Handler = func(ctx context.Context, matches []string) (*core.Result, error) {
			util.GetLogger().Debug(ctx, fmt.Sprintf("Currency pattern matched: %s, matches: %v", pattern, matches))
			return originalHandler(ctx, matches)
		}
	}

	m.regexBaseModule = NewRegexBaseModule(api, "currency", handlers)
	return m
}

func (m *CurrencyModule) StartExchangeRateSyncSchedule(ctx context.Context) {
	util.Go(ctx, "currency_exchange_rate_sync", func() {

		rates, err := m.parseExchangeRateFromHKAB(ctx)
		if err == nil {
			m.rates = rates
		} else {
			util.GetLogger().Error(ctx, fmt.Sprintf("Failed to fetch initial exchange rates from HKAB: %s", err.Error()))
		}

		for range time.NewTicker(1 * time.Hour).C {
			rates, err := m.parseExchangeRateFromHKAB(ctx)
			if err == nil {
				m.rates = rates
			} else {
				util.GetLogger().Error(ctx, fmt.Sprintf("Failed to fetch exchange rates from HKAB: %s", err.Error()))
			}
		}
	})
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
	return fmt.Sprintf("%s%s", symbol, amount.Round(2))
}

func (m *CurrencyModule) parseExchangeRateFromHKAB(ctx context.Context) (rates map[string]float64, err error) {
	util.GetLogger().Info(ctx, "Starting to parse exchange rates from HKAB")

	// Initialize maps
	rates = make(map[string]float64)
	rawRates := make(map[string]float64)

	body, err := util.HttpGet(ctx, "https://www.hkab.org.hk/en/rates/exchange-rates")
	if err != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("Failed to get exchange rates from HKAB: %s", err.Error()))
		return nil, err
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("Failed to parse HTML: %s", err.Error()))
		return nil, err
	}

	// Find the first general_table_container
	firstTable := doc.Find(".general_table_container").First()
	if firstTable.Length() == 0 {
		util.GetLogger().Error(ctx, "Failed to find exchange rate table")
		return nil, fmt.Errorf("exchange rate table not found")
	}

	// First pass: collect all raw rates from the first table only
	firstTable.Find(".general_table_row.exchange_rate").Each(func(i int, s *goquery.Selection) {
		// Get currency code
		currencyCode := strings.TrimSpace(s.Find(".exchange_rate_1 div:last-child").Text())
		if currencyCode == "" {
			return
		}

		// Get selling rate and buying rate
		var sellingRateStr, buyingRateStr string
		s.Find("div").Each(func(j int, sel *goquery.Selection) {
			text := strings.TrimSpace(sel.Text())
			if text == "Selling:" {
				sellingRateStr = strings.TrimSpace(sel.Parent().Find("div:last-child").Text())
			} else if text == "Buying TT:" {
				buyingRateStr = strings.TrimSpace(sel.Parent().Find("div:last-child").Text())
			}
		})

		if sellingRateStr == "" || buyingRateStr == "" {
			return
		}

		// Clean up rate strings and parse
		sellingRate, err := strconv.ParseFloat(strings.ReplaceAll(sellingRateStr, ",", ""), 64)
		if err != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("Failed to parse selling rate for %s: %v", currencyCode, err))
			return
		}

		buyingRate, err := strconv.ParseFloat(strings.ReplaceAll(buyingRateStr, ",", ""), 64)
		if err != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("Failed to parse buying rate for %s: %v", currencyCode, err))
			return
		}

		if sellingRate <= 0 || buyingRate <= 0 {
			return
		}

		// Calculate middle rate
		middleRate := (sellingRate + buyingRate) / 2
		rawRates[strings.ToUpper(currencyCode)] = middleRate
	})

	// Find USD rate first
	usdRate, exists := rawRates["USD"]
	if !exists {
		util.GetLogger().Error(ctx, "USD rate not found")
		return nil, fmt.Errorf("USD rate not found")
	}

	// Set base USD rate
	rates["USD"] = 1.0

	// Second pass: calculate all rates relative to USD
	for currency, rate := range rawRates {
		// Convert rates relative to USD
		usdToHkd := usdRate / 100.0
		currencyToHkd := rate / 100.0
		currencyPerUsd := usdToHkd / currencyToHkd
		rates[currency] = currencyPerUsd
	}

	if len(rates) < 2 {
		util.GetLogger().Error(ctx, "Failed to parse enough exchange rates")
		return nil, fmt.Errorf("failed to parse exchange rates")
	}

	util.GetLogger().Info(ctx, fmt.Sprintf("Successfully parsed %d exchange rates", len(rates)))
	return rates, nil
}
