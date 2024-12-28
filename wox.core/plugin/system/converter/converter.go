package converter

import (
	"context"
	"fmt"
	"strings"
	"wox/plugin"
	"wox/plugin/system/converter/core"
	"wox/plugin/system/converter/modules"
	"wox/util/clipboard"

	"github.com/samber/lo"
)

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &Converter{})
}

type Converter struct {
	api       plugin.API
	registry  *core.ModuleRegistry
	tokenizer *core.Tokenizer
}

func (c *Converter) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            "a48dc5f0-dab9-4112-b883-b68129d6782b",
		Name:          "Converter",
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

func (c *Converter) Init(ctx context.Context, initParams plugin.InitParams) {
	c.api = initParams.API

	registry := core.NewModuleRegistry()
	registry.Register(modules.NewTimeModule(ctx, c.api))

	currencyModule := modules.NewCurrencyModule(ctx, c.api)
	currencyModule.StartExchangeRateSyncSchedule(ctx)
	registry.Register(currencyModule)

	cryptoModule := modules.NewCryptoModule(ctx, c.api)
	cryptoModule.StartPriceSyncSchedule(ctx)
	registry.Register(cryptoModule)

	tokenizer := core.NewTokenizer(registry.GetTokenPatterns())
	c.registry = registry
	c.tokenizer = tokenizer
}

// parseExpression parses a complex expression like "1btc + 100usd"
func (c *Converter) parseExpression(ctx context.Context, tokens []core.Token) (results []core.Result, operators []string, targetUnit core.Unit, err error) {
	for i := 0; i < len(tokens); i++ {
		token := tokens[i]
		c.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("----- %s (%s) -----", token.Str, token.Kind.String()))

		if token.Kind == core.OperationToken {
			operators = append(operators, token.Str)
			c.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("	=> operators: %s", strings.Join(operators, ", ")))
			continue
		}

		if token.Kind == core.EosToken {
			break
		}

		if token.Kind == core.ConversionToken {
			// Try to find a module that can handle this token
			var result core.Result
			var err error
			for _, module := range c.registry.Modules() {
				result, err = module.Calculate(ctx, token)
				if err == nil {
					targetUnit = result.Unit
					break
				}
			}
			if targetUnit.Name == "" {
				c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to parse target unit from token %s", token.Str))
				return nil, nil, core.Unit{}, fmt.Errorf("failed to parse target unit: %s", token.Str)
			} else {
				c.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("	=> target unit: %s", targetUnit.Name))
			}

			continue
		}

		// Try to find a module that can handle this token
		var result core.Result
		var err error
		var moduleFound bool
		for _, module := range c.registry.Modules() {
			result, err = module.Calculate(ctx, token)
			if err == nil {
				moduleFound = true
				results = append(results, result)
				break
			}
		}

		if !moduleFound {
			c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to calculate token %s: no module can handle it", token.Str))
			return nil, nil, core.Unit{}, fmt.Errorf("no module can handle token: %s", token.Str)
		}
	}

	// If we have a target unit, convert all values to that unit
	if targetUnit.Name != "" {
		for i := range results {
			if results[i].Unit.Type == targetUnit.Type {
				convertedValue, err := results[i].Module.Convert(ctx, results[i], targetUnit)
				if err != nil {
					return nil, nil, core.Unit{}, err
				}
				results[i] = convertedValue
			}
		}
	}

	return results, operators, targetUnit, nil
}

