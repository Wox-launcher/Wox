package modules

import (
	"context"
	"fmt"
	"math"
	"strings"
	"wox/plugin"
	"wox/plugin/system/calculator/core"

	"github.com/shopspring/decimal"
)

var functions = map[string]interface{}{
	"abs":         math.Abs,
	"acos":        math.Acos,
	"acosh":       math.Acosh,
	"asin":        math.Asin,
	"asinh":       math.Asinh,
	"atan":        math.Atan,
	"atan2":       math.Atan2,
	"atanh":       math.Atanh,
	"cbrt":        math.Cbrt,
	"ceil":        math.Ceil,
	"copysign":    math.Copysign,
	"cos":         math.Cos,
	"cosh":        math.Cosh,
	"dim":         math.Dim,
	"erf":         math.Erf,
	"erfc":        math.Erfc,
	"erfcinv":     math.Erfcinv,
	"erfinv":      math.Erfinv,
	"exp":         math.Exp,
	"exp2":        math.Exp2,
	"expm1":       math.Expm1,
	"fma":         math.FMA,
	"floor":       math.Floor,
	"gamma":       math.Gamma,
	"hypot":       math.Hypot,
	"j0":          math.J0,
	"j1":          math.J1,
	"log":         math.Log,
	"log10":       math.Log10,
	"log1p":       math.Log1p,
	"log2":        math.Log2,
	"logb":        math.Logb,
	"max":         math.Max,
	"min":         math.Min,
	"mod":         math.Mod,
	"nan":         math.NaN,
	"nextafter":   math.Nextafter,
	"pow":         math.Pow,
	"remainder":   math.Remainder,
	"round":       math.Round,
	"roundtoeven": math.RoundToEven,
	"sin":         math.Sin,
	"sinh":        math.Sinh,
	"sqrt":        math.Sqrt,
	"tan":         math.Tan,
	"tanh":        math.Tanh,
	"trunc":       math.Trunc,
	"y0":          math.Y0,
	"y1":          math.Y1,
}

var constants = map[string]float64{
	"e":   math.E,
	"pi":  math.Pi,
	"phi": math.Phi,

	"sqrt2":   math.Sqrt2,
	"sqrte":   math.SqrtE,
	"sqrtpi":  math.SqrtPi,
	"sqrtphi": math.SqrtPhi,

	"ln2":    math.Ln2,
	"log2e":  math.Log2E,
	"ln10":   math.Ln10,
	"log10e": math.Log10E,
}

type MathModule struct {
	api    plugin.API
	parser *core.Parser
}

func NewMathModule(ctx context.Context, api plugin.API) *MathModule {
	return &MathModule{
		api: api,
	}
}

func (m *MathModule) Name() string {
	return "math"
}

func (m *MathModule) TokenPatterns() []core.TokenPattern {
	return []core.TokenPattern{
		{
			Pattern:  `[\+\-\*/\(\),]`,
			Type:     core.ReservedToken,
			Priority: 100,
		},
		{
			Pattern:  `[0-9]+(\.[0-9]+)?`,
			Type:     core.NumberToken,
			Priority: 90,
		},
		{
			Pattern:  `[a-zA-Z][a-zA-Z0-9]*`,
			Type:     core.IdentToken,
			Priority: 80,
		},
	}
}

func (m *MathModule) CanHandle(ctx context.Context, tokens []core.Token) bool {
	if len(tokens) == 0 {
		m.api.Log(ctx, plugin.LogLevelDebug, "MathModule.CanHandle: no tokens")
		return false
	}

	m.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("MathModule.CanHandle: tokens=%+v", tokens))

	firstToken := tokens[0]
	m.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("MathModule.CanHandle: first token kind=%v", firstToken.Kind))
	return firstToken.Kind == core.NumberToken ||
		firstToken.Kind == core.IdentToken ||
		(firstToken.Kind == core.ReservedToken && firstToken.Str == "(")
}

