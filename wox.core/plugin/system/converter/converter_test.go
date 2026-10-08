package converter

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"
	"wox/plugin"
	"wox/plugin/system/converter/engine"
	"wox/plugin/system/converter/modules"
	"wox/setting/definition"
	"wox/util/locale"
)

// TestMiddleEasternCurrencies covers the production catalog and offline price path
// in both directions, including cross-currency conversions and compact input.
func TestMiddleEasternCurrencies(t *testing.T) {
	m := modules.NewCurrencyModule()
	c := &Converter{api: converterTestAPI{}, catalog: newCatalog(), currencyModule: m}
	prices, updated := m.Snapshot()
	if updated != 0 {
		t.Fatal("offline prices were marked as live")
	}
	codes := []string{"USD", "AED", "SAR", "EGP", "JOD", "KWD"}
	for _, from := range codes {
		for _, to := range codes {
			input := fmt.Sprintf("100%s to %s", strings.ToLower(from), to)
			t.Run(input, func(t *testing.T) {
				query, err := c.catalog.Parse(input, engine.ParseOptions{DecimalSeparator: "."})
				if err != nil {
					t.Fatal(err)
				}
				result, err := c.catalog.Evaluate(context.Background(), query, engine.Env{Local: time.UTC, Prices: prices})
				if err != nil {
					t.Fatal(err)
				}
				want := new(big.Rat).Quo(prices[from], prices[to])
				want.Mul(want, big.NewRat(100, 1))
				if result.Value.Number.Cmp(want) != 0 || result.Value.Unit[to] != 1 {
					t.Fatalf("got %+v, want %v %s", result.Value, want, to)
				}
				response := c.Query(context.Background(), plugin.Query{Search: input})
				if len(response.Results) != 1 || !response.AutoRecordQueryHistory || len(response.Results[0].Actions) != 3 {
					t.Fatalf("missing currency result or copy actions: %+v", response)
				}
			})
		}
	}
}

type converterTestAPI struct {
	plugin.API
	settings map[string]string
}

// TestFiatCurrencyCatalog exercises every registered code through the production
// parser, including prefix notation and both sides of a USD conversion.
func TestFiatCurrencyCatalog(t *testing.T) {
	c := newCatalog()
	prices, _ := modules.NewCurrencyModule().Snapshot()
	for _, code := range modules.CurrencyCodes() {
		canonical := strings.ToUpper(code)
		if prices[canonical] == nil || prices[canonical].Sign() <= 0 {
			t.Fatalf("no offline price for %s", canonical)
		}
		spellings := []string{canonical, code}
		if canonical == "CUP" {
			spellings = []string{canonical}
		}
		for _, spelling := range spellings {
			for _, tc := range []struct{ input, from, to string }{
				{"100" + spelling + " to usd", canonical, "USD"},
				{spelling + "100 to USD", canonical, "USD"},
				{"100 usd to " + spelling, "USD", canonical},
			} {
				t.Run(tc.input, func(t *testing.T) {
					q, err := c.Parse(tc.input, engine.ParseOptions{DecimalSeparator: "."})
					if err != nil {
						t.Fatal(err)
					}
					if !q.Money || q.Crypto {
						t.Fatal("fiat query was not recognized as fiat")
					}
					result, err := c.Evaluate(context.Background(), q, engine.Env{Local: time.UTC, Prices: prices})
					if err != nil {
						t.Fatal(err)
					}
					want := new(big.Rat).Mul(big.NewRat(100, 1), prices[tc.from])
					want.Quo(want, prices[tc.to])
					if result.Value.Number.Cmp(want) != 0 || result.Value.Unit[tc.to] != 1 {
						t.Fatalf("got %+v, want %s %s", result.Value, want, tc.to)
					}
				})
			}
		}
	}
}

