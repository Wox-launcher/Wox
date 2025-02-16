package modules

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
	"time"
	"wox/plugin"
	"wox/plugin/system/converter/core"
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

	// Initialize pattern handlers with atomic patterns
	handlers := []*patternHandler{
		{
			Pattern:     numberPattern + `\s*` + currencyPattern,
			Priority:    1000,
			Description: "Handle currency amount (e.g., 10 USD)",
			Handler:     m.handleSingleCurrency,
		},
		{
			Pattern:     `in\s+` + currencyPattern,
			Priority:    900,
			Description: "Handle 'in' conversion format (e.g., in EUR)",
			Handler:     m.handleInConversion,
		},
		{
			Pattern:     `to\s+` + currencyPattern,
			Priority:    800,
			Description: "Handle 'to' conversion format (e.g., to EUR)",
			Handler:     m.handleToConversion,
		},
		{
			Pattern:     `=\s*\?\s*` + currencyPattern,
			Priority:    700,
			Description: "Handle '=?' conversion format (e.g., =?EUR)",
			Handler:     m.handleToConversion,
		},
	}

	m.regexBaseModule = NewRegexBaseModule(api, "currency", handlers)
	return m
}

func (m *CurrencyModule) StartExchangeRateSyncSchedule(ctx context.Context) {
	util.Go(ctx, "currency_exchange_rate_sync", func() {
		// Try multiple data sources
		sources := []func(context.Context) (map[string]float64, error){
			m.parseExchangeRateFromHKAB,
			m.parseExchangeRateFromECB,
		}

		for _, source := range sources {
			rates, err := source(ctx)
			if err == nil && len(rates) > 0 {
				m.rates = rates
				util.GetLogger().Info(ctx, "Successfully updated rates from source")
				break
			}
			util.GetLogger().Warn(ctx, fmt.Sprintf("Failed to update rates from source: %s", err.Error()))
		}

		for range time.NewTicker(1 * time.Hour).C {
			for _, source := range sources {
				rates, err := source(ctx)
				if err == nil && len(rates) > 0 {
					m.rates = rates
					util.GetLogger().Info(ctx, "Successfully updated rates from source")
					break
				}
				util.GetLogger().Warn(ctx, fmt.Sprintf("Failed to update rates from source: %s", err.Error()))
			}
		}
	})
}

func (m *CurrencyModule) Convert(ctx context.Context, value core.Result, toUnit core.Unit) (core.Result, error) {
	fromCurrency := value.Unit.Name
	toCurrency := toUnit.Name

	// Check if currencies are supported
	if _, ok := m.rates[fromCurrency]; !ok {
		return core.Result{}, fmt.Errorf("unsupported currency: %s", fromCurrency)
	}
	if _, ok := m.rates[toCurrency]; !ok {
		return core.Result{}, fmt.Errorf("unsupported currency: %s", toCurrency)
	}

	// Convert to USD first (as base currency), then to target currency
	amountFloat, _ := value.RawValue.Float64()
	amountInUSD := amountFloat / m.rates[fromCurrency]
	result := amountInUSD * m.rates[toCurrency]
	resultDecimal := decimal.NewFromFloat(result)

	return core.Result{
		DisplayValue: m.formatWithCurrencySymbol(resultDecimal, toCurrency),
		RawValue:     resultDecimal,
		Unit:         toUnit,
		Module:       m,
	}, nil
}

func (m *CurrencyModule) CanConvertTo(unit string) bool {
	_, ok := m.rates[strings.ToUpper(unit)]
	return ok
}

// Helper functions

func (m *CurrencyModule) handleSingleCurrency(ctx context.Context, matches []string) (core.Result, error) {
	amount, err := decimal.NewFromString(matches[1])
	if err != nil {
		return core.Result{}, fmt.Errorf("invalid amount: %s", matches[1])
	}

	currency := strings.ToUpper(matches[2])

	// Check if the currency is supported
	if _, ok := m.rates[currency]; !ok {
		return core.Result{}, fmt.Errorf("unsupported currency: %s", currency)
	}

	return core.Result{
		DisplayValue: m.formatWithCurrencySymbol(amount, currency),
		RawValue:     amount,
		Unit:         core.Unit{Name: currency, Type: core.UnitTypeCurrency},
		Module:       m,
	}, nil
}