// calculateExpression calculates expressions with mixed units
// For example: "1btc + 100usd" will convert everything to USD and then calculate
func (c *Converter) calculateExpression(ctx context.Context, results []core.Result, operators []string, targetUnit core.Unit) (core.Result, error) {
	// check if operators count is equal to results count - 1
	if len(operators) != len(results)-1 {
		return core.Result{}, fmt.Errorf("invalid expression: operators count (%d) does not match results count (%d) - 1", len(operators), len(results))
	}

	// If there are no operators and only one value, just return it as is
	if len(operators) == 0 && len(results) == 1 {
		if targetUnit.Name == "" {
			c.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("No operators, No target unit, returning the only result: %s", results[0].DisplayValue))
			return results[0], nil
		} else {
			c.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("No operators, target unit is set, converting the only result: %s to %s", results[0].DisplayValue, targetUnit.Name))
			return results[0].Module.Convert(ctx, results[0], targetUnit)
		}
	}

	if targetUnit.Name == "" {
		// if all results are time units, the target unit should be time
		allResultsAreTime := lo.EveryBy(results, func(r core.Result) bool { return r.Unit.Type == core.UnitTypeTime })
		allResultsAreCurrencyOrCrypto := lo.EveryBy(results, func(r core.Result) bool {
			return r.Unit.Type == core.UnitTypeCurrency || r.Unit.Type == core.UnitTypeCrypto
		})

		if allResultsAreTime {
			// use last timezone as the target unit
			for _, result := range results {
				if result.Unit.Type == core.UnitTypeTime {
					targetUnit = result.Unit
					break
				}
			}

			targetUnit = core.Unit{Name: results[len(results)-1].Unit.Name, Type: core.UnitTypeTime}
		} else if allResultsAreCurrencyOrCrypto {
			targetUnit = core.UnitUSD
		} else {
			c.api.Log(ctx, plugin.LogLevelDebug, "No target unit, using USD as default")
			targetUnit = core.UnitUSD
		}
	}

	// Convert all values to USD for currency and crypto
	for i := range results {
		if results[i].Unit.Type == core.UnitTypeCurrency || results[i].Unit.Type == core.UnitTypeCrypto {
			var err error
			results[i], err = results[i].Module.Convert(ctx, results[i], core.UnitUSD)
			if err != nil {
				c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to convert %s to USD: %v", results[i].DisplayValue, err))
				return core.Result{}, err
			} else {
				c.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("Converted %s to USD => %s", results[i].DisplayValue, results[i].RawValue.String()))
			}
		}
	}

	// Calcualte the result
	result := results[0]
	for i, operator := range operators {
		nextResult := results[i+1]
		switch operator {
		case "+":
			result.RawValue = result.RawValue.Add(nextResult.RawValue)
		case "-":
			result.RawValue = result.RawValue.Sub(nextResult.RawValue)
		case "*":
			result.RawValue = result.RawValue.Mul(nextResult.RawValue)
		case "/":
			result.RawValue = result.RawValue.Div(nextResult.RawValue)
		}

		c.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("Calculated result with %s %s %s => %s", result.DisplayValue, operator, nextResult.DisplayValue, result.RawValue.String()))
	}
	result.DisplayValue = fmt.Sprintf("%s %s", result.RawValue.String(), targetUnit.Name)

	// Convert the result to the target unit
	for _, module := range c.registry.Modules() {
		if convertedResult, err := module.Convert(ctx, result, targetUnit); err == nil {
			c.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("Converted result with module %s => displayValue=%s, rawValue=%s, unit=%s", module.Name(), convertedResult.DisplayValue, convertedResult.RawValue.String(), convertedResult.Unit.Name))
			result = convertedResult
			break
		}
	}

	return result, nil
}

func (c *Converter) Query(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	if query.Search == "" {
		return []plugin.QueryResult{}
	}

	tokens, err := c.tokenizer.Tokenize(ctx, query.Search)
	if err != nil {
		c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Tokenize error: %v", err))
		return []plugin.QueryResult{}
	}

	c.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("Tokens: %s", strings.Join(lo.Map(tokens, func(t core.Token, _ int) string { return t.String() }), ", ")))

	// Try to parse as an expression (could be a simple math expression or a mixed unit expression)
	results, operators, targetUnit, err := c.parseExpression(ctx, tokens)
	if err != nil {
		c.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("Parse expression error: %v", err))
		// For invalid expressions, return a search suggestion
		return []plugin.QueryResult{}
	}

	if len(results) == 0 {
		c.api.Log(ctx, plugin.LogLevelDebug, "No values parsed from expression")
		return []plugin.QueryResult{}
	}

	c.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("Expression parsed: values=%s, operators=%s, targetUnit=%s", strings.Join(lo.Map(results, func(v core.Result, _ int) string { return v.DisplayValue }), ", "), strings.Join(operators, ", "), targetUnit.Name))

	// Calculate the result (handles both simple and mixed unit expressions)
	result, err := c.calculateExpression(ctx, results, operators, targetUnit)
	if err != nil {
		c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Calculation  expression error: %v", err))
		return []plugin.QueryResult{}
	} else {
		c.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("Calculation result: displayValue=%s, rawValue=%s, unit=%s", result.DisplayValue, result.RawValue.String(), result.Unit.Name))
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
				{
					Name: "i18n:plugin_calculator_add_to_favorite",
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						// Handle add to favorite action
					},
				},
			},
		},
	}
}
