package calculator

import (
	"context"
	"fmt"
	"wox/plugin"
	"wox/plugin/system/calculator/core"
	"wox/plugin/system/calculator/modules"
	"wox/util/clipboard"
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
		Id:            "system.calculator",
		Name:          "Calculator",
		Author:        "Wox Launcher",
		Website:       "https://github.com/Wox-launcher/Wox",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Go",
		Description:   "Calculator for Wox",
		Icon:          "calculator.png",
		Entry:         "",
		TriggerKeywords: []string{
			"*",
			"calculator",
			"time",
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
	// Register all modules
	mathModule := modules.NewMathModule(ctx, c.api)
	registry.Register(mathModule)

	timeModule := modules.NewTimeModule(ctx, c.api)
	registry.Register(timeModule)

	// TODO: implement these modules
	//registry.Register(modules.NewCurrencyModule())
	//registry.Register(modules.NewUnitModule())
	//registry.Register(modules.NewCryptoModule())

	// Create tokenizer with all patterns from registered modules
	tokenizer := core.NewTokenizer(registry.GetTokenPatterns())

	c.registry = registry
	c.tokenizer = tokenizer
}

// parseExpression parses a complex expression like "1btc + 100usd"
// It returns a slice of tokens grouped by their module
func (c *Calculator) parseExpression(ctx context.Context, tokens []core.Token) ([]*core.Value, []string, error) {
	values := make([]*core.Value, 0)
	operators := make([]string, 0)

	currentTokens := make([]core.Token, 0)

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
func (c *Calculator) calculateMixedUnits(ctx context.Context, values []*core.Value, operators []string) (*core.Value, error) {
	if len(values) == 0 {
		return nil, fmt.Errorf("no values to calculate")
	}

	// Convert all values to USD (or the unit of the first value)
	targetUnit := values[0].Unit
	result := values[0].Amount

	for i := 0; i < len(operators); i++ {
		// Convert the next value to the target unit
		convertedValue, err := c.registry.Convert(ctx, values[i+1], targetUnit)
		if err != nil {
			return nil, err
		}

		// Perform the calculation
		switch operators[i] {
		case "+":
			result = result.Add(convertedValue.Amount)
		case "-":
			result = result.Sub(convertedValue.Amount)
		}
	}

	return &core.Value{Amount: result, Unit: targetUnit}, nil
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

	// Try to parse as a mixed unit expression
	values, operators, err := c.parseExpression(ctx, tokens)
	if err == nil && len(values) > 0 {
		c.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("Mixed unit expression: values=%+v, operators=%+v", values, operators))
		result, err := c.calculateMixedUnits(ctx, values, operators)
		if err == nil {
			return []plugin.QueryResult{
				{
					Title:    fmt.Sprintf("%s = %s", query.Search, result.Amount.String()),
					SubTitle: fmt.Sprintf("Copy %s to clipboard", result.Amount.String()),
					Icon:     plugin.PluginCalculatorIcon,
					Actions: []plugin.QueryResultAction{
						{
							Name: "i18n:plugin_calculator_copy_result",
							Action: func(ctx context.Context, actionContext plugin.ActionContext) {
								clipboard.WriteText(result.Amount.String())
							},
						},
					},
				},
			}
		} else {
			c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Mixed unit calculation error: %v", err))
		}
	} else if err != nil {
		c.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("Parse expression error: %v", err))
	}

	// If mixed unit calculation fails, try to find a single module to handle it
	for _, module := range c.registry.Modules() {
		c.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("Trying module: %s", module.Name()))
		if module.CanHandle(ctx, tokens) {
			c.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("Module %s can handle tokens", module.Name()))
			result, err := module.Calculate(ctx, tokens)
			if err != nil {
				c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Calculate error from module %s: %v", module.Name(), err))
				continue
			}
			return []plugin.QueryResult{
				{
					Title:    fmt.Sprintf("%s = %s", query.Search, result.Amount.String()),
					SubTitle: fmt.Sprintf("Copy %s to clipboard", result.Amount.String()),
					Icon:     plugin.PluginCalculatorIcon,
					Actions: []plugin.QueryResultAction{
						{
							Name: "i18n:plugin_calculator_copy_result",
							Action: func(ctx context.Context, actionContext plugin.ActionContext) {
								clipboard.WriteText(result.Amount.String())
							},
						},
					},
				},
			}
		} else {
			c.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("Module %s cannot handle tokens", module.Name()))
		}
	}

	c.api.Log(ctx, plugin.LogLevelDebug, "No module can handle the expression")
	return []plugin.QueryResult{}
}
