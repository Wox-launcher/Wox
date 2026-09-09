package engine

import (
	"context"
	"errors"
	"math"
	"math/big"
	"strings"
	"wox/util/calc"
)

// Evaluate uses a captured environment, so arithmetic never observes changing prices or dates.
func (c *Catalog) Evaluate(ctx context.Context, q *Query, env Env) (result Evaluation, err error) {
	defer func() {
		if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			if _, typed := err.(*Error); !typed {
				err = invalid(err.Error())
			}
		}
	}()

	if err := ctx.Err(); err != nil {
		return Evaluation{}, err
	}
	if env.Local == nil {
		return Evaluation{}, invalid("local timezone is required")
	}
	if q.temporal != nil {
		return c.evaluateTemporal(q, env)
	}
	v, err := c.eval(q.root, env)
	if err != nil {
		return Evaluation{}, err
	}
	if len(q.target) > 0 {
		v, err = c.convert(v, q.target, env, q.ppi)
		if err != nil {
			return Evaluation{}, err
		}
	}
	if len(q.target) == 0 && c.dimensions(v.Unit)["money"] == 1 {
		target := env.DefaultCurrency
		if target == "" {
			target = "USD"
		}
		v, err = c.convert(v, Unit{target: 1}, env, nil)
		if err != nil {
			return Evaluation{}, err
		}
	}
	if q.format == "timespan" && !sameUnit(c.dimensions(v.Unit), Unit{"time": 1}) {
		return Evaluation{}, invalid("timespan needs a duration")
	}
	if isBase(q.format) && (v.Kind != Number || !v.Number.IsInt()) {
		return Evaluation{}, invalid("base conversion requires an integer")
	}
	if q.root.text == "base" && q.format == "" {
		return Evaluation{}, invalid("base conversion requires a target")
	}
	r := Evaluation{Value: v, Target: q.format, Expression: q.Expression, Currency: q.Money, RateUpdatedAt: env.RateUpdatedAt}
	if q.root.op == "ratio" {
		a, _ := c.eval(q.root.args[0], env)
		b, _ := c.eval(q.root.args[1], env)
		r.Ratio = plain(a.Number) + ":" + plain(b.Number)
	}
	// Keep the existing single-duration shortcuts in presentation selection only.
	if len(q.target) == 0 && q.format == "" && q.root.op == "quantity" && len(v.Unit) == 1 {
		target := ""
		if v.Unit["h"] == 1 {
			target = "min"
		}
		if v.Unit["w"] == 1 {
			target = "d"
		}
		if v.Unit["d"] == 1 {
			target = "w"
		}
		if target != "" {
			r.Value, err = c.convert(v, Unit{target: 1}, env, nil)
		}
	}
	return r, err
}

// eval dispatches typed operations after recursively evaluating their operands.
func (c *Catalog) eval(n *node, env Env) (Value, error) {
	if n.op == "temporal" {
		r, e := c.evaluateTemporal(&Query{temporal: n.temporal}, env)
		return r.Value, e
	}
	if n.op == "value" {
		v := n.value
		v.Number = new(big.Rat).Set(v.Number)
		return v, nil
	}
	args := make([]Value, 0, len(n.args))
	for _, a := range n.args {
		v, err := c.eval(a, env)
		if err != nil {
			return Value{}, err
		}
		args = append(args, v)
	}
	if n.op == "function" {
		return c.function(n.text, args, env)
	}
	a := args[0]
	switch n.op {
	case "quantity":
		if a.Kind != Number {
			return Value{}, invalid("unit suffix requires a number")
		}
		if n.value.Unit["month"] == 1 && len(n.value.Unit) == 1 {
			if !a.Number.IsInt() || !a.Number.Num().IsInt64() || a.Number.Num().Int64() < -120000 || a.Number.Num().Int64() > 120000 {
				return Value{}, invalid("calendar span exceeds limit")
			}
			return Value{Kind: CalendarSpan, Months: int(a.Number.Num().Int64())}, nil
		}
		if _, ok := n.value.Unit["month"]; ok {
			return Value{}, invalid("calendar months cannot form compound units")
		}
		a.Kind = Quantity
		a.Unit = copyUnit(n.value.Unit)
		if c.absolute(a.Unit) && (len(a.Unit) != 1 || firstExponent(a.Unit) != 1) {
			return Value{}, invalid("absolute temperature cannot be compounded")
		}
		return checked(a)
	case "percent":
		if a.Kind != Number {
			return Value{}, invalid("percentage requires a number")
		}
		a.Kind = Percent
		a.Number.Quo(a.Number, big.NewRat(100, 1))
		return checked(a)
	case "neg":
		if a.Number == nil {
			return Value{}, invalid("cannot negate a temporal value")
		}
		a.Number.Neg(a.Number)
		return checked(a)
	}
	b := args[1]
	if a.Kind >= Date || b.Kind >= Date {
		return c.temporalOperation(a, b, n.op, env)
	}
	switch n.op {
	case "+", "-":
		return c.add(a, b, n.op == "-", n.args[1].op == "quantity", env)
	case "of", "off", "tip":
		if a.Kind != Percent || b.Kind == Percent || c.absolute(b.Unit) {
			return Value{}, invalid("expected percentage of a number or quantity")
		}
		factor := a.Number
		if n.op == "off" {
			factor = new(big.Rat).Sub(big.NewRat(1, 1), factor)
		}
		// Tip queries return the tip amount; the total is not silently substituted.
		b.Number.Mul(b.Number, factor)
		return checked(b)
	case "*", "/", "ratio":
		return c.multiply(a, b, n.op != "*", env)
	case "^", "power":
		return c.power(a, b)
	}
	return Value{}, invalid("unsupported operation")
}
func firstExponent(u Unit) int {
	for _, n := range u {
		return n
	}
	return 0
}