func (m *CurrencyModule) handleInConversion(ctx context.Context, matches []string) (core.Result, error) {
	currency := strings.ToUpper(matches[1])
	if _, ok := m.rates[currency]; !ok {
		return core.Result{}, fmt.Errorf("unsupported currency: %s", currency)
	}
	return core.Result{
		DisplayValue: fmt.Sprintf("in %s", currency),
		RawValue:     decimal.NewFromInt(0),
		Unit:         core.Unit{Name: currency, Type: core.UnitTypeCurrency},
		Module:       m,
	}, nil
}

func (m *CurrencyModule) handleToConversion(ctx context.Context, matches []string) (core.Result, error) {
	currency := strings.ToUpper(matches[1])
	if _, ok := m.rates[currency]; !ok {
		return core.Result{}, fmt.Errorf("unsupported currency: %s", currency)
	}
	return core.Result{
		DisplayValue: fmt.Sprintf("to %s", currency),
		RawValue:     decimal.NewFromInt(0),
		Unit:         core.Unit{Name: currency, Type: core.UnitTypeCurrency},
		Module:       m,
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

// parseExchangeRateFromECB parses exchange rates from European Central Bank
func (m *CurrencyModule) parseExchangeRateFromECB(ctx context.Context) (rates map[string]float64, err error) {
	util.GetLogger().Info(ctx, "Starting to parse exchange rates from ECB")

	// Initialize maps
	rates = make(map[string]float64)

	// ECB provides daily reference rates in XML format
	body, err := util.HttpGet(ctx, "https://www.ecb.europa.eu/stats/eurofxref/eurofxref-daily.xml")
	if err != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("Failed to get exchange rates from ECB: %s", err.Error()))
		return nil, err
	}

	// Parse XML
	type Cube struct {
		Currency string  `xml:"currency,attr"`
		Rate     float64 `xml:"rate,attr"`
	}

	type CubeTime struct {
		Time  string `xml:"time,attr"`
		Cubes []Cube `xml:"Cube"`
	}

	type CubeWrapper struct {
		CubeTime CubeTime `xml:"Cube>Cube"`
	}

	var result CubeWrapper
	err = xml.Unmarshal(body, &result)
	if err != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("Failed to parse XML: %s", err.Error()))
		return nil, err
	}

	// ECB rates are based on EUR, we need to convert them to USD base
	// First, find the USD/EUR rate
	var usdEurRate float64
	for _, cube := range result.CubeTime.Cubes {
		if cube.Currency == "USD" {
			usdEurRate = cube.Rate
			break
		}
	}

	if usdEurRate == 0 {
		util.GetLogger().Error(ctx, "USD rate not found in ECB data")
		return nil, fmt.Errorf("USD rate not found")
	}

	// Set base USD rate
	rates["USD"] = 1.0
	// Set EUR rate
	rates["EUR"] = 1.0 / usdEurRate

	// Convert other rates to USD base
	for _, cube := range result.CubeTime.Cubes {
		if cube.Currency == "USD" {
			continue
		}
		// Convert EUR based rate to USD based rate
		rates[cube.Currency] = cube.Rate / usdEurRate
	}

	if len(rates) < 2 {
		util.GetLogger().Error(ctx, "Failed to parse enough exchange rates from ECB")
		return nil, fmt.Errorf("failed to parse exchange rates")
	}

	util.GetLogger().Info(ctx, fmt.Sprintf("Successfully parsed %d exchange rates from ECB", len(rates)))
	return rates, nil
}

func (m *CurrencyModule) TokenPatterns() []core.TokenPattern {
	return []core.TokenPattern{
		{
			Pattern:   `([0-9]+(?:\.[0-9]+)?)\s*(?i)(usd|eur|gbp|jpy|cny|aud|cad)`,
			Type:      core.IdentToken,
			Priority:  1000,
			FullMatch: false,
		},
		{
			Pattern:   `(?i)(?:in|to)\s+(usd|eur|gbp|jpy|cny|aud|cad)`,
			Type:      core.ConversionToken,
			Priority:  900,
			FullMatch: false,
		},
		{
			Pattern:   `(?i)=\s*\?\s*(usd|eur|gbp|jpy|cny|aud|cad)`,
			Type:      core.ConversionToken,
			Priority:  800,
			FullMatch: false,
		},
	}
}