// TestFiatCurrencyUnitCollisions protects cooking units when CUP is registered.
func TestFiatCurrencyUnitCollisions(t *testing.T) {
	c := &Converter{api: converterTestAPI{}, catalog: newCatalog(), currencyModule: modules.NewCurrencyModule()}
	for _, input := range []string{"1 cup to ml", "1 Cup to ml", "1 cups to ml", "1 cup + 1 cup to ml", "236.5882365 ml to cup"} {
		q, err := c.catalog.Parse(input, engine.ParseOptions{DecimalSeparator: "."})
		if err != nil || q.Money {
			t.Fatalf("%s was treated as currency: %v", input, err)
		}
		response := c.Query(context.Background(), plugin.Query{Search: input})
		if len(response.Results) != 1 {
			t.Fatalf("cooking conversion stopped working: %s", input)
		}
	}
	for _, input := range []string{"100 cup to usd", "100 usd to cup"} {
		if r := c.Query(context.Background(), plugin.Query{Search: input}); len(r.Results) != 0 {
			t.Fatalf("lowercase cup silently became currency: %s", input)
		}
	}
}

func (a converterTestAPI) GetSetting(ctx context.Context, key string) string { return a.settings[key] }
func (converterTestAPI) GetTranslation(ctx context.Context, key string) string {
	if key == "plugin_converter_time_format" {
		return "%02d:%02d (%s)"
	}
	return key
}
func (converterTestAPI) Log(ctx context.Context, level plugin.LogLevel, msg string) {}

func TestTimeDisplayResultOptsIntoAutomaticQueryHistory(t *testing.T) {
	ctx := context.Background()
	api := converterTestAPI{}
	converter := &Converter{api: api, catalog: newCatalog()}

	response := converter.Query(ctx, plugin.Query{Type: plugin.QueryTypeInput, RawQuery: "time in ny", Search: "time in ny"})

	if len(response.Results) != 1 {
		t.Fatalf("time query returned %d results", len(response.Results))
	}
	if !response.AutoRecordQueryHistory {
		t.Fatal("successful time query did not opt into automatic history")
	}
}

func TestInvalidConverterQueryDoesNotOptIntoAutomaticQueryHistory(t *testing.T) {
	ctx := context.Background()
	api := converterTestAPI{}
	converter := &Converter{api: api, catalog: newCatalog()}

	response := converter.Query(ctx, plugin.Query{Type: plugin.QueryTypeInput, RawQuery: "time in nowhere-invalid", Search: "time in nowhere-invalid"})

	if response.AutoRecordQueryHistory {
		t.Fatal("invalid converter query opted into automatic history")
	}
}

// TestCryptoConsentPrecedesPrices deliberately omits all price services: syntax
// alone must produce consent, and incomplete syntax must not start data access.
func TestCryptoConsentPrecedesPrices(t *testing.T) {
	c := &Converter{api: converterTestAPI{}, catalog: newCatalog()}
	r := c.Query(context.Background(), plugin.Query{Search: "(1BTC + 1 ETH) to USD"})
	if len(r.Results) != 1 || r.Results[0].Id != cryptoConsentResultID {
		t.Fatalf("expected consent: %+v", r)
	}
	if r.AutoRecordQueryHistory {
		t.Fatal("consent is not a calculated result")
	}
	if r = c.Query(context.Background(), plugin.Query{Search: "1BTC +"}); len(r.Results) != 0 {
		t.Fatal("incomplete input requested consent")
	}
}

func TestConverterDecimalRounding(t *testing.T) {
	c := &Converter{api: converterTestAPI{}, catalog: newCatalog()}
	for _, tc := range []struct{ input, title string }{
		{"1/3 to 2 dp", "0.33"},
		{"π to 5 digits", "3.14159"},
		{"21 rounded up to nearest 5", "25"},
		{"17 rounded down to nearest 3", "15"},
		{"cube root of 27", "3"},
		{"meters in 10 km", "10000 meters"},
		{"days in 3 weeks", "21 days"},
		{"seconds in a day", "86400 seconds"},
		{"300 + 20 km", "320 kilometers"},
		{"5.5 minutes as timespan", "5 min 30 s"},
		{"5.5 minutes as laptime", "00:05:30"},
		{"03:04:05 + 01:02:03", "04:06:08"},
		{"time saved 5 min at 1.5x", "1 min 40 s"},
		{"10% on 200", "220"},
		{"20 is 10% of what", "200"},
		{"average of 36, 42, 19 and 81", "44.5"},
		{"256 as hex", "0x100"},
		{"cmFpbDE2Mw==", "rail163"},
		{"hello to base64", "aGVsbG8="},
	} {
		r := c.Query(context.Background(), plugin.Query{Search: tc.input})
		if len(r.Results) != 1 || r.Results[0].Title != tc.title {
			t.Fatalf("%s: %+v", tc.input, r)
		}
	}
}

