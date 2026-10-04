package modules

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"math"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"wox/util"

	"github.com/PuerkitoBio/goquery"
)

type CurrencyModule struct {
	rates         *util.HashMap[string, float64]
	snapshotMu    sync.RWMutex
	rateUpdatedAt atomic.Int64
}

// defaultCurrencyRates is the fiat allowlist and offline fallback, quoted as units
// per USD. New entries use the 2026-10-03 Currency API snapshot. It covers the
// circulating ISO 4217 currencies available in that feed; funds, metals, and
// crypto are excluded. BGN remains registered for existing queries.
var defaultCurrencyRates = map[string]float64{
	"AED": 3.6725,
	"AFN": 64.45639444,
	"ALL": 81.68598718,
	"AMD": 362.49696104,
	"AOA": 918.21835868,
	"ARS": 1522.21036116,
	"AUD": 1.52,
	"AWG": 1.79,
	"AZN": 1.70000081,
	"BAM": 1.73800154,
	"BBD": 2,
	"BDT": 122.92463913,
	"BGN": 1.8,
	"BHD": 0.376,
	"BIF": 2997.75742065,
	"BMD": 1,
	"BND": 1.27936025,
	"BOB": 11.95448871,
	"BRL": 5.0,
	"BSD": 1,
	"BTN": 96.18688524,
	"BWP": 14.17293222,
	"BYN": 3.01311953,
	"BZD": 2.01437225,
	"CAD": 1.36,
	"CDF": 2296.56,
	"CHF": 0.88,
	"CLP": 990.17293844,
	"CNY": 7.2,
	"COP": 3271.03599926,
	"CRC": 458.14403338,
	"CUP": 24.00822304,
	"CVE": 97.98879727,
	"CZK": 23.0,
	"DJF": 178.28952,
	"DKK": 6.85,
	"DOP": 59.98680996,
	"DZD": 133.99726837,
	"EGP": 52.22,
	"ERN": 15,
	"ETB": 161.00075764,
	"EUR": 0.92,
	"FJD": 2.26390904,
	"FKP": 0.75533999,
	"GBP": 0.79,
	"GEL": 2.60140097,
	"GHS": 11.73506829,
	"GIP": 0.75533999,
	"GMD": 73.46796927,
	"GNF": 8745.8442211,
	"GTQ": 7.64151504,
	"GYD": 208.87720187,
	"HKD": 7.82,
	"HNL": 26.92430836,
	"HTG": 130.93958012,
	"HUF": 360.0,
	"IDR": 15600.0,
	"ILS": 3.7,
	"INR": 83.0,
	"IQD": 1310.47936967,
	"IRR": 1747474.87834207,
	"ISK": 140.0,
	"JMD": 158.29928012,
	"JOD": 0.709,
	"JPY": 150.0,
	"KES": 128.50616091,
	"KGS": 87.45541528,
	"KHR": 4055.05461866,
	"KMF": 437.17537062,
	"KPW": 900.48232116,
	"KRW": 1350.0,
	"KWD": 0.309,
	"KYD": 0.83219962,
	"KZT": 448.36878322,
	"LAK": 22546.45253105,
	"LBP": 90043.54103005,
	"LKR": 330.61722673,
	"LRD": 171.33637683,
	"LSL": 16.64673909,
	"LYD": 6.39995241,
	"MAD": 9.93526897,
	"MDL": 17.80731155,
	"MGA": 4426.08840084,
	"MKD": 54.60664593,
	"MMK": 2100.78315534,
	"MNT": 3597.06716958,
	"MOP": 8.08219416,
	"MRU": 40.07836951,
	"MUR": 48.12862858,
	"MVR": 15.45736307,
	"MWK": 1736.42004856,
	"MXN": 17.0,
	"MYR": 4.7,
	"MZN": 63.83045317,
	"NAD": 16.64673909,
	"NGN": 1330.5021385,
	"NIO": 36.65519515,
	"NOK": 10.8,
	"NPR": 153.97115654,
	"NZD": 1.65,
	"OMR": 0.38399587,
	"PAB": 1,
	"PEN": 3.43405515,
	"PGK": 4.41865972,
	"PHP": 56.0,
	"PKR": 276.65545745,
	"PLN": 4.0,
	"PYG": 5856.05149084,
	"QAR": 3.64,
	"RON": 4.6,
	"RSD": 104.33620221,
	"RUB": 83.67816657,
	"RWF": 1474.4635649,
	"SAR": 3.75,
	"SBD": 8.07561826,
	"SCR": 13.69664553,
	"SDG": 600.2056294,
	"SEK": 10.5,
	"SGD": 1.34,
	"SHP": 0.75533999,
	"SLE": 23.0565525,
	"SOS": 570.49738707,
	"SRD": 37.76216782,
	"SSP": 5712.59152749,
	"STN": 22.21947614,
	"SVC": 8.75,
	"SYP": 13008.96358905,
	"SZL": 16.64673909,
	"THB": 36.0,
	"TJS": 9.21439386,
	"TMT": 3.49571986,
	"TND": 2.9939329,
	"TOP": 2.40245467,
	"TRY": 32.0,
	"TTD": 6.7785177,
	"TWD": 31.82188553,
	"TZS": 2635.45091593,
	"UAH": 44.96739468,
	"UGX": 3992.99711488,
	"USD": 1.0,
	"UYU": 40.28565824,
	"UZS": 11818.02055136,
	"VED": 865.51774908,
	"VES": 865.51774908,
	"VND": 26002.63646348,
	"VUV": 119.62299348,
	"WST": 2.78859075,
	"XAF": 582.90049416,
	"XCD": 2.70247843,
	"XCG": 1.79938342,
	"XOF": 582.90049416,
	"XPF": 106.04129771,
	"YER": 236.39642372,
	"ZAR": 18.5,
	"ZMW": 19.63923759,
	"ZWG": 26.77164445,
}

