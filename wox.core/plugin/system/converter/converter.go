package converter

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"
	"wox/common"
	"wox/common/icons"
	"wox/plugin"
	"wox/plugin/system/converter/engine"
	"wox/plugin/system/converter/modules"
	"wox/util"
	"wox/util/calc"
	"wox/util/clipboard"
	"wox/util/locale"
)

const (
	cryptoPriceSyncConsentSettingKey = "cryptoPriceSyncConsent"
	cryptoConsentResultID            = "converter_crypto_price_sync_consent"
	cryptoConsentActionID            = "converter_crypto_price_sync_consent_allow"
)

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &Converter{})
}

type Converter struct {
	api            plugin.API
	lifecycleCtx   context.Context
	catalog        *engine.Catalog
	currencyModule *modules.CurrencyModule
	cryptoModule   *modules.CryptoModule
}

func (c *Converter) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            "a48dc5f0-dab9-4112-b883-b68129d6782b",
		Name:          "i18n:plugin_converter_plugin_name",
		Author:        "Wox Launcher",
		Website:       "https://github.com/Wox-launcher/Wox",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Go",
		Description:   "i18n:plugin_converter_plugin_description",
		Icon:          icons.Get(icons.PluginConverter).String(),
		Entry:         "",
		TriggerKeywords: []string{
			"*",
		},
		Commands: []plugin.MetadataCommand{},
		SupportedOS: []string{
			"Windows",
			"Macos",
			"Linux",
		},
		Features: []plugin.MetadataFeature{
			{
				Name: plugin.MetadataFeatureMRU,
				Params: map[string]any{
					"HashBy": "rawQuery",
				},
			},
		},
	}
}

func (c *Converter) Init(ctx context.Context, initParams plugin.InitParams) {
	c.api = initParams.API
	c.lifecycleCtx = ctx

	c.catalog = newCatalog()
	c.currencyModule = modules.NewCurrencyModule()
	c.currencyModule.StartExchangeRateSyncSchedule(ctx)
	cryptoModule := modules.NewCryptoModule()
	c.cryptoModule = cryptoModule

	if c.api.GetSetting(ctx, cryptoPriceSyncConsentSettingKey) == "true" {
		cryptoModule.StartPriceSyncSchedule(ctx, nil)
	}
	c.api.OnUnload(ctx, func(ctx context.Context) {
		cryptoModule.StopPriceSyncSchedule()
	})
	c.api.OnMRURestore(ctx, c.handleMRURestore)
}

// GetUserDefaultCurrency returns the user's default currency based on their locale
func GetUserDefaultCurrency() string {
	var regionToCurrency = map[string]string{
		"CN": "CNY", // China -> Chinese Yuan
		"US": "USD", // United States -> US Dollar
		"GB": "GBP", // United Kingdom -> British Pound
		"JP": "JPY", // Japan -> Japanese Yen
		"EU": "EUR", // European Union -> Euro
		"DE": "EUR", // Germany -> Euro
		"FR": "EUR", // France -> Euro
		"IT": "EUR", // Italy -> Euro
		"ES": "EUR", // Spain -> Euro
		"AU": "AUD", // Australia -> Australian Dollar
		"CA": "CAD", // Canada -> Canadian Dollar
		"BR": "BRL", // Brazil -> Brazilian Real
		"RU": "RUB", // Russia -> Russian Ruble
	}

	_, region := locale.GetLocale()
	if currency, ok := regionToCurrency[strings.ToUpper(region)]; ok {
		return currency
	}
	return "USD" // fallback to USD
}

// newCatalog binds static vocabulary without creating any data services.
func newCatalog() *engine.Catalog {
	c := engine.NewCatalog()
	for _, code := range modules.CurrencyCodes() {
		c.AddCurrency(code, false)
	}
	for _, code := range []string{"BTC", "ETH", "USDT", "BNB"} {
		c.AddCurrency(code, true)
	}
	return c
}