func TestConverterRoutingAndCopyActions(t *testing.T) {
	c := &Converter{api: converterTestAPI{}, catalog: newCatalog()}
	ctx := context.Background()
	for _, input := range []string{"sqrt(625)", "sin(1) + 2", "(2+3)*4", "1/3"} {
		if r := c.Query(ctx, plugin.Query{Search: input}); len(r.Results) != 0 {
			t.Errorf("duplicated calculator result for %s", input)
		}
	}
	for _, input := range []string{"square root of 625", "cube root of 27", "2 power 10", "cot(1)", "(1h+30min)/2 to minutes", "1/3 to 2 dp", "π to 5 digits", "21 rounded up to nearest 5", "17 rounded down to nearest 3", "meters in 10 km", "seconds in a day", "300 + 20 km", "Tokyo time", "7:30am LAX to Japan", "time difference between Seattle and Moscow", "5.5 minutes as timespan", "03:04:05 + 01:02:03", "time saved 5 min at 1.5x", "10% on 200", "256 as hex", "cmFpbDE2Mw=="} {
		r := c.Query(ctx, plugin.Query{Search: input})
		if len(r.Results) != 1 || len(r.Results[0].Actions) != 3 {
			t.Fatalf("missing result/actions for %s: %+v", input, r)
		}
		for _, a := range r.Results[0].Actions {
			if a.ContextData["query"] != input {
				t.Fatal("MRU lost original query")
			}
		}
	}
}

// TestDefaultCurrencyForRegion covers regional currencies, aliases, and fallback.
func TestDefaultCurrencyForRegion(t *testing.T) {
	for _, tc := range []struct{ region, currency string }{
		{"BR", "BRL"}, {"br", "BRL"}, {"US", "USD"}, {"GB", "GBP"}, {"UK", "GBP"},
		{"EU", "EUR"}, {"PT", "EUR"}, {"IE", "EUR"},
		{"IN", "INR"}, {"CH", "CHF"}, {"NZ", "NZD"}, {"KR", "KRW"}, {"ZA", "ZAR"},
		{"AE", "AED"}, {"SA", "SAR"}, {"EG", "EGP"}, {"JO", "JOD"}, {"KW", "KWD"},
		{"ST", "STN"}, {"PS", "ILS"},
		{"", "USD"}, {"ZZ", "USD"}, {"AQ", "USD"}, {"419", "USD"}, {"invalid", "USD"}, {"pt-BR", "USD"},
	} {
		t.Run(tc.region, func(t *testing.T) {
			if got := currencyForRegion(tc.region); got != tc.currency {
				t.Fatalf("got %s, want %s", got, tc.currency)
			}
		})
	}
}

