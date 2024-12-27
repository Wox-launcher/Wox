package calculator

import (
	"context"
	"fmt"
	"strings"
	"wox/plugin"
	"wox/plugin/system/calculator/core"
	"wox/plugin/system/calculator/modules"
	"wox/util/clipboard"

	"github.com/samber/lo"
)

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &Calculator{})
}

type Calculator struct {
	api       plugin.API
	registry  *core.ModuleRegistry
	tokenizer *core.Tokenizer
}

func (c *Calculator) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            "a48dc5f0-dab9-4112-b883-b68129d6782b",
		Name:          "Calculator",
		Author:        "Wox Launcher",
		Website:       "https://github.com/Wox-launcher/Wox",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Go",
		Description:   "Calculator for Wox",
		Icon:          plugin.PluginCalculatorIcon.String(),
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

func (c *Calculator) Init(ctx context.Context, initParams plugin.InitParams) {
	c.api = initParams.API

	registry := core.NewModuleRegistry()
	registry.Register(modules.NewMathModule(ctx, c.api))
	registry.Register(modules.NewTimeModule(ctx, c.api))

	currencyModule := modules.NewCurrencyModule(ctx, c.api)
	currencyModule.StartExchangeRateSyncSchedule(ctx)
	registry.Register(currencyModule)

	tokenizer := core.NewTokenizer(registry.GetTokenPatterns())
	c.registry = registry
	c.tokenizer = tokenizer
}

// parseExpression parses a complex expression like "1btc + 100usd"
// It returns a slice of tokens grouped by their module
func (c *Calculator) parseExpression(ctx context.Context, tokens []core.Token) ([]*core.Result, []string, error) {
	values := make([]*core.Result, 0)
	operators := make([]string, 0)

	currentTokens := make([]core.Token, 0)

	// First try math module for the entire expression
	// because +-/* are supported by math module, which will be used for mixed unit expression
	mathModule := c.registry.GetModule("math")
	if mathModule != nil && mathModule.CanHandle(ctx, tokens) {
		value, err := mathModule.Parse(ctx, tokens)
		if err == nil {
			values = append(values, value)
			return values, operators, nil
		}
	}

	// If math module can't handle it, try parsing as mixed unit expression
	for i := 0; i < len(tokens); i++ {
		t := tokens[i]

		if t.Kind == core.ReservedToken && (t.Str == "+" || t.Str == "-") {
			// Found an operator, parse the current tokens
			if len(currentTokens) > 0 {
				// Try to find a module that can handle these tokens
				for _, module := range c.registry.Modules() {
					if module.CanHandle(ctx, currentTokens) {
						value, err := module.Parse(ctx, currentTokens)
						if err != nil {
							continue
						}
						values = append(values, value)
						operators = append(operators, t.Str)
						currentTokens = make([]core.Token, 0)
						break
					}
				}
			}
		} else {
			currentTokens = append(currentTokens, t)
		}
	}

	// Handle the last group of tokens
	if len(currentTokens) > 0 {
		for _, module := range c.registry.Modules() {
			if module.CanHandle(ctx, currentTokens) {
				value, err := module.Parse(ctx, currentTokens)
				if err != nil {
					continue
				}
				values = append(values, value)
				break
			}
		}
	}

	return values, operators, nil
}

// calculateMixedUnits calculates expressions with mixed units
// For example: "1btc + 100usd" will convert everything to USD and then calculate
func (c *Calculator) calculateMixedUnits(ctx context.Context, values []*core.Result, operators []string) (*core.Result, error) {
	if len(values) == 0 {
		return nil, fmt.Errorf("no values to calculate")
	}

	// If there are no operators, just return the first value as is
	if len(operators) == 0 {
		return values[0], nil
	}

	// Convert all values to the first value's unit
	targetUnit := values[0].Unit
	if targetUnit == "" || values[0].RawValue == nil {
		return nil, fmt.Errorf("first value must have a unit and raw value")
	}

	result := values[0].RawValue
	unit := values[0].Unit

	for i := 0; i < len(operators); i++ {
		// Convert the next value to the target unit
		if values[i+1].Unit == "" || values[i+1].RawValue == nil {
			return nil, fmt.Errorf("value must have a unit and raw value")
		}

		convertedValue, err := c.registry.Convert(ctx, values[i+1], targetUnit)
		if err != nil {
			return nil, err
		}

		// Perform the calculation
		switch operators[i] {
		case "+":
			val := result.Add(*convertedValue.RawValue)
			result = &val
			unit = convertedValue.Unit
		case "-":
			val := result.Sub(*convertedValue.RawValue)
			result = &val
			unit = convertedValue.Unit
		}
	}

	return &core.Result{
		DisplayValue: fmt.Sprintf("%s %s", result.String(), unit),
		RawValue:     result,
		Unit:         unit,
	}, nil
}

func (c *Calculator) Query(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	if query.Search == "" {
		return []plugin.QueryResult{}
	}

	tokens, err := c.tokenizer.Tokenize(ctx, query.Search)
	if err != nil {
		c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Tokenize error: %v", err))
		return []plugin.QueryResult{}
	}
	c.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("Tokens: %+v", tokens))

	// Try to parse as an expression (could be a simple math expression or a mixed unit expression)
	values, operators, err := c.parseExpression(ctx, tokens)
	if err != nil {
		c.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("Parse expression error: %v", err))
		return []plugin.QueryResult{}
	}

	if len(values) == 0 {
		c.api.Log(ctx, plugin.LogLevelDebug, "No values parsed from expression")
		return []plugin.QueryResult{}
	}

	c.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("Expression parsed: values=[%s], operators=[%s]",
		lo.Map(values, func(v *core.Result, _ int) string { return v.DisplayValue }),
		strings.Join(operators, "")))

	// Calculate the result (handles both simple and mixed unit expressions)
	result, err := c.calculateMixedUnits(ctx, values, operators)
	if err != nil {
		c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Calculation error: %v", err))
		return []plugin.QueryResult{}
	}

	return []plugin.QueryResult{
		{
			Title: result.DisplayValue,
			Icon:  plugin.PluginCalculatorIcon,
			Actions: []plugin.QueryResultAction{
				{
					Name: "i18n:plugin_calculator_copy_result",
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						clipboard.WriteText(result.DisplayValue)
					},
				},
			},
		},
	}
}
