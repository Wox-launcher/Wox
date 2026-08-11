package converter

import (
	"context"
	"testing"
	"wox/plugin"
	"wox/plugin/system/converter/core"
	"wox/plugin/system/converter/modules"
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
	registry := core.NewModuleRegistry()
	registry.Register(modules.NewTimeModule(ctx, api))
	converter := &Converter{
		api:       api,
		registry:  registry,
		tokenizer: core.NewTokenizer(registry.GetTokenPatterns()),
	}

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
	registry := core.NewModuleRegistry()
	registry.Register(modules.NewTimeModule(ctx, api))
	converter := &Converter{
		api:       api,
		registry:  registry,
		tokenizer: core.NewTokenizer(registry.GetTokenPatterns()),
	}

	response := converter.Query(ctx, plugin.Query{Type: plugin.QueryTypeInput, RawQuery: "time in nowhere-invalid", Search: "time in nowhere-invalid"})

	if response.AutoRecordQueryHistory {
		t.Fatal("invalid converter query opted into automatic history")
	}
}
