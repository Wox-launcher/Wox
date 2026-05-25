package modules

import (
	"context"
	"fmt"
	"strings"
	"wox/plugin"
	"wox/plugin/system/converter/core"

	"github.com/shopspring/decimal"
)

type unitSpec struct {
	unitType core.UnitType
	singular string
	plural   string
	toBase   func(decimal.Decimal) decimal.Decimal
	fromBase func(decimal.Decimal) decimal.Decimal
}

type UnitModule struct {
	*regexBaseModule
	units map[string]unitSpec
}

func NewUnitModule(ctx context.Context, api plugin.API) *UnitModule {
	m := &UnitModule{
		units: map[string]unitSpec{},
	}

	const (
		numberPattern    = `([+-]?\d+(?:\.\d+)?)`
		lengthUnits      = `(?:mm|millimeter|millimeters|cm|centimeter|centimeters|m|meter|meters|metre|metres|km|kilometer|kilometers|kilometre|kilometres|inch|inches|ft|foot|feet|yd|yard|yards|mi|mile|miles)`
		weightUnits      = `(?:mg|milligram|milligrams|g|gram|grams|kg|kilogram|kilograms|oz|ounce|ounces|lb|lbs|pound|pounds|t|ton|tons)`
		temperatureUnits = `(?:°?\s*c|celsius|centigrade|°?\s*f|fahrenheit|°?\s*k|kelvin)`
		storageUnits     = `(?:b|byte|bytes)`
	)

	// Length, weight, and temperature add single-letter aliases like "m", "g", and "c".
	// Matching the whole conversion query avoids stealing those tokens from the existing
	// time parser, which already uses short forms such as "1m" for minutes.
	handlers := []*patternHandler{
		{
			Pattern:     numberPattern + `\s*(` + lengthUnits + `)\s*(?:to|in|=\s*\?)\s*(` + lengthUnits + `)`,
			Priority:    1500,
			Description: "Convert length units (e.g. 10cm to mm)",
			Handler:     m.handleUnitConversion,
			FullMatch:   true,
		},
		{
			Pattern:     numberPattern + `\s*(` + weightUnits + `)\s*(?:to|in|=\s*\?)\s*(` + weightUnits + `)`,
			Priority:    1500,
			Description: "Convert weight units (e.g. 100lb to kg)",
			Handler:     m.handleUnitConversion,
			FullMatch:   true,
		},
		{
			Pattern:     numberPattern + `\s*(` + temperatureUnits + `)\s*(?:to|in|=\s*\?)\s*(` + temperatureUnits + `)`,
			Priority:    1500,
			Description: "Convert temperature units (e.g. 32f to c)",
			Handler:     m.handleUnitConversion,
			FullMatch:   true,
		},
		{
			Pattern:     numberPattern + `\s*(` + storageUnits + `)\s*(?:to|in|=\s*\?)\s*(` + storageUnits + `)`,
			Priority:    1500,
			Description: "Convert byte aliases (e.g. 32 b to bytes)",
			Handler:     m.handleUnitConversion,
			FullMatch:   true,
		},
	}

	m.regexBaseModule = NewRegexBaseModule(api, "units", handlers)
	m.registerLinearUnit(
		[]string{"mm", "millimeter", "millimeters"},
		core.UnitTypeLength,
		"millimeter",
		"millimeters",
		decimal.RequireFromString("0.001"),
	)
	m.registerLinearUnit(
		[]string{"cm", "centimeter", "centimeters"},
		core.UnitTypeLength,
		"centimeter",
		"centimeters",
		decimal.RequireFromString("0.01"),
	)
	m.registerLinearUnit(
		[]string{"m", "meter", "meters", "metre", "metres"},
		core.UnitTypeLength,
		"meter",
		"meters",
		decimal.NewFromInt(1),
	)
	m.registerLinearUnit(
		[]string{"km", "kilometer", "kilometers", "kilometre", "kilometres"},
		core.UnitTypeLength,
		"kilometer",
		"kilometers",
		decimal.RequireFromString("1000"),
	)
	m.registerLinearUnit(
		[]string{"inch", "inches"},
		core.UnitTypeLength,
		"inch",
		"inches",
		decimal.RequireFromString("0.0254"),
	)
	m.registerLinearUnit(
		[]string{"ft", "foot", "feet"},
		core.UnitTypeLength,
		"foot",
		"feet",
		decimal.RequireFromString("0.3048"),
	)
	m.registerLinearUnit(
		[]string{"yd", "yard", "yards"},
		core.UnitTypeLength,
		"yard",
		"yards",
		decimal.RequireFromString("0.9144"),
	)
	m.registerLinearUnit(
		[]string{"mi", "mile", "miles"},
		core.UnitTypeLength,
		"mile",
		"miles",
		decimal.RequireFromString("1609.344"),
	)
	m.registerLinearUnit(
		[]string{"mg", "milligram", "milligrams"},
		core.UnitTypeWeight,
		"milligram",
		"milligrams",
		decimal.RequireFromString("0.001"),
	)
	m.registerLinearUnit(
		[]string{"g", "gram", "grams"},
		core.UnitTypeWeight,
		"gram",
		"grams",
		decimal.NewFromInt(1),
	)
	m.registerLinearUnit(
		[]string{"kg", "kilogram", "kilograms"},
		core.UnitTypeWeight,
		"kilogram",
		"kilograms",
		decimal.NewFromInt(1000),
	)
	m.registerLinearUnit(
		[]string{"oz", "ounce", "ounces"},
		core.UnitTypeWeight,
		"ounce",
		"ounces",
		decimal.RequireFromString("28.349523125"),
	)
	m.registerLinearUnit(
		[]string{"lb", "lbs", "pound", "pounds"},
		core.UnitTypeWeight,
		"pound",
		"pounds",
		decimal.RequireFromString("453.59237"),
	)
	m.registerLinearUnit(
		[]string{"t", "ton", "tons"},
		core.UnitTypeWeight,
		"ton",
		"tons",
		decimal.NewFromInt(1000000),
	)
	m.registerTemperatureUnit(
		[]string{"c", "°c", "celsius", "centigrade"},
		"celsius",
		func(value decimal.Decimal) decimal.Decimal { return value },
		func(value decimal.Decimal) decimal.Decimal { return value },
	)
	m.registerTemperatureUnit(
		[]string{"f", "°f", "fahrenheit"},
		"fahrenheit",
		func(value decimal.Decimal) decimal.Decimal {
			return value.Sub(decimal.NewFromInt(32)).Mul(decimal.NewFromInt(5)).Div(decimal.NewFromInt(9))
		},
		func(value decimal.Decimal) decimal.Decimal {
			return value.Mul(decimal.NewFromInt(9)).Div(decimal.NewFromInt(5)).Add(decimal.NewFromInt(32))
		},
	)
	m.registerTemperatureUnit(
		[]string{"k", "°k", "kelvin"},
		"kelvin",
		func(value decimal.Decimal) decimal.Decimal {
			return value.Sub(decimal.RequireFromString("273.15"))
		},
		func(value decimal.Decimal) decimal.Decimal {
			return value.Add(decimal.RequireFromString("273.15"))
		},
	)
	m.registerLinearUnit(
		[]string{"b", "byte", "bytes"},
		core.UnitTypeStorage,
		"byte",
		"bytes",
		decimal.NewFromInt(1),
	)

	return m
}

