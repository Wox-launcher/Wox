package modules

import (
	"testing"
	"wox/util"
)

func TestParseExchangeRateFromHKAB(t *testing.T) {
	ctx := util.NewTraceContext()
	err := util.GetLocation().Init()
	if err != nil {
		panic(err)
	}

	m := &CurrencyModule{
		rates: make(map[string]float64),
	}

	rates, err := m.parseExchangeRateFromECB(ctx)
	if err != nil {
		t.Errorf("TestParseExchangeRateFromHKAB failed: %v", err)
		return
	}

	// Check if we have rates
	if len(rates) < 2 {
		t.Errorf("Expected at least 2 rates, got %d", len(rates))
		return
	}

	// Check USD rate
	if rates["USD"] != 1.0 {
		t.Errorf("Expected USD rate to be 1.0, got %f", rates["USD"])
	}

	// Check CNY rate is in reasonable range (6-8)
	cnyRate := rates["CNY"]
	if cnyRate < 6.0 || cnyRate > 8.0 {
		t.Errorf("CNY rate %f is outside expected range [6.0, 8.0]", cnyRate)
	}
}
