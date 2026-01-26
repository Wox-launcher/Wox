package calculator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode"
	"wox/common"
	"wox/i18n"
	"wox/plugin"
	"wox/setting"
	"wox/setting/definition"
	"wox/util"
	"wox/util/clipboard"

	"github.com/shopspring/decimal"
)

var calculatorIcon = common.PluginCalculatorIcon

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &CalculatorPlugin{})
}

type CalculatorHistory struct {
	Expression string
	Result     string
	AddDate    string
}

type CalculatorPlugin struct {
	api              plugin.API
	histories        []CalculatorHistory
	lastQueryText    string
	debounceTimer    *time.Timer
	debounceInterval time.Duration
}

func (c *CalculatorPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            "bd723c38-f28d-4152-8621-76fd21d6456e",
		Name:          "i18n:plugin_calculator_plugin_name",
		Author:        "Wox Launcher",
		Website:       "https://github.com/Wox-launcher/Wox",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Go",
		Description:   "i18n:plugin_calculator_plugin_description",
		Icon:          calculatorIcon.String(),
		Entry:         "",
		TriggerKeywords: []string{
			"*",
			"calculator",
		},
		Commands: []plugin.MetadataCommand{},
		SupportedOS: []string{
			"Macos",
			"Linux",
		},
		SettingDefinitions: []definition.PluginSettingDefinitionItem{
			{
				Type: definition.PluginSettingDefinitionTypeSelect,
				Value: &definition.PluginSettingValueSelect{
					Key:          "ThousandsSeparator",
					Label:        "i18n:plugin_calculator_thousands_separator",
					DefaultValue: "System Locale",
					Options: []definition.PluginSettingValueSelectOption{
						{Label: "i18n:plugin_calculator_thousands_separator_system_locale", Value: "System Locale"},
						{Label: "i18n:plugin_calculator_thousands_separator_comma", Value: "Comma"},
						{Label: "i18n:plugin_calculator_thousands_separator_dot", Value: "Dot"},
						{Label: "i18n:plugin_calculator_thousands_separator_space", Value: "Space"},
						{Label: "i18n:plugin_calculator_thousands_separator_apostrophe", Value: "Apostrophe"},
						{Label: "i18n:plugin_calculator_thousands_separator_none", Value: "None"},
					},
					Tooltip: "i18n:plugin_calculator_thousands_separator_description",
					Style: definition.PluginSettingValueStyle{
						LabelWidth:    140,
						Width:         220,
						PaddingBottom: 10,
						I18nOverrideMap: map[i18n.LangCode]definition.PluginSettingValueStyle{
							i18n.LangCodeZhCn: {
								LabelWidth:    80,
								Width:         220,
								PaddingBottom: 10,
							},
						},
					},
				},
			},
			{
				Type: definition.PluginSettingDefinitionTypeSelect,
				Value: &definition.PluginSettingValueSelect{
					Key:          "DecimalSeparator",
					Label:        "i18n:plugin_calculator_decimal_separator",
					DefaultValue: "System Locale",
					Options: []definition.PluginSettingValueSelectOption{
						{Label: "i18n:plugin_calculator_decimal_separator_system_locale", Value: "System Locale"},
						{Label: "i18n:plugin_calculator_decimal_separator_dot", Value: "Dot"},
						{Label: "i18n:plugin_calculator_decimal_separator_comma", Value: "Comma"},
					},
					Tooltip: "i18n:plugin_calculator_decimal_separator_description",
					Style: definition.PluginSettingValueStyle{
						LabelWidth:    140,
						Width:         220,
						PaddingBottom: 10,
						I18nOverrideMap: map[i18n.LangCode]definition.PluginSettingValueStyle{
							i18n.LangCodeZhCn: {
								LabelWidth:    80,
								Width:         220,
								PaddingBottom: 10,
							},
						},
					},
				},
			},
			{
				Type:  definition.PluginSettingDefinitionTypeNewLine,
				Value: &definition.PluginSettingValueNewLine{},
			},
			{
				Type: definition.PluginSettingDefinitionTypeDynamic,
				Value: &definition.PluginSettingValueDynamic{
					Key: "SeparatorPreview",
				},
			},
		},
	}
}

func (c *CalculatorPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	c.api = initParams.API
	c.api.OnGetDynamicSetting(ctx, func(ctx context.Context, key string) definition.PluginSettingDefinitionItem {
		if key == "SeparatorPreview" {
			thousandsSep, decimalSep := c.getSeparators(ctx)
			sampleNumber := "1234567.89"
			if thousandsSep != "" {
				sampleNumber = c.addThousandsSeparator("1234567", thousandsSep) + decimalSep + "89"
			} else {
				sampleNumber = "1234567" + decimalSep + "89"
			}

			return definition.PluginSettingDefinitionItem{
				Type: definition.PluginSettingDefinitionTypeLabel,
				Value: &definition.PluginSettingValueLabel{
					Content: c.api.GetTranslation(ctx, "plugin_calculator_separator_preview_example") + ": " + sampleNumber,
				},
			}
		}

		return definition.PluginSettingDefinitionItem{}
	})

	c.debounceInterval = 500 * time.Millisecond // 500ms debounce interval
	c.histories = c.loadHistories(ctx)
}