func (m *UnitModule) handleUnitConversion(ctx context.Context, matches []string) (core.Result, error) {
	value, err := decimal.NewFromString(matches[1])
	if err != nil {
		return core.Result{}, fmt.Errorf("invalid unit value: %w", err)
	}

	fromUnitName, fromSpec, err := m.lookupUnit(matches[2])
	if err != nil {
		return core.Result{}, err
	}
	toUnitName, toSpec, err := m.lookupUnit(matches[3])
	if err != nil {
		return core.Result{}, err
	}
	if fromSpec.unitType != toSpec.unitType {
		return core.Result{}, fmt.Errorf("unsupported conversion: %s to %s", matches[2], matches[3])
	}

	convertedValue, err := m.convertValue(value, fromUnitName, toUnitName)
	if err != nil {
		return core.Result{}, err
	}

	return core.Result{
		DisplayValue: m.formatValue(convertedValue, toSpec),
		RawValue:     convertedValue,
		Unit:         core.Unit{Name: toUnitName, Type: toSpec.unitType},
		Module:       m,
	}, nil
}

func (m *UnitModule) Convert(ctx context.Context, value core.Result, toUnit core.Unit) (core.Result, error) {
	fromUnitName, fromSpec, err := m.lookupUnit(value.Unit.Name)
	if err != nil {
		return core.Result{}, err
	}
	toUnitName, toSpec, err := m.lookupUnit(toUnit.Name)
	if err != nil {
		return core.Result{}, err
	}
	// Unit names like "m" overlap with the time module, so checking the alias alone
	// is not enough here. Guard on UnitType as well to keep time conversions routed to
	// the time module instead of silently reinterpreting them as meters.
	if value.Unit.Type != fromSpec.unitType || toUnit.Type != toSpec.unitType {
		return core.Result{}, fmt.Errorf("unit type mismatch: %s (%d) -> %s (%d)", value.Unit.Name, value.Unit.Type, toUnit.Name, toUnit.Type)
	}
	if fromSpec.unitType != toSpec.unitType {
		return core.Result{}, fmt.Errorf("unsupported conversion: %s to %s", value.Unit.Name, toUnit.Name)
	}

	convertedValue, err := m.convertValue(value.RawValue, fromUnitName, toUnitName)
	if err != nil {
		return core.Result{}, err
	}

	return core.Result{
		DisplayValue: m.formatValue(convertedValue, toSpec),
		RawValue:     convertedValue,
		Unit:         core.Unit{Name: toUnitName, Type: toSpec.unitType},
		Module:       m,
	}, nil
}

