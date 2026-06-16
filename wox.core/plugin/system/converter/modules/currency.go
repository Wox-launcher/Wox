package modules

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
	"wox/plugin"
	"wox/plugin/system/converter/core"
	"wox/util"

	"github.com/PuerkitoBio/goquery"
	"github.com/shopspring/decimal"
)

type CurrencyModule struct {
	*regexBaseModule
	rates         *util.HashMap[string, float64]
	rateUpdatedAt atomic.Int64
}

var supportedCurrencyCodes = []string{
	"usd", "eur", "gbp", "jpy", "cny", "aud", "cad",
	"hkd", "sgd", "chf", "nzd", "sek", "nok", "dkk",
	"pln", "czk", "huf", "ron", "bgn", "isk", "try",
	"inr", "krw", "mxn", "brl", "zar", "thb", "myr",
	"idr", "ils", "php",
}

// Use one supported-currency pattern for tokenizer and handlers. The previous
// duplicated regex fragments made it easy to add a rate without making the
// query parseable, so this shared pattern keeps both stages in sync.
var supportedCurrencyPattern = `(?i)(` + strings.Join(supportedCurrencyCodes, "|") + `)`

func NewCurrencyModule(ctx context.Context, api plugin.API) *CurrencyModule {
	m := &CurrencyModule{
		rates: util.NewHashMap[string, float64](),
	}

	// Keep offline fallback rates aligned with the tokenizer-supported currency list.
	// The old hard-coded seven-currency list rejected common queries such as
	// "10000hkd in cny" before the converter could calculate anything. These
	// approximate USD-based rates let common currencies work until HKAB/ECB sync
	// replaces them with live data.
	defaultRates := map[string]float64{
		"USD": 1.0,     // Base currency
		"EUR": 0.92,    // Approximate
		"GBP": 0.79,    // Approximate
		"JPY": 150.0,   // Approximate
		"CNY": 7.2,     // Approximate
		"AUD": 1.52,    // Approximate
		"CAD": 1.36,    // Approximate
		"HKD": 7.82,    // Approximate
		"SGD": 1.34,    // Approximate
		"CHF": 0.88,    // Approximate
		"NZD": 1.65,    // Approximate
		"SEK": 10.5,    // Approximate
		"NOK": 10.8,    // Approximate
		"DKK": 6.85,    // Approximate
		"PLN": 4.0,     // Approximate
		"CZK": 23.0,    // Approximate
		"HUF": 360.0,   // Approximate
		"RON": 4.6,     // Approximate
		"BGN": 1.8,     // Approximate
		"ISK": 140.0,   // Approximate
		"TRY": 32.0,    // Approximate
		"INR": 83.0,    // Approximate
		"KRW": 1350.0,  // Approximate
		"MXN": 17.0,    // Approximate
		"BRL": 5.0,     // Approximate
		"ZAR": 18.5,    // Approximate
		"THB": 36.0,    // Approximate
		"MYR": 4.7,     // Approximate
		"IDR": 15600.0, // Approximate
		"ILS": 3.7,     // Approximate
		"PHP": 56.0,    // Approximate
	}
	for currency, rate := range defaultRates {
		m.rates.Store(currency, rate)
	}

	const (
		numberPattern = `([0-9]+(?:\.[0-9]+)?)`
	)

	// Initialize pattern handlers with atomic patterns
	handlers := []*patternHandler{
		{
			Pattern:     numberPattern + `\s*` + supportedCurrencyPattern,
			Priority:    1000,
			Description: "Handle currency amount (e.g., 10 USD)",
			Handler:     m.handleSingleCurrency,
		},
		{
			Pattern:     `in\s+` + supportedCurrencyPattern,
			Priority:    900,
			Description: "Handle 'in' conversion format (e.g., in EUR)",
			Handler:     m.handleInConversion,
		},
		{
			Pattern:     `to\s+` + supportedCurrencyPattern,
			Priority:    800,
			Description: "Handle 'to' conversion format (e.g., to EUR)",
			Handler:     m.handleToConversion,
		},
		{
			Pattern:     `=\s*\?\s*` + supportedCurrencyPattern,
			Priority:    700,
			Description: "Handle '=?' conversion format (e.g., =?EUR)",
			Handler:     m.handleToConversion,
		},
	}

	m.regexBaseModule = NewRegexBaseModule(api, "currency", handlers)
	return m
}