// NewCurrencyModule initializes offline rates without starting network access.
func NewCurrencyModule() *CurrencyModule {
	m := &CurrencyModule{
		rates: util.NewHashMap[string, float64](),
	}
	for currency, rate := range defaultCurrencyRates {
		m.rates.Store(currency, rate)
	}
	return m
}

// exchangeRateSource keeps a parser paired with its display name so refresh
// logs and future result metadata can identify where the live rates came from.
type exchangeRateSource struct {
	name  string
	parse func(context.Context) (map[string]float64, error)
}

// StartExchangeRateSyncSchedule refreshes rates until the plugin lifecycle ends.
func (m *CurrencyModule) StartExchangeRateSyncSchedule(ctx context.Context) {
	util.Go(ctx, "currency_exchange_rate_sync", func() {
		// Try named data sources so successful refreshes can be surfaced in the
		// result tail. Previously users could see a converted value without any
		// signal that live rates had actually refreshed.
		// HKAB and ECB do not cover all supported fiat currencies.
		// Try the broader daily feed and its independent mirror before them.
		sources := []exchangeRateSource{
			{name: "Currency API (jsDelivr)", parse: func(ctx context.Context) (map[string]float64, error) {
				return m.parseExchangeRateFromCurrencyAPI(ctx, "https://cdn.jsdelivr.net/npm/@fawazahmed0/currency-api@latest/v1/currencies/usd.json")
			}},
			{name: "Currency API (Cloudflare)", parse: func(ctx context.Context) (map[string]float64, error) {
				return m.parseExchangeRateFromCurrencyAPI(ctx, "https://latest.currency-api.pages.dev/v1/currencies/usd.json")
			}},
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

		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
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
	m.snapshotMu.Lock()
	defer m.snapshotMu.Unlock()
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

// parseExchangeRateFromCurrencyAPI reads a USD-based daily feed from either mirror.
func (m *CurrencyModule) parseExchangeRateFromCurrencyAPI(ctx context.Context, url string) (map[string]float64, error) {
	body, err := util.HttpGet(ctx, url)
	if err != nil {
		return nil, err
	}
	return decodeCurrencyAPIRates(body)
}

// decodeCurrencyAPIRates accepts only the registered fiat currencies. The feed also
// contains crypto and metals, which must not bypass the separate crypto service.
func decodeCurrencyAPIRates(body []byte) (map[string]float64, error) {
	var result struct {
		USD map[string]float64 `json:"usd"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	if result.USD["usd"] != 1 {
		return nil, fmt.Errorf("invalid USD base rate")
	}
	rates := make(map[string]float64, len(defaultCurrencyRates))
	for code := range defaultCurrencyRates {
		rate := result.USD[strings.ToLower(code)]
		// A partial response must not stop failover to a complete daily feed.
		if rate <= 0 || math.IsNaN(rate) || math.IsInf(rate, 0) {
			return nil, fmt.Errorf("missing or invalid rate for %s", code)
		}
		rates[code] = rate
	}
	return rates, nil
}

// Snapshot copies USD-per-unit prices and their timestamp under the refresh lock.
func (m *CurrencyModule) Snapshot() (map[string]*big.Rat, int64) {
	m.snapshotMu.RLock()
	defer m.snapshotMu.RUnlock()
	prices := map[string]*big.Rat{}
	m.rates.Range(func(code string, rate float64) bool {
		if rate > 0 {
			r, ok := new(big.Rat).SetString(strconv.FormatFloat(rate, 'f', -1, 64))
			if ok {
				prices[strings.ToUpper(code)] = new(big.Rat).Inv(r)
			}
		}
		return true
	})
	return prices, m.rateUpdatedAt.Load()
}

// CurrencyCodes exposes syntax without requiring prices or network access.
func CurrencyCodes() []string {
	codes := make([]string, 0, len(defaultCurrencyRates))
	for code := range defaultCurrencyRates {
		codes = append(codes, strings.ToLower(code))
	}
	sort.Strings(codes)
	return codes
}