func (m *UnitModule) registerLinearUnit(aliases []string, unitType core.UnitType, singular string, plural string, factor decimal.Decimal) {
	spec := unitSpec{
		unitType: unitType,
		singular: singular,
		plural:   plural,
		toBase: func(value decimal.Decimal) decimal.Decimal {
			return value.Mul(factor)
		},
		fromBase: func(value decimal.Decimal) decimal.Decimal {
			return value.Div(factor)
		},
	}
	for _, alias := range aliases {
		m.units[normalizeUnitAlias(alias)] = spec
	}
}

func (m *UnitModule) registerTemperatureUnit(aliases []string, displayName string, toBase func(decimal.Decimal) decimal.Decimal, fromBase func(decimal.Decimal) decimal.Decimal) {
	spec := unitSpec{
		unitType: core.UnitTypeTemperature,
		singular: displayName,
		plural:   displayName,
		toBase:   toBase,
		fromBase: fromBase,
	}
	for _, alias := range aliases {
		m.units[normalizeUnitAlias(alias)] = spec
	}
}

func (m *UnitModule) lookupUnit(unit string) (string, unitSpec, error) {
	normalized := normalizeUnitAlias(unit)
	spec, ok := m.units[normalized]
	if !ok {
		return "", unitSpec{}, fmt.Errorf("unsupported unit: %s", unit)
	}
	return normalized, spec, nil
}

func (m *UnitModule) convertValue(value decimal.Decimal, fromUnitName string, toUnitName string) (decimal.Decimal, error) {
	fromSpec, ok := m.units[fromUnitName]
	if !ok {
		return decimal.Decimal{}, fmt.Errorf("unsupported unit: %s", fromUnitName)
	}
	toSpec, ok := m.units[toUnitName]
	if !ok {
		return decimal.Decimal{}, fmt.Errorf("unsupported unit: %s", toUnitName)
	}
	if fromSpec.unitType != toSpec.unitType {
		return decimal.Decimal{}, fmt.Errorf("unsupported conversion: %s to %s", fromUnitName, toUnitName)
	}

	baseValue := fromSpec.toBase(value)
	return toSpec.fromBase(baseValue), nil
}

func (m *UnitModule) formatValue(value decimal.Decimal, spec unitSpec) string {
	displayValue := value
	if spec.unitType == core.UnitTypeLength {
		displayValue = value.Round(3)
	} else {
		displayValue = value.Round(2)
	}
	displayText := displayValue.StringFixedBank(int32(max(displayValue.Exponent()*-1, 0)))

	if strings.Contains(displayText, ".") {
		displayText = strings.TrimRight(strings.TrimRight(displayText, "0"), ".")
	}

	unitName := spec.plural
	if value.Abs().Equal(decimal.NewFromInt(1)) {
		unitName = spec.singular
	}
	return fmt.Sprintf("%s %s", displayText, unitName)
}

func normalizeUnitAlias(alias string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(alias)), " ", "")
}

func max(a int32, b int32) int32 {
	if a > b {
		return a
	}
	return b
}
