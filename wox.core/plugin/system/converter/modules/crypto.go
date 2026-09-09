package modules

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"sync"
	"time"
	"wox/util"
)

type CryptoModule struct {
	prices           *util.HashMap[string, float64]
	priceSyncMu      sync.Mutex
	snapshotMu       sync.RWMutex
	priceSyncStarted bool
	priceSyncCancel  context.CancelFunc
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

// NewCryptoModule initializes local prices; fetching requires the separate consent-gated start.
func NewCryptoModule() *CryptoModule {
	m := &CryptoModule{
		prices: util.NewHashMap[string, float64](),
	}

	m.prices.Store("btc", 80000.0) // Approximate BTC price in USD
	m.prices.Store("eth", 3000.0)  // Approximate ETH price in USD
	m.prices.Store("usdt", 1.0)    // USDT is pegged to USD
	m.prices.Store("bnb", 850.0)   // Approximate BNB price in USD

	return m
}

// StartPriceSyncSchedule starts the immediate and periodic price refresh at most once.
func (m *CryptoModule) StartPriceSyncSchedule(ctx context.Context, onInitialSyncFinished func()) bool {
	m.priceSyncMu.Lock()
	if m.priceSyncStarted {
		m.priceSyncMu.Unlock()
		return false
	}
	syncCtx, cancel := context.WithCancel(ctx)
	m.priceSyncStarted = true
	m.priceSyncCancel = cancel
	m.priceSyncMu.Unlock()

	util.Go(syncCtx, "crypto_price_sync", func() {
		prices, err := m.fetchCryptoPrices(syncCtx)
		if err == nil {
			m.applyPrices(prices)
		} else if syncCtx.Err() == nil {
			util.GetLogger().Error(syncCtx, fmt.Sprintf("Failed to fetch initial crypto prices: %s", err.Error()))
		}
		if syncCtx.Err() == nil && onInitialSyncFinished != nil {
			onInitialSyncFinished()
		}

		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-syncCtx.Done():
				return
			case <-ticker.C:
				prices, err := m.fetchCryptoPrices(syncCtx)
				if err == nil {
					m.applyPrices(prices)
				} else if syncCtx.Err() == nil {
					util.GetLogger().Error(syncCtx, fmt.Sprintf("Failed to fetch crypto prices: %s", err.Error()))
				}
			}
		}
	})
	return true
}

// StopPriceSyncSchedule cancels the refresh loop and releases its ticker.
func (m *CryptoModule) StopPriceSyncSchedule() {
	m.priceSyncMu.Lock()
	defer m.priceSyncMu.Unlock()
	if m.priceSyncCancel != nil {
		m.priceSyncCancel()
		m.priceSyncCancel = nil
	}
}

func (m *CryptoModule) fetchCryptoPrices(ctx context.Context) (map[string]float64, error) {
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

	return prices, nil
}

// applyPrices publishes a complete refresh before any query can copy it.
func (m *CryptoModule) applyPrices(prices map[string]float64) {
	m.snapshotMu.Lock()
	defer m.snapshotMu.Unlock()
	for k, v := range prices {
		m.prices.Store(k, v)
	}
}

// Snapshot returns independent USD prices; parsing never needs this data.
func (m *CryptoModule) Snapshot() map[string]*big.Rat {
	m.snapshotMu.RLock()
	defer m.snapshotMu.RUnlock()
	prices := map[string]*big.Rat{}
	m.prices.Range(func(code string, price float64) bool {
		if price > 0 {
			r, ok := new(big.Rat).SetString(strconv.FormatFloat(price, 'f', -1, 64))
			if ok {
				prices[strings.ToUpper(code)] = r
			}
		}
		return true
	})
	return prices
}
