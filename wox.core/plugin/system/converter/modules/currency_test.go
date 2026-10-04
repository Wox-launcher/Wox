package modules

import (
	"encoding/json"
	"math/big"
	"os"
	"testing"
	"wox/util"
)

// TestCurrencyAPIRates checks feed decoding, fiat isolation, and USD price direction.
func TestCurrencyAPIRates(t *testing.T) {
	body, err := os.ReadFile("testdata/currency_api_usd.json")
	if err != nil {
		t.Fatal(err)
	}
	rates, err := decodeCurrencyAPIRates(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(rates) != len(CurrencyCodes()) || rates["BTC"] != 0 || rates["XAU"] != 0 {
		t.Fatalf("unexpected fiat coverage: %v", rates)
	}
	m := NewCurrencyModule()
	m.applyLiveRates(rates)
	prices, updated := m.Snapshot()
	if updated == 0 {
		t.Fatal("live refresh did not record a timestamp")
	}
	for code, rate := range map[string]string{"AED": "3.6725", "SAR": "3.75", "EGP": "52.21607746", "JOD": "0.709", "KWD": "0.30890656"} {
		want, _ := new(big.Rat).SetString(rate)
		want.Inv(want)
		if prices[code] == nil || prices[code].Cmp(want) != 0 {
			t.Errorf("%s: got %v USD per unit, want %v", code, prices[code], want)
		}
	}
}

// TestCurrencyAPIRejectsInvalidRates ensures corrupt or partial feeds allow failover.
func TestCurrencyAPIRejectsInvalidRates(t *testing.T) {
	body, err := os.ReadFile("testdata/currency_api_usd.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []string{`{`, `{}`, `{"usd":{"usd":1}}`, `{"usd":{"usd":1,"aed":1e999}}`} {
		if _, err := decodeCurrencyAPIRates([]byte(invalid)); err == nil {
			t.Errorf("accepted invalid response: %s", invalid)
		}
	}
	for _, rate := range []float64{0, -1, 2} {
		var fixture struct {
			USD map[string]float64 `json:"usd"`
		}
		if err := json.Unmarshal(body, &fixture); err != nil {
			t.Fatal(err)
		}
		if rate == 2 {
			fixture.USD["usd"] = rate
		} else {
			fixture.USD["aed"] = rate
		}
		invalid, err := json.Marshal(fixture)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := decodeCurrencyAPIRates(invalid); err == nil {
			t.Fatalf("accepted invalid rate %v", rate)
		}
	}
}

func TestParseExchangeRateFromHKAB(t *testing.T) {
	if os.Getenv("WOX_TEST_ENABLE_NETWORK") == "false" {
		t.Skip("external price service disabled by WOX_TEST_ENABLE_NETWORK")
	}
	ctx := util.NewTraceContext()
	err := util.GetLocation().Init()
	if err != nil {
		panic(err)
	}

	m := &CurrencyModule{
		rates: util.NewHashMap[string, float64](),
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
