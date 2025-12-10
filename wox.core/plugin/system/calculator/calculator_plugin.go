package calculator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"wox/common"
	"wox/plugin"
	"wox/setting"
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

// formatWithThousandsSeparator formats a decimal number with thousands separators
// e.g., 56335258.87 -> "56,335,258.87"
func formatWithThousandsSeparator(val decimal.Decimal) string {
	valStr := val.String()
	parts := strings.Split(valStr, ".")

	intPart := parts[0]
	// Handle negative numbers
	negative := false
	if strings.HasPrefix(intPart, "-") {
		negative = true
		intPart = intPart[1:]
	}

	// Add thousands separators
	formattedInt := addThousandsSeparator(intPart)

	if negative {
		formattedInt = "-" + formattedInt
	}

	if len(parts) == 2 {
		return formattedInt + "." + parts[1]
	}

	return formattedInt
}

// addThousandsSeparator adds comma separators to an integer string
// e.g., "56335258" -> "56,335,258"
func addThousandsSeparator(s string) string {
	n := len(s)
	if n <= 3 {
		return s
	}

	// Calculate how many commas we need
	numCommas := (n - 1) / 3
	result := make([]byte, n+numCommas)

	// Fill from right to left
	j := len(result) - 1
	for i := n - 1; i >= 0; i-- {
		result[j] = s[i]
		j--
		// Add comma every 3 digits, but not at the beginning
		if (n-i)%3 == 0 && i > 0 {
			result[j] = ','
			j--
		}
	}

	return string(result)
}

func (c *CalculatorPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            "bd723c38-f28d-4152-8621-76fd21d6456e",
		Name:          "Calculator",
		Author:        "Wox Launcher",
		Website:       "https://github.com/Wox-launcher/Wox",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Go",
		Description:   "Calculator for Wox",
		Icon:          calculatorIcon.String(),
		Entry:         "",
		TriggerKeywords: []string{
			"*",
			"calculator",
		},
		Commands: []plugin.MetadataCommand{},
		SupportedOS: []string{
			"Windows",
			"Macos",
			"Linux",
		},
	}
}

func (c *CalculatorPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	c.api = initParams.API
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
		val, err := Calculate(query.Search)
		if err != nil {
			c.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("Calculator failed to parse expression: %v", err))
			return []plugin.QueryResult{}
		}
		result := val.String()
		formattedResult := formatWithThousandsSeparator(val)

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
		val, err := Calculate(query.Search)
		if err == nil {
			result := val.String()
			formattedResult := formatWithThousandsSeparator(val)

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
					formattedHistoryResult = formatWithThousandsSeparator(historyVal)
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

func (c *CalculatorPlugin) hasOperator(query string) bool {
	return strings.ContainsAny(query, "+-*/(^")
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
