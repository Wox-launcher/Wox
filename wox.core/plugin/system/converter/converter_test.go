package converter

import (
	"context"
	"testing"
	"wox/plugin"
)

type converterTestAPI struct {
	plugin.API
}

func (converterTestAPI) GetSetting(ctx context.Context, key string) string { return "" }
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

func TestConverterRoutingAndCopyActions(t *testing.T) {
	c := &Converter{api: converterTestAPI{}, catalog: newCatalog()}
	ctx := context.Background()
	for _, input := range []string{"sqrt(625)", "sin(1) + 2", "(2+3)*4"} {
		if r := c.Query(ctx, plugin.Query{Search: input}); len(r.Results) != 0 {
			t.Errorf("duplicated calculator result for %s", input)
		}
	}
	for _, input := range []string{"square root of 625", "2 power 10", "cot(1)", "(1h+30min)/2 to minutes"} {
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
