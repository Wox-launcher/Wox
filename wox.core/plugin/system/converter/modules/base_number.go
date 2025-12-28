package modules

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"wox/plugin"
	"wox/plugin/system/converter/core"

	"github.com/shopspring/decimal"
)

type BaseModule struct {
	*regexBaseModule
}

func NewBaseModule(ctx context.Context, api plugin.API) *BaseModule {
	m := &BaseModule{}

	const (
		hexDigits = `([0-9a-f]+)`
		octDigits = `([0-7]+)`
		binDigits = `([01]+)`
		decDigits = `([0-9]+)`
		baseUnits = `(bin|oct|dec|hex)`
	)

	handlers := []*patternHandler{
		{
			Pattern:     `(?i)(?:in|to)\s+` + baseUnits,
			Priority:    900,
			Description: "Handle base conversion keyword (e.g., to hex)",
			Handler:     m.handleBaseConversionToken,
		},
		{
			Pattern:     `(?i)=\s*\?\s*` + baseUnits,
			Priority:    800,
			Description: "Handle base conversion =? keyword (e.g., =?hex)",
			Handler:     m.handleBaseConversionToken,
		},
		{
			Pattern:     `(?i)0x` + hexDigits,
			Priority:    1000,
			Description: "Handle hex literal (e.g., 0xFF)",
			Handler:     m.handleHexPrefix,
		},
		{
			Pattern:     `(?i)0b` + binDigits,
			Priority:    1000,
			Description: "Handle binary literal (e.g., 0b1010)",
			Handler:     m.handleBinPrefix,
		},
		{
			Pattern:     `(?i)0o` + octDigits,
			Priority:    1000,
			Description: "Handle octal literal (e.g., 0o17)",
			Handler:     m.handleOctPrefix,
		},
		{
			Pattern:     `(?i)` + hexDigits + `\s*hex`,
			Priority:    950,
			Description: "Handle hex with suffix (e.g., FF hex)",
			Handler:     m.handleHexSuffix,
		},
		{
			Pattern:     `(?i)hex\s*` + hexDigits,
			Priority:    950,
			Description: "Handle hex with prefix keyword (e.g., hex FF)",
			Handler:     m.handleHexPrefixWord,
		},
		{
			Pattern:     `(?i)` + binDigits + `\s*bin`,
			Priority:    940,
			Description: "Handle binary with suffix (e.g., 1010 bin)",
			Handler:     m.handleBinSuffix,
		},
		{
			Pattern:     `(?i)bin\s*` + binDigits,
			Priority:    940,
			Description: "Handle binary with prefix keyword (e.g., bin 1010)",
			Handler:     m.handleBinPrefixWord,
		},
		{
			Pattern:     `(?i)` + octDigits + `\s*oct`,
			Priority:    930,
			Description: "Handle octal with suffix (e.g., 17 oct)",
			Handler:     m.handleOctSuffix,
		},
		{
			Pattern:     `(?i)oct\s*` + octDigits,
			Priority:    930,
			Description: "Handle octal with prefix keyword (e.g., oct 17)",
			Handler:     m.handleOctPrefixWord,
		},
		{
			Pattern:     `(?i)` + decDigits + `\s*dec`,
			Priority:    920,
			Description: "Handle decimal with suffix (e.g., 255 dec)",
			Handler:     m.handleDecSuffix,
		},
		{
			Pattern:     `(?i)dec\s*` + decDigits,
			Priority:    920,
			Description: "Handle decimal with prefix keyword (e.g., dec 255)",
			Handler:     m.handleDecPrefixWord,
		},
	}

	m.regexBaseModule = NewRegexBaseModule(api, "base", handlers)
	return m
}

