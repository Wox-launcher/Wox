package modules

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"wox/plugin"
	"wox/plugin/system/converter/core"
	"wox/util"

	"github.com/shopspring/decimal"
)

type CryptoModule struct {
	*regexBaseModule
	prices map[string]float64
}

// CoinGecko API response structure
type CoinGeckoResponse struct {
	Bitcoin struct {
		Usd float64 `json:"usd"`
	} `json:"bitcoin"`
	Ethereum struct {
		Usd float64 `json:"usd"`
	} `json:"ethereum"`
	Tether struct {
		Usd float64 `json:"usd"`
	} `json:"tether"`
	BinanceCoin struct {
		Usd float64 `json:"usd"`
	} `json:"binancecoin"`
}

func NewCryptoModule(ctx context.Context, api plugin.API) *CryptoModule {
	m := &CryptoModule{
		prices: make(map[string]float64),
	}

	const (
		cryptoPattern = `(?i)(btc|eth|usdt|bnb)`
		numberPattern = `([0-9]+(?:\.[0-9]+)?)`
	)

	// Initialize pattern handlers with atomic patterns
	handlers := []*patternHandler{
		{
			Pattern:     numberPattern + `\s*` + cryptoPattern,
			Priority:    1000,
			Description: "Handle cryptocurrency amount (e.g., 1 BTC)",
			Handler:     m.handleSingleCrypto,
		},
		{
			Pattern:     `in\s+` + cryptoPattern,
			Priority:    900,
			Description: "Handle 'in' conversion format (e.g., in BTC)",
			Handler:     m.handleInConversion,
		},
		{
			Pattern:     `to\s+` + cryptoPattern,
			Priority:    800,
			Description: "Handle 'to' conversion format (e.g., to BTC)",
			Handler:     m.handleToConversion,
		},
		{
			Pattern:     `=\s*\?\s*` + cryptoPattern,
			Priority:    700,
			Description: "Handle '=?' conversion format (e.g., =?BTC)",
			Handler:     m.handleToConversion,
		},
	}

	m.regexBaseModule = NewRegexBaseModule(api, "crypto", handlers)
	return m
}

func (m *CryptoModule) StartPriceSyncSchedule(ctx context.Context) {
	util.Go(ctx, "crypto_price_sync", func() {
		prices, err := m.fetchCryptoPrices(ctx)
		if err == nil {
			m.prices = prices
		} else {
			util.GetLogger().Error(ctx, fmt.Sprintf("Failed to fetch initial crypto prices: %s", err.Error()))
		}

		for range time.NewTicker(1 * time.Minute).C {
			prices, err := m.fetchCryptoPrices(ctx)
			if err == nil {
				m.prices = prices
			} else {
				util.GetLogger().Error(ctx, fmt.Sprintf("Failed to fetch crypto prices: %s", err.Error()))
			}
		}
	})
}

func (m *CryptoModule) Convert(ctx context.Context, value core.Result, toUnit core.Unit) (core.Result, error) {
	// We only support converting to USD
	if toUnit.Name != core.UnitUSD.Name {
		return core.Result{}, fmt.Errorf("crypto module only supports converting to USD")
	}

	fromCrypto := value.Unit.Name

	// Get crypto price in USD
	cryptoPrice, ok := m.prices[fromCrypto]
	if !ok {
		return core.Result{}, fmt.Errorf("unsupported cryptocurrency: %s", fromCrypto)
	}

	// Convert amount to USD
	amountFloat, _ := value.RawValue.Float64()
	amountInUSD := amountFloat * cryptoPrice

	// Format result
	resultDecimal := decimal.NewFromFloat(amountInUSD)
	return core.Result{
		DisplayValue: fmt.Sprintf("$%s", resultDecimal.Round(2)),
		RawValue:     resultDecimal,
		Unit:         core.UnitUSD,
		Module:       m,
	}, nil
}

// Helper functions
func (m *CryptoModule) handleSingleCrypto(ctx context.Context, matches []string) (core.Result, error) {
	amount, err := decimal.NewFromString(matches[1])
	if err != nil {
		return core.Result{}, fmt.Errorf("invalid amount: %s", matches[1])
	}

	crypto := strings.ToLower(matches[2])

	// Check if the cryptocurrency is supported
	if _, ok := m.prices[crypto]; !ok {
		return core.Result{}, fmt.Errorf("unsupported cryptocurrency: %s", crypto)
	}

	// Create a result with the crypto amount
	result := core.Result{
		DisplayValue: fmt.Sprintf("%s %s", amount.String(), strings.ToUpper(crypto)),
		RawValue:     amount,
		Unit:         core.Unit{Name: crypto, Type: core.UnitTypeCrypto},
		Module:       m,
	}

	return result, nil
}

func (m *CryptoModule) handleInConversion(ctx context.Context, matches []string) (core.Result, error) {
	crypto := strings.ToLower(matches[1])
	if _, ok := m.prices[crypto]; !ok {
		return core.Result{}, fmt.Errorf("unsupported cryptocurrency: %s", crypto)
	}
	return core.Result{
		DisplayValue: fmt.Sprintf("in %s", strings.ToUpper(crypto)),
		Unit:         core.Unit{Name: crypto, Type: core.UnitTypeCrypto},
		Module:       m,
	}, nil
}

func (m *CryptoModule) handleToConversion(ctx context.Context, matches []string) (core.Result, error) {
	crypto := strings.ToLower(matches[1])
	if _, ok := m.prices[crypto]; !ok {
		return core.Result{}, fmt.Errorf("unsupported cryptocurrency: %s", crypto)
	}
	return core.Result{
		DisplayValue: fmt.Sprintf("to %s", strings.ToUpper(crypto)),
		Unit:         core.Unit{Name: crypto, Type: core.UnitTypeCrypto},
		Module:       m,
	}, nil
}

func (m *CryptoModule) fetchCryptoPrices(ctx context.Context) (map[string]float64, error) {
	util.GetLogger().Info(ctx, "Starting to fetch crypto prices from CoinGecko")

	body, err := util.HttpGet(ctx, "https://api.coingecko.com/api/v3/simple/price?ids=bitcoin,ethereum,tether,binancecoin&vs_currencies=usd")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch prices: %w", err)
	}

	var response CoinGeckoResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	prices := make(map[string]float64)
	prices["btc"] = response.Bitcoin.Usd
	prices["eth"] = response.Ethereum.Usd
	prices["usdt"] = response.Tether.Usd
	prices["bnb"] = response.BinanceCoin.Usd

	util.GetLogger().Info(ctx, fmt.Sprintf("Successfully fetched crypto prices: %v", prices))
	return prices, nil
}
