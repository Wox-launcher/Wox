package modules

import (
	"sync"
	"testing"
)

// TestPriceSnapshotsAreIndependent checks both copy isolation and whole-refresh consistency.
func TestPriceSnapshotsAreIndependent(t *testing.T) {
	m := NewCurrencyModule()
	m.applyLiveRates(map[string]float64{"USD": 1, "EUR": 2, "GBP": 2})
	before, stamp := m.Snapshot()
	before["EUR"].SetInt64(99)
	after, _ := m.Snapshot()
	if after["EUR"].RatString() != "1/2" || stamp == 0 {
		t.Fatal("snapshot leaked mutable price storage")
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			rate := float64(i + 1)
			m.applyLiveRates(map[string]float64{"EUR": rate, "GBP": rate})
		}
	}()
	for i := 0; i < 100; i++ {
		prices, _ := m.Snapshot()
		if prices["EUR"].Cmp(prices["GBP"]) != 0 {
			t.Error("mixed refresh versions")
		}
	}
	wg.Wait()
	crypto := NewCryptoModule()
	old := crypto.Snapshot()
	crypto.applyPrices(map[string]float64{"btc": 10})
	if old["BTC"].RatString() != "80000" || crypto.Snapshot()["BTC"].RatString() != "10" {
		t.Fatal("crypto snapshot was not isolated")
	}
}