func (m *BaseModule) Calculate(ctx context.Context, token core.Token) (core.Result, error) {
	if token.Kind == core.EosToken {
		return core.Result{}, fmt.Errorf("cannot calculate EOS token")
	}

	m.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("Processing in module %s", m.name))

	input := strings.TrimSpace(strings.ToLower(token.Str))
	for _, handler := range m.patternHandlers {
		if matches := handler.regexp.FindStringSubmatch(input); len(matches) > 0 && matches[0] == input {
			result, err := handler.Handler(ctx, matches)
			if err != nil {
				m.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("	=> Pattern '%s': %v", handler.Description, err))
				continue
			}

			m.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("	=> Pattern '%s' matched: %v", handler.Description, strings.Join(matches[1:], ", ")))
			m.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("	=> matched result: value=%s, raw=%s, unit=%s", result.DisplayValue, result.RawValue, result.Unit.Name))
			return result, nil
		}
	}

	m.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("	=> No matching patterns found for token '%s'", token.Str))
	return core.Result{}, fmt.Errorf("unsupported format")
}

func (m *BaseModule) Convert(ctx context.Context, value core.Result, toUnit core.Unit) (core.Result, error) {
	if value.Unit.Type != core.UnitTypeNumber || toUnit.Type != core.UnitTypeNumber {
		return core.Result{}, fmt.Errorf("base module only supports number conversion")
	}
	if !m.isBaseUnit(value.Unit.Name) || !m.isBaseUnit(toUnit.Name) {
		return core.Result{}, fmt.Errorf("unsupported base conversion: %s -> %s", value.Unit.Name, toUnit.Name)
	}
	if !m.isInteger(value.RawValue) {
		return core.Result{}, fmt.Errorf("base conversion only supports integers")
	}

	displayValue, err := m.formatValueForUnit(value.RawValue.BigInt(), toUnit.Name)
	if err != nil {
		return core.Result{}, err
	}

	return core.Result{
		DisplayValue: fmt.Sprintf("%s %s", displayValue, toUnit.Name),
		RawValue:     value.RawValue,
		Unit:         toUnit,
		Module:       m,
	}, nil
}

func (m *BaseModule) TokenPatterns() []core.TokenPattern {
	return []core.TokenPattern{
		{
			Pattern:   `(?i)0x[0-9a-f]+`,
			Type:      core.IdentToken,
			Priority:  1000,
			FullMatch: false,
		},
		{
			Pattern:   `(?i)0b[01]+`,
			Type:      core.IdentToken,
			Priority:  1000,
			FullMatch: false,
		},
		{
			Pattern:   `(?i)0o[0-7]+`,
			Type:      core.IdentToken,
			Priority:  1000,
			FullMatch: false,
		},
		{
			Pattern:   `(?i)([0-9a-f]+)\s*hex`,
			Type:      core.IdentToken,
			Priority:  950,
			FullMatch: false,
		},
		{
			Pattern:   `(?i)hex\s*([0-9a-f]+)`,
			Type:      core.IdentToken,
			Priority:  950,
			FullMatch: false,
		},
		{
			Pattern:   `(?i)([01]+)\s*bin`,
			Type:      core.IdentToken,
			Priority:  940,
			FullMatch: false,
		},
		{
			Pattern:   `(?i)bin\s*([01]+)`,
			Type:      core.IdentToken,
			Priority:  940,
			FullMatch: false,
		},
		{
			Pattern:   `(?i)([0-7]+)\s*oct`,
			Type:      core.IdentToken,
			Priority:  930,
			FullMatch: false,
		},
		{
			Pattern:   `(?i)oct\s*([0-7]+)`,
			Type:      core.IdentToken,
			Priority:  930,
			FullMatch: false,
		},
		{
			Pattern:   `(?i)([0-9]+)\s*dec`,
			Type:      core.IdentToken,
			Priority:  920,
			FullMatch: false,
		},
		{
			Pattern:   `(?i)dec\s*([0-9]+)`,
			Type:      core.IdentToken,
			Priority:  920,
			FullMatch: false,
		},
		{
			Pattern:   `(?i)(?:in|to)\s+(bin|oct|dec|hex)`,
			Type:      core.ConversionToken,
			Priority:  900,
			FullMatch: false,
		},
		{
			Pattern:   `(?i)=\s*\?\s*(bin|oct|dec|hex)`,
			Type:      core.ConversionToken,
			Priority:  800,
			FullMatch: false,
		},
	}
}

