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

		// Add to query history with debounce when calculation is successful
		c.addQueryHistoryDebounced(ctx, query.Search, result)

		results = append(results, plugin.QueryResult{
			Title: result,
			Icon:  calculatorIcon,
			Actions: []plugin.QueryResultAction{
				{
					Name: "i18n:plugin_calculator_copy_result",
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						c.histories = append(c.histories, CalculatorHistory{
							Expression: query.Search,
							Result:     result,
							AddDate:    util.FormatDateTime(util.GetSystemTime()),
						})
						clipboard.WriteText(result)
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

			// Add to query history with debounce when calculation is successful
			c.addQueryHistoryDebounced(ctx, query.Search, result)

			results = append(results, plugin.QueryResult{
				Title: result,
				Icon:  calculatorIcon,
				Actions: []plugin.QueryResultAction{
					{
						Name: "i18n:plugin_calculator_copy_result",
						Action: func(ctx context.Context, actionContext plugin.ActionContext) {

							clipboard.WriteText(result)
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
				results = append(results, plugin.QueryResult{
					Title:    h.Expression,
					SubTitle: h.Result,
					Icon:     calculatorIcon,
					Actions: []plugin.QueryResultAction{
						{
							Name:      "i18n:plugin_calculator_copy_result",
							IsDefault: true,
							Action: func(ctx context.Context, actionContext plugin.ActionContext) {
								clipboard.WriteText(h.Result)
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
