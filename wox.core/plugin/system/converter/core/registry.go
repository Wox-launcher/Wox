package core

import (
	"context"

	"github.com/shopspring/decimal"
)

// UnitType represents the type of unit in a calculation result
type UnitType int

const (
	UnitTypeNumber   UnitType = iota // For pure numbers without unit
	UnitTypeCrypto                   // For cryptocurrency units (BTC, ETH, etc.)
	UnitTypeCurrency                 // For fiat currency units (USD, EUR, etc.)
	UnitTypeTime                     // For time units (seconds, minutes, etc.)
)

type Unit struct {
	Name string   // The name of the unit, E.g. "USD", "BTC", "ETH", "seconds", "minutes", etc.
	Type UnitType // The type of the unit
}

var (
	UnitUSD          Unit = Unit{Name: "USD", Type: UnitTypeCurrency}      // display as $xxx
	UnitUTCTimestamp Unit = Unit{Name: "UTCTimestamp", Type: UnitTypeTime} // display as timestamp
	UnitUSDT         Unit = Unit{Name: "USDT", Type: UnitTypeCrypto}       // display as $xxx
)

// Result represents a calculation result
type Result struct {
	// The display value that will be shown to user
	DisplayValue string
	// The raw value that will be used for calculation
	RawValue decimal.Decimal
	// The unit of the result (optional)
	Unit Unit
	// The module that this result belongs to
	Module Module
}

// Module interface defines methods that a calculator module must implement
type Module interface {
	// Name returns the name of the module
	Name() string

	// Calculate parses the token and returns a result
	Calculate(ctx context.Context, token Token) (Result, error)

	// Convert converts a value from one unit to another within the same unit type
	Convert(ctx context.Context, value Result, toUnit Unit) (Result, error)

	// TokenPatterns returns the token patterns for this module
	TokenPatterns() []TokenPattern
}

// ModuleRegistry holds all calculator modules
// Uses a slice to preserve registration order (deterministic iteration)
type ModuleRegistry struct {
	modules []Module
}

func NewModuleRegistry() *ModuleRegistry {
	return &ModuleRegistry{
		modules: make([]Module, 0),
	}
}

func (r *ModuleRegistry) Register(module Module) {
	r.modules = append(r.modules, module)
}

func (r *ModuleRegistry) GetModule(name string) Module {
	for _, module := range r.modules {
		if module.Name() == name {
			return module
		}
	}
	return nil
}

func (r *ModuleRegistry) Modules() []Module {
	return r.modules
}

func (r *ModuleRegistry) GetTokenPatterns() []TokenPattern {
	var patterns []TokenPattern
	for _, module := range r.modules {
		patterns = append(patterns, module.TokenPatterns()...)
	}
	return patterns
}