// add keeps relative percentages and temperature offsets out of ordinary addition.
func (c *Catalog) add(a, b Value, subtract, rightLiteral bool, env Env) (Value, error) {
	if a.Kind == Percent && b.Kind != Percent {
		return Value{}, invalid("percentage must follow the base value")
	}
	if b.Kind == Percent && a.Kind != Percent {
		if c.absolute(a.Unit) {
			return Value{}, invalid("relative absolute temperature")
		}
		b.Number.Mul(a.Number, b.Number)
		b.Kind = a.Kind
		b.Unit = copyUnit(a.Unit)
	}
	if a.Kind != b.Kind {
		return Value{}, invalid("incompatible addition")
	}
	if a.Kind == Quantity {
		absA, absB := c.absolute(a.Unit), c.absolute(b.Unit)
		if absA && absB && rightLiteral && !subtract {
			b.Unit = deltaUnit(b.Unit)
			absB = false
		}
		if !absA && absB {
			return Value{}, invalid("absolute temperature on right")
		}
		if absA && absB && !subtract {
			return Value{}, invalid("cannot add absolute temperatures")
		}
		target := a.Unit
		if absA && !absB {
			target = deltaUnit(a.Unit)
		}
		var err error
		b, err = c.convert(b, target, env, nil)
		if err != nil {
			return Value{}, err
		}
		if absA && absB {
			a.Unit = deltaUnit(a.Unit)
		}
	}
	op := "+"
	if subtract {
		op = "-"
	}
	a.Number, _ = calc.Arithmetic(op, a.Number, b.Number)
	return checked(a)
}

// deltaUnit replaces an absolute temperature factor with its offset-free counterpart.
func deltaUnit(u Unit) Unit {
	d := Unit{}
	for k, n := range u {
		d["delta"+strings.TrimPrefix(k, "°")] = n
	}
	return d
}

// multiply combines factors and applies their scale when compatible dimensions cancel.
func (c *Catalog) multiply(a, b Value, divide bool, env Env) (Value, error) {
	if c.absolute(a.Unit) || c.absolute(b.Unit) {
		return Value{}, invalid("absolute temperature cannot be multiplied")
	}
	if divide && b.Number.Sign() == 0 {
		return Value{}, invalid("division by zero")
	}
	op := "*"
	if divide {
		op = "/"
	}
	a.Number, _ = calc.Arithmetic(op, a.Number, b.Number)
	if a.Kind != Quantity && b.Kind != Quantity {
		if a.Kind == Percent && b.Kind == Number && divide {
			a.Kind = Percent
		} else {
			a.Kind = Number
		}
		return checked(a)
	}
	u := copyUnit(a.Unit)
	sign := 1
	if divide {
		sign = -1
	}
	for k, n := range b.Unit {
		u[k] += sign * n
		if u[k] == 0 {
			delete(u, k)
		}
	}
	a.Unit = u
	a.Kind = Quantity
	// A duration denominator must cancel even when the multiplier uses minutes
	// instead of hours. Preserve remaining authored factors after cancellation.
	for dimension, total := range c.dimensionsWithZeros(u) {
		if total != 0 {
			continue
		}
		group := Unit{}
		for name, exponent := range u {
			if c.Units[name].Dimension == dimension {
				group[name] = exponent
				delete(u, name)
			}
		}
		factor, err := c.factor(group, env)
		if err != nil {
			return Value{}, err
		}
		a.Number.Mul(a.Number, factor)
	}
	// Cancel compatible factors even when their authored units differ (km/m).
	dims := c.dimensions(u)
	if len(dims) == 0 {
		f, err := c.factor(u, env)
		if err != nil {
			return Value{}, err
		}
		a.Number.Mul(a.Number, f)
		a.Kind = Number
		a.Unit = nil
	}
	return checked(a)
}