// TestDefaultCurrencyCatalog tests the default currency for the current OS locale.
func TestDefaultCurrencyCatalog(t *testing.T) {
	selected := currencyForRegion(locale.GetCurrencyRegion())
	want := "USD"
	for _, code := range modules.CurrencyCodes() {
		if strings.EqualFold(selected, code) {
			want = selected
			break
		}
	}
	if got := GetUserDefaultCurrency(); got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

// TestDefaultCurrencySetting tests Auto, every fiat option, and invalid saved values.
func TestDefaultCurrencySetting(t *testing.T) {
	ctx := context.Background()
	settings := map[string]string{}
	c := &Converter{api: converterTestAPI{settings: settings}, catalog: newCatalog()}
	metadata := c.GetMetadata()
	if len(metadata.SettingDefinitions) != 1 {
		t.Fatalf("got %d settings, want 1", len(metadata.SettingDefinitions))
	}
	selectSetting, ok := metadata.SettingDefinitions[0].Value.(*definition.PluginSettingValueSelect)
	if !ok || selectSetting.Key != defaultCurrencySettingKey || selectSetting.DefaultValue != "auto" || !selectSetting.Filterable {
		t.Fatalf("invalid default currency setting: %+v", metadata.SettingDefinitions[0])
	}
	codes := modules.CurrencyCodes()
	if len(selectSetting.Options) != len(codes)+1 || selectSetting.Options[0].Value != "auto" {
		t.Fatal("expected Auto and all fiat currencies")
	}
	for i, code := range codes {
		want := strings.ToUpper(code)
		if selectSetting.Options[i+1].Value != want {
			t.Fatalf("option %d: got %q, want %q", i+1, selectSetting.Options[i+1].Value, want)
		}
		settings[defaultCurrencySettingKey] = want
		if got := c.defaultCurrency(ctx); got != want {
			t.Fatalf("setting %q: got %q, want %q", want, got, want)
		}
	}
	for _, value := range []string{"", "auto", "invalid", "BTC", "m"} {
		settings[defaultCurrencySettingKey] = value
		if got := c.defaultCurrency(ctx); got != GetUserDefaultCurrency() {
			t.Fatalf("setting %q: got %q, want Auto", value, got)
		}
	}
	settings[defaultCurrencySettingKey] = " eur "
	if got := c.defaultCurrency(ctx); got != "EUR" {
		t.Fatalf("got %q, want EUR", got)
	}
}

// TestConverterDefaultCurrencyQueries tests setting changes and conversion after arithmetic.
func TestConverterDefaultCurrencyQueries(t *testing.T) {
	ctx := context.Background()
	settings := map[string]string{cryptoPriceSyncConsentSettingKey: "true"}
	c := &Converter{api: converterTestAPI{settings: settings}, catalog: newCatalog(), currencyModule: modules.NewCurrencyModule(), cryptoModule: modules.NewCryptoModule()}
	prices, _ := c.currencyModule.Snapshot()
	for code, price := range c.cryptoModule.Snapshot() {
		prices[code] = price
	}
	options := numberOptions()
	for _, selected := range []string{"auto", "BRL", "EUR", "USD", ""} {
		// Reuse the same plugin to ensure setting changes do not require initialization.
		settings[defaultCurrencySettingKey] = selected
		for _, tc := range []struct{ input, amount, source, target, period string }{
			{"5 usd", "5", "USD", "", ""},
			{"25 eur", "25", "EUR", "", ""},
			{"USD1K", "1000", "USD", "", ""},
			{"$20 + 30", "50", "USD", "", ""},
			{"12% of (100 USD + 50 USD)", "18", "USD", "", ""},
			{"0.001 btc", "0.001", "BTC", "", ""},
			{"1 eur + 1", "2", "EUR", "", ""},
			{"1 btc + 1", "2", "BTC", "", ""},
			{"8 usd/hour", "8", "USD", "", "h"},
			{"60 usd/hour to /minute", "1", "USD", "", "min"},
			{"5 usd to eur", "5", "USD", "EUR", ""},
			{"8 usd/hour to eur", "8", "USD", "EUR", "h"},
			{"8 usd/week to eur/month", "104/3", "USD", "EUR", "mo"},
		} {
			t.Run(selected+"/"+tc.input, func(t *testing.T) {
				target := tc.target
				if target == "" {
					target = selected
					if target == "auto" || target == "" {
						target = GetUserDefaultCurrency()
					}
				}
				number, _ := new(big.Rat).SetString(tc.amount)
				number.Mul(number, prices[tc.source]).Quo(number, prices[target])
				unit := engine.Unit{target: 1}
				if tc.period != "" {
					unit[tc.period] = -1
				}
				want := c.catalog.Format(engine.Evaluation{Value: engine.Value{Kind: engine.Quantity, Number: number, Unit: unit}}, engine.FormatOptions{DecimalSeparator: options.DecimalSeparator, ThousandsSeparator: options.ThousandsSeparator}).Formatted
				input := strings.ReplaceAll(tc.input, ".", options.DecimalSeparator)
				response := c.Query(ctx, plugin.Query{Search: input})
				if len(response.Results) != 1 || response.Results[0].Title != want {
					t.Fatalf("got %+v, want %q", response, want)
				}
			})
		}
	}
}
