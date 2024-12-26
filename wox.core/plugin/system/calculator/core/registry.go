package core

import (
	"context"
	"fmt"
)

// Module represents a calculator module that can handle specific types of calculations
type Module interface {
	// Name returns the name of the module
	Name() string

	// TokenPatterns returns the token patterns this module needs
	TokenPatterns() []TokenPattern

	// CanHandle returns true if this module can handle the given tokens
	CanHandle(ctx context.Context, tokens []Token) bool

	// Parse parses tokens into a Result
	Parse(ctx context.Context, tokens []Token) (*Result, error)

	// Calculate performs the calculation for this module
	Calculate(ctx context.Context, tokens []Token) (*Result, error)

	// Convert converts a result to another unit within the same module
	// For example: USD -> EUR, m -> km
	Convert(ctx context.Context, value *Result, toUnit string) (*Result, error)

	// CanConvertTo returns true if this module can convert to the specified unit
	CanConvertTo(unit string) bool
}

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

func (r *ModuleRegistry) Modules() []Module {
	return r.modules
}

func (r *ModuleRegistry) GetModule(name string) Module {
	for _, module := range r.modules {
		if module.Name() == name {
			return module
		}
	}
	return nil
}

func (r *ModuleRegistry) GetTokenPatterns() []TokenPattern {
	var patterns []TokenPattern
	for _, module := range r.modules {
		patterns = append(patterns, module.TokenPatterns()...)
	}
	return patterns
}

func (r *ModuleRegistry) Convert(ctx context.Context, value *Result, toUnit string) (*Result, error) {
	// Try to find a module that can convert to the target unit
	for _, module := range r.modules {
		if module.CanConvertTo(toUnit) {
			return module.Convert(ctx, value, toUnit)
		}
	}
	return nil, fmt.Errorf("no module can convert to unit: %s", toUnit)
}