func (m *BaseModule) handleHexPrefix(ctx context.Context, matches []string) (core.Result, error) {
	return m.handleBaseValue(matches[1], 16, "hex")
}

func (m *BaseModule) handleHexSuffix(ctx context.Context, matches []string) (core.Result, error) {
	return m.handleBaseValue(matches[1], 16, "hex")
}

func (m *BaseModule) handleHexPrefixWord(ctx context.Context, matches []string) (core.Result, error) {
	return m.handleBaseValue(matches[1], 16, "hex")
}

func (m *BaseModule) handleBinPrefix(ctx context.Context, matches []string) (core.Result, error) {
	return m.handleBaseValue(matches[1], 2, "bin")
}

func (m *BaseModule) handleBinSuffix(ctx context.Context, matches []string) (core.Result, error) {
	return m.handleBaseValue(matches[1], 2, "bin")
}

func (m *BaseModule) handleBinPrefixWord(ctx context.Context, matches []string) (core.Result, error) {
	return m.handleBaseValue(matches[1], 2, "bin")
}

func (m *BaseModule) handleOctPrefix(ctx context.Context, matches []string) (core.Result, error) {
	return m.handleBaseValue(matches[1], 8, "oct")
}

func (m *BaseModule) handleOctSuffix(ctx context.Context, matches []string) (core.Result, error) {
	return m.handleBaseValue(matches[1], 8, "oct")
}

func (m *BaseModule) handleOctPrefixWord(ctx context.Context, matches []string) (core.Result, error) {
	return m.handleBaseValue(matches[1], 8, "oct")
}

func (m *BaseModule) handleDecSuffix(ctx context.Context, matches []string) (core.Result, error) {
	return m.handleBaseValue(matches[1], 10, "dec")
}

func (m *BaseModule) handleDecPrefixWord(ctx context.Context, matches []string) (core.Result, error) {
	return m.handleBaseValue(matches[1], 10, "dec")
}

func (m *BaseModule) handleBaseConversionToken(ctx context.Context, matches []string) (core.Result, error) {
	unit := strings.ToLower(matches[1])
	if !m.isBaseUnit(unit) {
		return core.Result{}, fmt.Errorf("unsupported base unit: %s", unit)
	}

	return core.Result{
		DisplayValue: fmt.Sprintf("to %s", unit),
		RawValue:     decimal.NewFromInt(0),
		Unit:         core.Unit{Name: unit, Type: core.UnitTypeNumber},
		Module:       m,
	}, nil
}

func (m *BaseModule) handleBaseValue(value string, base int, unitName string) (core.Result, error) {
	parsed := new(big.Int)
	if _, ok := parsed.SetString(value, base); !ok {
		return core.Result{}, fmt.Errorf("invalid %s value: %s", unitName, value)
	}

	displayValue, err := m.formatValueForUnit(parsed, unitName)
	if err != nil {
		return core.Result{}, err
	}

	return core.Result{
		DisplayValue: fmt.Sprintf("%s %s", displayValue, unitName),
		RawValue:     decimal.NewFromBigInt(parsed, 0),
		Unit:         core.Unit{Name: unitName, Type: core.UnitTypeNumber},
		Module:       m,
	}, nil
}

func (m *BaseModule) formatValueForUnit(value *big.Int, unitName string) (string, error) {
	switch strings.ToLower(unitName) {
	case "bin":
		return value.Text(2), nil
	case "oct":
		return value.Text(8), nil
	case "dec":
		return value.Text(10), nil
	case "hex":
		return strings.ToUpper(value.Text(16)), nil
	default:
		return "", fmt.Errorf("unsupported base unit: %s", unitName)
	}
}

func (m *BaseModule) isBaseUnit(unitName string) bool {
	switch strings.ToLower(unitName) {
	case "bin", "oct", "dec", "hex":
		return true
	default:
		return false
	}
}

func (m *BaseModule) isInteger(value decimal.Decimal) bool {
	return value.Equal(value.Truncate(0))
}