func (c *CalculatorPlugin) Query(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	var results []plugin.QueryResult

	if query.TriggerKeyword == "" {
		if !c.hasOperator(query.Search) {
			return []plugin.QueryResult{}
		}

		// Try to calculate the expression, if it fails then it's not a valid calculator expression
		thousandsSep, decimalSep := c.getSeparators(ctx)
		val, err := Calculate(query.Search, thousandsSep, decimalSep)
		if err != nil {
			c.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("Calculator failed to parse expression: %v", err))
			return []plugin.QueryResult{}
		}
		result := val.String()
		formattedResult := c.formatWithThousandsSeparator(ctx, val)

		// Add to query history with debounce when calculation is successful
		c.addQueryHistoryDebounced(ctx, query.Search, result)

		results = append(results, plugin.QueryResult{
			Title: formattedResult,
			Icon:  calculatorIcon,
			Actions: []plugin.QueryResultAction{
				{
					Name: "i18n:plugin_calculator_copy_result",
					Icon: common.CopyIcon,
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						c.histories = append(c.histories, CalculatorHistory{
							Expression: query.Search,
							Result:     result,
							AddDate:    util.FormatDateTime(util.GetSystemTime()),
						})
						clipboard.WriteText(result)
					},
				},
				{
					Name:      "i18n:plugin_calculator_copy_result_with_thousands_separator",
					IsDefault: true,
					Icon:      common.CopyIcon,
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						c.histories = append(c.histories, CalculatorHistory{
							Expression: query.Search,
							Result:     result,
							AddDate:    util.FormatDateTime(util.GetSystemTime()),
						})
						clipboard.WriteText(formattedResult)
					},
				},
			},
		})
	}

	// only show history if query has trigger keyword
	if query.TriggerKeyword != "" {
		thousandsSep, decimalSep := c.getSeparators(ctx)
		val, err := Calculate(query.Search, thousandsSep, decimalSep)
		if err == nil {
			result := val.String()
			formattedResult := c.formatWithThousandsSeparator(ctx, val)

			// Add to query history with debounce when calculation is successful
			c.addQueryHistoryDebounced(ctx, query.Search, result)

			results = append(results, plugin.QueryResult{
				Title: formattedResult,
				Icon:  calculatorIcon,
				Actions: []plugin.QueryResultAction{
					{
						Name: "i18n:plugin_calculator_copy_result",
						Icon: common.CopyIcon,
						Action: func(ctx context.Context, actionContext plugin.ActionContext) {
							clipboard.WriteText(result)
						},
					},
					{
						Name:      "i18n:plugin_calculator_copy_result_with_thousands_separator",
						IsDefault: true,
						Icon:      common.CopyIcon,
						Action: func(ctx context.Context, actionContext plugin.ActionContext) {
							clipboard.WriteText(formattedResult)
						},
					},
				},
			})
		}

		//show top 500 histories order by desc
		var count = 0
		for i := len(c.histories) - 1; i >= 0; i-- {
			h := c.histories[i]

			count++
			if count >= 500 {
				break
			}

			if strings.Contains(h.Expression, query.Search) || strings.Contains(h.Result, query.Search) {
				// Try to parse history result to format with thousands separator
				historyVal, parseErr := decimal.NewFromString(h.Result)
				formattedHistoryResult := h.Result
				if parseErr == nil {
					formattedHistoryResult = c.formatWithThousandsSeparator(ctx, historyVal)
				}

				results = append(results, plugin.QueryResult{
					Title:    h.Expression,
					SubTitle: formattedHistoryResult,
					Icon:     calculatorIcon,
					Actions: []plugin.QueryResultAction{
						{
							Name: "i18n:plugin_calculator_copy_result",
							Icon: common.CopyIcon,
							Action: func(ctx context.Context, actionContext plugin.ActionContext) {
								clipboard.WriteText(h.Result)
							},
						},
						{
							Name:      "i18n:plugin_calculator_copy_result_with_thousands_separator",
							IsDefault: true,
							Icon:      common.CopyIcon,
							Action: func(ctx context.Context, actionContext plugin.ActionContext) {
								clipboard.WriteText(formattedHistoryResult)
							},
						},
						{
							Name: "i18n:plugin_calculator_recalculate",
							Action: func(ctx context.Context, actionContext plugin.ActionContext) {
								c.api.ChangeQuery(ctx, common.PlainQuery{
									QueryType: plugin.QueryTypeInput,
									QueryText: h.Expression,
								})
							},
						},
					},
				})
			}
		}

		if len(results) == 0 {
			results = append(results, plugin.QueryResult{
				Title:   "i18n:plugin_calculator_input_expression",
				Icon:    calculatorIcon,
				Actions: []plugin.QueryResultAction{},
			})
		}
	}

	return results
}