// dimensionsWithZeros retains canceled dimensions until their scale factors are applied.
func (c *Catalog) dimensionsWithZeros(u Unit) Unit {
	d := Unit{}
	for name, n := range u {
		d[c.Units[name].Dimension] += n
	}
	return d
}

// power bounds exact integer growth before allocating exponentiation results.
func (c *Catalog) power(a, b Value) (Value, error) {
	if b.Kind != Number || c.absolute(a.Unit) {
		return Value{}, invalid("invalid exponent")
	}
	if !b.Number.IsInt() {
		if a.Kind != Number {
			return Value{}, invalid("quantity exponent must be integer")
		}
		x, _ := a.Number.Float64()
		y, _ := b.Number.Float64()
		n, e := calc.RatFromFloat(math.Pow(x, y))
		if e != nil {
			return Value{}, e
		}
		a.Number = n
		return checked(a)
	}
	if !b.Number.Num().IsInt64() {
		return Value{}, invalid("exponent exceeds limit")
	}
	n := b.Number.Num().Int64()
	if n < -4096 || n > 4096 {
		return Value{}, invalid("exponent exceeds limit")
	}
	abs := n
	if abs < 0 {
		abs = -abs
	}
	if int64(a.Number.Num().BitLen())*abs > maxBits || int64(a.Number.Denom().BitLen())*abs > maxBits {
		return Value{}, invalid("power exceeds numeric limit")
	}
	if n < 0 && a.Number.Sign() == 0 {
		return Value{}, invalid("division by zero")
	}
	r := new(big.Rat).SetFrac(new(big.Int).Exp(a.Number.Num(), big.NewInt(abs), nil), new(big.Int).Exp(a.Number.Denom(), big.NewInt(abs), nil))
	if n < 0 {
		r.Inv(r)
	}
	a.Number = r
	u := copyUnit(a.Unit)
	for k, e := range u {
		u[k] = e * int(n)
		if u[k] == 0 {
			delete(u, k)
		}
	}
	a.Unit = u
	if len(u) == 0 {
		a.Kind = Number
	}
	return checked(a)
}

func isExtraFunction(s string) bool {
	return strings.Contains("|cot|csc|sec|coth|csch|sech|acot|acsc|asec|acoth|acsch|asech|", "|"+s+"|") && s != ""
}

// function validates arity and units before calling the shared approximate math library.
func (c *Catalog) function(name string, args []Value, env Env) (Value, error) {
	if len(args) == 0 {
		return Value{}, invalid("missing function arguments")
	}
	var values []*big.Rat
	for _, a := range args {
		if a.Kind != Number && !(name == "sqrt" && a.Kind == Quantity) {
			return Value{}, invalid("function requires numbers")
		}
		values = append(values, a.Number)
	}
	result := args[0]
	if name == "sqrt" && result.Kind == Quantity {
		if c.absolute(result.Unit) {
			return Value{}, invalid("absolute temperature root")
		}
		result.Unit = copyUnit(result.Unit)
		for k, n := range result.Unit {
			if n%2 != 0 {
				return Value{}, invalid("fractional unit dimension")
			}
			result.Unit[k] = n / 2
		}
	}
	if isExtraFunction(name) {
		if len(values) != 1 {
			return Value{}, invalid("function requires one argument")
		}
		x, _ := values[0].Float64()
		var v float64
		switch name {
		case "cot":
			v = 1 / math.Tan(x)
		case "csc":
			v = 1 / math.Sin(x)
		case "sec":
			v = 1 / math.Cos(x)
		case "coth":
			v = 1 / math.Tanh(x)
		case "csch":
			v = 1 / math.Sinh(x)
		case "sech":
			v = 1 / math.Cosh(x)
		case "acot":
			v = math.Atan2(1, x)
		case "acsc":
			v = math.Asin(1 / x)
		case "asec":
			v = math.Acos(1 / x)
		case "acoth":
			v = math.Atanh(1 / x)
		case "acsch":
			v = math.Asinh(1 / x)
		case "asech":
			v = math.Acosh(1 / x)
		}
		r, err := calc.RatFromFloat(v)
		if err != nil {
			return Value{}, err
		}
		result.Number = r
		return checked(result)
	}
	f := calc.Functions[name]
	arity := -1
	switch f.(type) {
	case func() float64:
		arity = 0
	case func(float64) float64:
		arity = 1
	case func(float64, float64) float64:
		arity = 2
	case func(float64, float64, float64) float64:
		arity = 3
	}
	if arity != len(values) {
		return Value{}, invalid("wrong function argument count")
	}
	r, err := calc.Call(name, values)
	if err != nil {
		return Value{}, err
	}
	result.Number = r
	return checked(result)
}