// numberOptions shares Calculator's effective persisted settings without rewriting them.
func numberOptions() engine.ParseOptions {
	decimalMode := calc.DecimalSeparatorSystem
	thousandsMode := calc.ThousandsSeparatorSystem
	if instance := plugin.GetPluginManager().GetPluginInstanceById("bd723c38-f28d-4152-8621-76fd21d6456e"); instance != nil && instance.Setting != nil {
		if value, ok := instance.Setting.Get("DecimalSeparator"); ok && value != "" {
			decimalMode = calc.DecimalSeparator(value)
		}
		if value, ok := instance.Setting.Get("ThousandsSeparator"); ok && value != "" {
			thousandsMode = calc.ThousandsSeparator(value)
		}
	}
	decimal := calc.GetDecimalSeparator(decimalMode)
	thousands := calc.GetThousandsSeparator(thousandsMode, decimal)
	if thousands == decimal {
		thousands = ""
	}
	return engine.ParseOptions{DecimalSeparator: decimal, ThousandsSeparator: thousands}
}

func (c *Converter) Query(ctx context.Context, query plugin.Query) plugin.QueryResponse {
	if strings.TrimSpace(query.Search) == "" {
		return plugin.QueryResponse{}
	}
	options := numberOptions()
	parsed, err := c.catalog.Parse(query.Search, options)
	if err != nil || !parsed.Domain {
		return plugin.QueryResponse{}
	}
	if parsed.Crypto && c.api.GetSetting(ctx, cryptoPriceSyncConsentSettingKey) != "true" {
		return plugin.NewQueryResponse([]plugin.QueryResult{c.buildCryptoConsentResult()})
	}
	prices := map[string]*big.Rat{}
	var updated int64
	if parsed.Money && c.currencyModule != nil {
		prices, updated = c.currencyModule.Snapshot()
	}
	if parsed.Crypto && c.cryptoModule != nil {
		for code, price := range c.cryptoModule.Snapshot() {
			prices[code] = price
		}
	}
	evaluation, err := c.catalog.Evaluate(ctx, parsed, engine.Env{Now: time.Now(), Local: time.Local, DefaultCurrency: GetUserDefaultCurrency(), Prices: prices, RateUpdatedAt: updated})
	if err != nil {
		return plugin.QueryResponse{}
	}
	presentation := c.catalog.Format(evaluation, engine.FormatOptions{DecimalSeparator: options.DecimalSeparator, ThousandsSeparator: options.ThousandsSeparator, Translate: func(key string) string { return c.api.GetTranslation(ctx, key) }})
	actions := []plugin.QueryResultAction{}
	for _, item := range []struct{ key, text string }{{"plugin_converter_copy_result", presentation.Formatted}, {"plugin_converter_copy_raw", presentation.Raw}, {"plugin_converter_copy_question_answer", presentation.Expression + " = " + presentation.Formatted}} {
		text := item.text
		actions = append(actions, plugin.QueryResultAction{Name: "i18n:" + item.key, Icon: icons.Get(icons.ActionCopy), ContextData: common.ContextData{"query": query.Search}, Action: func(context.Context, plugin.ActionContext) { clipboard.WriteText(text) }})
	}
	return plugin.QueryResponse{AutoRecordQueryHistory: true, Results: []plugin.QueryResult{{Title: presentation.Formatted, SubTitle: presentation.SubTitle, Icon: icons.Get(icons.PluginConverter), Tails: c.buildResultTails(ctx, presentation), Actions: actions}}}
}

