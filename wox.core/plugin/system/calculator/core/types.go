package core

import (
	"context"
	"fmt"

	"github.com/shopspring/decimal"
)

type TokenKind string

const (
	ReservedToken TokenKind = "reserved"
	NumberToken   TokenKind = "number"
	IdentToken    TokenKind = "ident"
	EosToken      TokenKind = "eos"
)

type Token struct {
	Kind TokenKind
	Val  decimal.Decimal
	Str  string
}

// TokenPattern defines how a module's tokens should be recognized
type TokenPattern struct {
	Pattern  string
	Type     TokenKind
	Priority int
	// If true, try to match the entire remaining input
	FullMatch bool
}

// Value represents a value with its unit
type Value struct {
	Amount decimal.Decimal
	Unit   string // e.g., "USD", "BTC", "m", "kg"
}

// NodeKind represents the type of AST node
type NodeKind string

const (
	AddNode   NodeKind = "+"
	SubNode   NodeKind = "-"
	MulNode   NodeKind = "*"
	DivNode   NodeKind = "/"
	FuncNode  NodeKind = "func"
	NumNode   NodeKind = "num"
	IdentNode NodeKind = "ident"
)

// Node represents a node in the AST
type Node struct {
	Kind     NodeKind
	Left     *Node
	Right    *Node
	FuncName string
	Args     []*Node
	Val      decimal.Decimal
	Str      string // Used for identifiers
}

// Module represents a calculator module that can handle specific types of calculations
type Module interface {
	// Name returns the name of the module
	Name() string

	// TokenPatterns returns the token patterns this module needs
	TokenPatterns() []TokenPattern

	// CanHandle returns true if this module can handle the given tokens
	CanHandle(ctx context.Context, tokens []Token) bool

	// Parse parses tokens into a Value
	Parse(ctx context.Context, tokens []Token) (*Value, error)

	// Calculate performs the calculation for this module
	Calculate(ctx context.Context, tokens []Token) (*Value, error)

	// Convert converts a value to another unit within the same module
	// For example: USD -> EUR, m -> km
	Convert(ctx context.Context, value *Value, toUnit string) (*Value, error)

	// CanConvertTo returns true if this module can convert to the specified unit
	CanConvertTo(unit string) bool
}

// ModuleRegistry manages all calculator modules
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

func (r *ModuleRegistry) GetTokenPatterns() []TokenPattern {
	patterns := make([]TokenPattern, 0)
	for _, module := range r.modules {
		patterns = append(patterns, module.TokenPatterns()...)
	}
	return patterns
}

// Modules returns all registered modules
func (r *ModuleRegistry) Modules() []Module {
	return r.modules
}

// FindModuleForUnit finds the module that can handle the specified unit
func (r *ModuleRegistry) FindModuleForUnit(unit string) Module {
	for _, module := range r.modules {
		if module.CanConvertTo(unit) {
			return module
		}
	}
	return nil
}

// Convert converts a value from one unit to another, possibly across different modules
// For example: BTC -> USD (requires crypto module to convert BTC to USD)
func (r *ModuleRegistry) Convert(ctx context.Context, value *Value, toUnit string) (*Value, error) {
	// First try to convert within the same module
	fromModule := r.FindModuleForUnit(value.Unit)
	if fromModule != nil && fromModule.CanConvertTo(toUnit) {
		return fromModule.Convert(ctx, value, toUnit)
	}

	// If direct conversion is not possible, try to find a module that can handle the target unit
	toModule := r.FindModuleForUnit(toUnit)
	if toModule == nil {
		return nil, fmt.Errorf("no module can handle unit: %s", toUnit)
	}

	// Try to convert through USD as an intermediate currency
	// This is a simplified example, we might need a more sophisticated conversion graph
	if value.Unit != "USD" {
		usdValue, err := fromModule.Convert(ctx, value, "USD")
		if err != nil {
			return nil, err
		}
		return toModule.Convert(ctx, usdValue, toUnit)
	}

	return nil, fmt.Errorf("cannot convert from %s to %s", value.Unit, toUnit)
}