// exchangeRateSource keeps a parser paired with its display name so refresh
// logs and future result metadata can identify where the live rates came from.
type exchangeRateSource struct {
	name  string
	parse func(context.Context) (map[string]float64, error)
}

func (m *CurrencyModule) StartExchangeRateSyncSchedule(ctx context.Context) {
	util.Go(ctx, "currency_exchange_rate_sync", func() {
		// Try named data sources so successful refreshes can be surfaced in the
		// result tail. Previously users could see a converted value without any
		// signal that live rates had actually refreshed.
		sources := []exchangeRateSource{
			{name: "HKAB", parse: m.parseExchangeRateFromHKAB},
			{name: "ECB", parse: m.parseExchangeRateFromECB},
		}

		for _, source := range sources {
			rates, err := source.parse(ctx)
			if err != nil {
				util.GetLogger().Warn(ctx, fmt.Sprintf("Failed to update rates from %s: %s", source.name, err.Error()))
				continue
			}
			if len(rates) == 0 {
				// Treat an empty response as a failed refresh. The previous loop
				// only checked err and could log a successful update without any
				// usable rates, which made rate freshness hard to trust.
				util.GetLogger().Warn(ctx, fmt.Sprintf("Failed to update rates from %s: no rates parsed", source.name))
				continue
			}

			m.applyLiveRates(rates)
			m.logLiveRateUpdate(ctx, source.name, rates)
			break
		}

		for range time.NewTicker(1 * time.Hour).C {
			for _, source := range sources {
				rates, err := source.parse(ctx)
				if err != nil {
					util.GetLogger().Warn(ctx, fmt.Sprintf("Failed to update rates from %s: %s", source.name, err.Error()))
					continue
				}
				if len(rates) == 0 {
					// Keep hourly refresh semantics identical to startup: an empty
					// parse must not advance the refresh timestamp or produce a
					// misleading "updated" log line.
					util.GetLogger().Warn(ctx, fmt.Sprintf("Failed to update rates from %s: no rates parsed", source.name))
					continue
				}

				m.applyLiveRates(rates)
				m.logLiveRateUpdate(ctx, source.name, rates)
				break
			}
		}
	})
}

func (m *CurrencyModule) applyLiveRates(rates map[string]float64) {
	// Record the refresh timestamp only after rates are stored. That keeps the UI
	// tail tied to data the converter can actually use, instead of showing a
	// misleading "fresh" marker for a failed refresh attempt.
	for k, v := range rates {
		m.rates.Store(k, v)
	}
	m.rateUpdatedAt.Store(util.GetSystemTimestamp())
}

func (m *CurrencyModule) logLiveRateUpdate(ctx context.Context, source string, rates map[string]float64) {
	// Include the currencies involved in the most common local verification path.
	// A plain source-level success log was not enough to tell whether a specific
	// query such as "1000hkd in cny" had both currencies from the live feed.
	_, hasHKD := rates["HKD"]
	_, hasCNY := rates["CNY"]
	util.GetLogger().Info(ctx, fmt.Sprintf("Successfully updated %d rates from %s (HKD=%t, CNY=%t)", len(rates), source, hasHKD, hasCNY))
}

// LastRateUpdatedAt returns the last successful live-rate refresh time in
// milliseconds. A zero value means the module is still using startup fallback
// rates, which the converter exposes as a warning tail.
func (m *CurrencyModule) LastRateUpdatedAt() int64 {
	return m.rateUpdatedAt.Load()
}