// buildCryptoConsentResult gates the first network access behind an explicit action.
func (c *Converter) buildCryptoConsentResult() plugin.QueryResult {
	return plugin.QueryResult{
		Id:       cryptoConsentResultID,
		Title:    "i18n:plugin_converter_crypto_consent_title",
		SubTitle: "i18n:plugin_converter_crypto_consent_subtitle",
		Icon:     icons.Get(icons.PluginConverter),
		Actions: []plugin.QueryResultAction{
			{
				Id:                     cryptoConsentActionID,
				Name:                   "i18n:plugin_converter_crypto_consent_allow",
				Icon:                   icons.Get(icons.ActionRun),
				IsDefault:              true,
				PreventHideAfterAction: true,
				Action: func(actionCtx context.Context, actionContext plugin.ActionContext) {
					saveResult := c.api.SetSetting(actionCtx, plugin.SetSettingOption{
						Key:     cryptoPriceSyncConsentSettingKey,
						Value:   "true",
						IsLocal: true,
					})
					if !saveResult.Success {
						c.api.Notify(actionCtx, saveResult.ErrMsg)
						return
					}

					if updatable := c.api.GetUpdatableResult(actionCtx, actionContext.ResultId); updatable != nil {
						loadingTitle := c.api.GetTranslation(actionCtx, "plugin_converter_crypto_loading")
						emptyActions := []plugin.QueryResultAction{}
						updatable.Title = &loadingTitle
						updatable.Actions = &emptyActions
						c.api.UpdateResult(actionCtx, *updatable)
					}

					c.cryptoModule.StartPriceSyncSchedule(c.lifecycleCtx, func() {
						if c.api.GetUpdatableResult(c.lifecycleCtx, actionContext.ResultId) != nil {
							c.api.RefreshQuery(c.lifecycleCtx, plugin.RefreshQueryParam{PreserveSelectedIndex: true})
						}
					})
				},
			},
		},
	}
}

// buildResultTails uses the timestamp captured with the prices that produced this result.
func (c *Converter) buildResultTails(ctx context.Context, result engine.Presentation) []plugin.QueryResultTail {
	if result.TimeZone != "" {
		return []plugin.QueryResultTail{plugin.NewQueryResultTailText(result.TimeZone)}
	}
	if !result.Currency {
		return nil
	}
	if result.RateUpdatedAt == 0 {
		return []plugin.QueryResultTail{plugin.NewQueryResultTailText(c.api.GetTranslation(ctx, "plugin_converter_rates_fallback"))}
	}
	return []plugin.QueryResultTail{plugin.NewQueryResultTailText(fmt.Sprintf(c.api.GetTranslation(ctx, "plugin_converter_rates_updated"), c.formatCurrencyRateUpdatedAgo(ctx, result.RateUpdatedAt)))}
}

func (c *Converter) formatCurrencyRateUpdatedAgo(ctx context.Context, updatedAt int64) string {
	// Keep this formatter local to converter currency tails. Reusing or changing
	// the global timestamp formatter would accidentally alter unrelated event
	// dates in history, screenshots, backups, and other plugins.
	elapsedSeconds := (util.GetSystemTimestamp() - updatedAt) / 1000
	if elapsedSeconds < 0 {
		elapsedSeconds = 0
	}

	if elapsedSeconds < 60 {
		return fmt.Sprintf(c.api.GetTranslation(ctx, "plugin_converter_rates_updated_seconds_ago"), elapsedSeconds)
	}

	elapsedMinutes := elapsedSeconds / 60
	if elapsedMinutes < 60 {
		return fmt.Sprintf(c.api.GetTranslation(ctx, "plugin_converter_rates_updated_minutes_ago"), elapsedMinutes)
	}

	elapsedHours := elapsedMinutes / 60
	return fmt.Sprintf(c.api.GetTranslation(ctx, "plugin_converter_rates_updated_hours_ago"), elapsedHours)
}

func (c *Converter) handleMRURestore(ctx context.Context, mruData plugin.MRUData) (*plugin.QueryResult, error) {
	query := mruData.ContextData["query"]
	if query == "" {
		return nil, fmt.Errorf("empty converter query in context data")
	}

	// Query now returns a QueryResponse so query-scoped metadata can travel with
	// results. MRU restore only needs the first restored row, so unwrap Results
	// here instead of treating the response object like the old result slice.
	response := c.Query(context.Background(), plugin.Query{
		Type:     plugin.QueryTypeInput,
		RawQuery: query,
		Search:   query,
	})
	results := response.Results
	if len(results) == 0 {
		return nil, fmt.Errorf("no result for query: %s", query)
	}

	restored := results[0]
	if restored.Id == cryptoConsentResultID {
		return &restored, nil
	}
	restored.Title = fmt.Sprintf("%s: %s", query, restored.Title)
	return &restored, nil
}