func (m *MathModule) call(funcName string, args []decimal.Decimal) (decimal.Decimal, error) {
	f, ok := functions[funcName]
	if !ok {
		return decimal.Zero, fmt.Errorf("unknown function %s", funcName)
	}
	switch f := f.(type) {
	case func() float64:
		return decimal.NewFromFloat(f()), nil
	case func(float64) float64:
		return decimal.NewFromFloat(f(args[0].InexactFloat64())), nil
	case func(float64, float64) float64:
		return decimal.NewFromFloat(f(args[0].InexactFloat64(), args[1].InexactFloat64())), nil
	case func(float64, float64, float64) float64:
		return decimal.NewFromFloat(f(args[0].InexactFloat64(), args[1].InexactFloat64(), args[2].InexactFloat64())), nil
	default:
		return decimal.Zero, fmt.Errorf("invalid function %s", funcName)
	}
}

func (m *MathModule) calculate(ctx context.Context, n *core.Node) (decimal.Decimal, error) {
	switch n.Kind {
	case core.AddNode:
		left, err := m.calculate(ctx, n.Left)
		if err != nil {
			return decimal.Zero, err
		}
		right, err := m.calculate(ctx, n.Right)
		if err != nil {
			return decimal.Zero, err
		}
		return left.Add(right), nil
	case core.SubNode:
		left, err := m.calculate(ctx, n.Left)
		if err != nil {
			return decimal.Zero, err
		}
		right, err := m.calculate(ctx, n.Right)
		if err != nil {
			return decimal.Zero, err
		}
		return left.Sub(right), nil
	case core.MulNode:
		left, err := m.calculate(ctx, n.Left)
		if err != nil {
			return decimal.Zero, err
		}
		right, err := m.calculate(ctx, n.Right)
		if err != nil {
			return decimal.Zero, err
		}
		return left.Mul(right), nil
	case core.DivNode:
		left, err := m.calculate(ctx, n.Left)
		if err != nil {
			return decimal.Zero, err
		}
		right, err := m.calculate(ctx, n.Right)
		if err != nil {
			return decimal.Zero, err
		}
		return left.Div(right), nil
	case core.NumNode:
		return n.Val, nil
	case core.IdentNode:
		// Check if it's a constant
		if val, ok := constants[strings.ToLower(n.Str)]; ok {
			return decimal.NewFromFloat(val), nil
		}
		// Check if it's a function
		if _, ok := functions[strings.ToLower(n.Str)]; ok {
			return decimal.Zero, fmt.Errorf("function %s must be called with arguments", n.Str)
		}
		return decimal.Zero, fmt.Errorf("unknown identifier: %s", n.Str)
	case core.FuncNode:
		var args []decimal.Decimal
		for _, arg := range n.Args {
			val, err := m.calculate(ctx, arg)
			if err != nil {
				return decimal.Zero, err
			}
			args = append(args, val)
		}
		return m.call(n.FuncName, args)
	}
	return decimal.Zero, fmt.Errorf("unknown node type: %s", n.Kind)
}

func (m *MathModule) Parse(ctx context.Context, tokens []core.Token) (*core.Value, error) {
	m.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("MathModule.Parse: tokens=%+v", tokens))
	m.parser = core.NewParser(tokens)
	node, err := m.parser.Parse(ctx)
	if err != nil {
		m.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("MathModule.Parse: parse error=%v", err))
		return nil, err
	}
	m.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("MathModule.Parse: node=%+v", node))
	result, err := m.calculate(ctx, node)
	if err != nil {
		m.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("MathModule.Parse: calculate error=%v", err))
		return nil, err
	}
	m.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("MathModule.Parse: result=%v", result))
	return &core.Value{Amount: result, Unit: ""}, nil
}

func (m *MathModule) Calculate(ctx context.Context, tokens []core.Token) (*core.Value, error) {
	return m.Parse(ctx, tokens)
}

func (m *MathModule) Convert(ctx context.Context, value *core.Value, toUnit string) (*core.Value, error) {
	return nil, fmt.Errorf("math module doesn't support unit conversion")
}

func (m *MathModule) CanConvertTo(unit string) bool {
	return false
}