func (m *CurrencyModule) Convert(ctx context.Context, value core.Result, toUnit core.Unit) (core.Result, error) {
	fromCurrency := value.Unit.Name
	toCurrency := toUnit.Name

	// Check if currencies are supported
	fromRate, fromOk := m.rates.Load(fromCurrency)
	toRate, toOk := m.rates.Load(toCurrency)

	if !fromOk {
		return core.Result{}, fmt.Errorf("unsupported currency: %s", fromCurrency)
	}
	if !toOk {
		return core.Result{}, fmt.Errorf("unsupported currency: %s", toCurrency)
	}

	// Convert to USD first (as base currency), then to target currency
	amountFloat, _ := value.RawValue.Float64()
	amountInUSD := amountFloat / fromRate
	result := amountInUSD * toRate
	resultDecimal := decimal.NewFromFloat(result)

	return core.Result{
		DisplayValue: m.formatWithCurrencySymbol(resultDecimal, toCurrency),
		RawValue:     resultDecimal,
		Unit:         toUnit,
		Module:       m,
	}, nil
}

func (m *CurrencyModule) CanConvertTo(unit string) bool {
	return m.rates.Exist(strings.ToUpper(unit))
}

// Helper functions

func (m *CurrencyModule) handleSingleCurrency(ctx context.Context, matches []string) (core.Result, error) {
	amount, err := decimal.NewFromString(matches[1])
	if err != nil {
		return core.Result{}, fmt.Errorf("invalid amount: %s", matches[1])
	}

	currency := strings.ToUpper(matches[2])

	// Check if the currency is supported
	if !m.rates.Exist(currency) {
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
	if !m.rates.Exist(currency) {
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
	if !m.rates.Exist(currency) {
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
	// Format newly supported currencies with recognizable symbols or code prefixes.
	// A bare numeric result was not enough once the converter accepted more
	// currencies because several share the same local symbol.
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
	case "HKD":
		symbol = "HK$"
	case "SGD":
		symbol = "S$"
	case "CHF":
		symbol = "CHF "
	case "NZD":
		symbol = "NZ$"
	case "SEK":
		symbol = "kr "
	case "NOK":
		symbol = "kr "
	case "DKK":
		symbol = "kr "
	case "PLN":
		symbol = "zł "
	case "CZK":
		symbol = "Kč "
	case "HUF":
		symbol = "Ft "
	case "RON":
		symbol = "lei "
	case "BGN":
		symbol = "лв "
	case "ISK":
		symbol = "kr "
	case "TRY":
		symbol = "₺"
	case "INR":
		symbol = "₹"
	case "KRW":
		symbol = "₩"
	case "MXN":
		symbol = "MX$"
	case "BRL":
		symbol = "R$"
	case "ZAR":
		symbol = "R "
	case "THB":
		symbol = "฿"
	case "MYR":
		symbol = "RM "
	case "IDR":
		symbol = "Rp "
	case "ILS":
		symbol = "₪"
	case "PHP":
		symbol = "₱"
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
	usdToHkd := usdRate / 100.0

	// HKAB quotes every listed foreign currency in HKD, so HKD itself is not a
	// table row. The old live refresh therefore left HKD on the startup fallback
	// after a successful HKAB sync; derive HKD per USD from the USD row so HKD
	// queries use the same live snapshot as the other HKAB currencies.
	rates["HKD"] = usdToHkd

	// Second pass: calculate all rates relative to USD
	for currency, rate := range rawRates {
		// Convert rates relative to USD
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
	// Currency tokens must carry their owner module into parsing. Without this,
	// the generic parser can retry earlier regex modules and let short unit aliases
	// reinterpret strings like "1000hkd" before currency conversion runs.
	return []core.TokenPattern{
		{
			Pattern:   `([0-9]+(?:\.[0-9]+)?)\s*` + supportedCurrencyPattern,
			Type:      core.IdentToken,
			Priority:  1000,
			FullMatch: false,
			Module:    m,
		},
		{
			Pattern:   `(?i)(?:in|to)\s+` + supportedCurrencyPattern,
			Type:      core.ConversionToken,
			Priority:  900,
			FullMatch: false,
			Module:    m,
		},
		{
			Pattern:   `(?i)=\s*\?\s*` + supportedCurrencyPattern,
			Type:      core.ConversionToken,
			Priority:  800,
			FullMatch: false,
			Module:    m,
		},
	}
}