func (c *CalculatorPlugin) getDecimalSeparator(ctx context.Context) DecimalSeparator {
	mode := c.api.GetSetting(ctx, "DecimalSeparator")
	if mode == "" {
		return DecimalSeparatorSystem
	}
	return DecimalSeparator(mode)
}

func (c *CalculatorPlugin) getThousandsSeparator(ctx context.Context) ThousandsSeparator {
	mode := c.api.GetSetting(ctx, "ThousandsSeparator")
	if mode == "" {
		return ThousandsSeparatorSystem
	}
	return ThousandsSeparator(mode)
}

func (c *CalculatorPlugin) getSeparators(ctx context.Context) (thousandsSep string, decimalSep string) {
	decimalSep = GetDecimalSeparator(c.getDecimalSeparator(ctx))
	thousandsSep = GetThousandsSeparator(c.getThousandsSeparator(ctx), decimalSep)
	if decimalSep != "" && decimalSep == thousandsSep {
		thousandsSep = ""
	}

	return thousandsSep, decimalSep
}

func (c *CalculatorPlugin) formatWithThousandsSeparator(ctx context.Context, val decimal.Decimal) string {
	valStr := val.String()
	parts := strings.Split(valStr, ".")

	thousandsSep, decimalSep := c.getSeparators(ctx)

	intPart := parts[0]
	// Handle negative numbers
	negative := false
	if strings.HasPrefix(intPart, "-") {
		negative = true
		intPart = intPart[1:]
	}

	// Add thousands separators
	formattedInt := c.addThousandsSeparator(intPart, thousandsSep)

	if negative {
		formattedInt = "-" + formattedInt
	}

	if len(parts) == 2 {
		return formattedInt + decimalSep + parts[1]
	}

	return formattedInt
}

// addThousandsSeparator adds separators to an integer string
func (c *CalculatorPlugin) addThousandsSeparator(s string, sep string) string {
	n := len(s)
	if n <= 3 {
		return s
	}

	// Calculate how many separators we need
	numSeps := (n - 1) / 3
	// sep can be multi-byte? usually just 1 byte but string is safer
	sepLen := len(sep)

	result := make([]byte, n+numSeps*sepLen)

	// Fill from right to left
	j := len(result) - 1
	for i := n - 1; i >= 0; i-- {
		result[j] = s[i]
		j--
		// Add separator every 3 digits, but not at the beginning
		if (n-i)%3 == 0 && i > 0 {
			for k := sepLen - 1; k >= 0; k-- {
				result[j] = sep[k]
				j--
			}
		}
	}

	return string(result)
}

func (c *CalculatorPlugin) hasOperator(query string) bool {
	if strings.ContainsAny(query, "+-*/(^") {
		return true
	}

	if !strings.Contains(query, "(") || !strings.Contains(query, ")") {
		return false
	}

	for _, r := range query {
		if unicode.IsLetter(r) {
			return true
		}
	}
	return false
}

func (c *CalculatorPlugin) loadHistories(ctx context.Context) []CalculatorHistory {
	historiesJson := c.api.GetSetting(ctx, "calculatorHistories")
	if historiesJson == "" {
		return []CalculatorHistory{}
	}

	var histories []CalculatorHistory
	err := json.Unmarshal([]byte(historiesJson), &histories)
	if err != nil {
		c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to unmarshal calculator history: %s", err.Error()))
		return []CalculatorHistory{}
	}

	c.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("Calculator history loaded: %d", len(histories)))

	return histories
}

// addQueryHistoryDebounced adds query to history with debounce mechanism
// Only records the last valid calculation when user stops typing
func (c *CalculatorPlugin) addQueryHistoryDebounced(ctx context.Context, queryText string, result string) {
	if !c.hasOperator(queryText) {
		return
	}

	// Cancel existing timer if any
	if c.debounceTimer != nil {
		c.debounceTimer.Stop()
	}

	// Store the current query text
	c.lastQueryText = queryText

	// Create new timer that will execute after debounce interval
	c.debounceTimer = time.AfterFunc(c.debounceInterval, func() {
		// Only add to history if this is still the latest query
		if c.lastQueryText == queryText {
			// Check if this query is the same as the last query in global history
			settingManager := setting.GetSettingManager()
			latestHistories := settingManager.GetLatestQueryHistory(ctx, 1)

			// Skip if the query is the same as the most recent one
			if len(latestHistories) > 0 && latestHistories[0].Query.QueryText == queryText {
				c.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("Calculator query skipped (duplicate): %s", queryText))
				return
			}

			plainQuery := common.PlainQuery{
				QueryType: plugin.QueryTypeInput,
				QueryText: queryText,
			}
			settingManager.AddQueryHistory(ctx, plainQuery)

			c.histories = append(c.histories, CalculatorHistory{
				Expression: queryText,
				Result:     result,
				AddDate:    util.FormatDateTime(util.GetSystemTime()),
			})

			historiesJson, err := json.Marshal(c.histories)
			if err != nil {
				c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to marshal calculator history: %s", err.Error()))
			} else {
				c.api.SaveSetting(ctx, "calculatorHistories", string(historiesJson), false)
			}

			c.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("Calculator query added to history: %s", queryText))
		}
	})
}
