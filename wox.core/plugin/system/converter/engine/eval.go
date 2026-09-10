package engine

import (
	"context"
	"errors"
	"math"
	"math/big"
	"strings"
	"time"
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
	if q.fps != nil {
		env.FPS = q.fps
	}
	if q.substance != "" {
		env.Substance = q.substance
	}
	v, err := c.eval(q.root, env)
	if err != nil {
		return Evaluation{}, err
	}
	if q.root != nil && q.root.op == "note" && len(q.target) == 1 && q.target["Hz"] == 1 && v.Number != nil {
		midi, _ := v.Number.Float64()
		v = Value{Kind: Quantity, Number: ratPlaces(freqFromMIDI(midi), 2), Unit: Unit{"Hz": 1}}
		q.target = nil
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
		// Mixed fiat keeps the authored unit ($200 + €200 stays euros).
		// Lone crypto and crypto+crypto convert to default fiat (1BTC, 1BTC+1ETH).
		// Crypto plus a bare number keeps the coin (1btc + 1 → 2 BTC).
		keepFiat := false
		for k := range v.Unit {
			if c.Units[k].Dimension == "money" && k != target && !c.Crypto[k] {
				keepFiat = true
			}
		}
		keepCrypto := false
		for k := range v.Unit {
			if c.Crypto[k] && hasBareNumberAddend(q.root) {
				keepCrypto = true
			}
		}
		if !keepFiat && !keepCrypto {
			v, err = c.convert(v, Unit{target: 1}, env, nil)
			if err != nil {
				return Evaluation{}, err
			}
		}
	}
	if q.speed != nil {
		if !sameUnit(c.dimensions(v.Unit), Unit{"time": 1}) {
			return Evaluation{}, invalid("playback speed needs a duration")
		}
		if q.timeSaved {
			factor := new(big.Rat).Sub(q.speed, big.NewRat(1, 1))
			factor.Quo(factor, q.speed)
			v.Number.Mul(v.Number, factor)
		} else {
			v.Number.Quo(v.Number, q.speed)
		}
		if q.format == "" && len(q.formatUnits) == 0 {
			q.format = "timespan"
		}
	}
	format := q.format
	if format == "" && q.root != nil && q.root.op == "function" {
		switch q.root.text {
		case "hex", "bin", "oct":
			format = q.root.text
		}
	}
	if format == "number" || format == "decimal" || format == "dec" {
		if v.Kind == Clock {
			hours := float64(v.Time.Hour()) + float64(v.Time.Minute())/60 + float64(v.Time.Second())/3600
			v = Value{Kind: Number, Number: ratPlaces(hours, 4)}
			format = ""
		} else if v.Kind == Percent || v.Kind == Quantity {
			v.Kind = Number
			v.Unit = nil
			format = ""
		} else if format == "number" {
			format = ""
		} else if format == "decimal" {
			format = "dec"
		}
	}
	if format == "pitch" {
		if v.Kind == Quantity && v.Unit["Hz"] == 1 {
			f, _ := v.Number.Float64()
			v = Value{Kind: Number, Number: ratPlaces(midiFromFreq(f), 4)}
		}
	}
	if format == "midi" && v.Kind == Quantity && v.Unit["Hz"] == 1 {
		f, _ := v.Number.Float64()
		v = Value{Kind: Number, Number: ratPlaces(midiFromFreq(f), 0)}
	}
	if format == "" && q.laptime && len(q.target) == 0 && len(q.formatUnits) == 0 && sameUnit(c.dimensions(v.Unit), Unit{"time": 1}) {
		format = "laptime"
	}
	if format == "" && (q.fps != nil || q.timecode) && len(q.target) == 0 && v.Unit["frame"] == 1 {
		format = "timecode"
	}
	if format == "timestamp" && (v.Kind == Date || v.Kind == Instant) {
		v = Value{Kind: Number, Number: big.NewRat(v.Time.Unix(), 1)}
		format = ""
	}
	if format == "iso8601" && v.Kind == Number {
		sec, _ := v.Number.Float64()
		if sec > 1e12 {
			sec = sec / 1000
		}
		v = Value{Kind: Instant, Time: time.Unix(int64(sec), 0).UTC()}
	}
	if format == "iso8601" && (v.Kind == Date || v.Kind == Instant || v.Kind == Clock) {
		if v.Kind == Date {
			// "today as iso" uses the current instant, matching Soulver's iso alias.
			if q.root != nil && q.root.op == "temporal" && q.root.temporal != nil &&
				q.root.temporal.kind == "relative" && strings.EqualFold(q.root.temporal.date, "today") {
				v = Value{Kind: Instant, Time: env.Now.In(env.Local)}
			} else {
				v.Time = time.Date(v.Time.Year(), v.Time.Month(), v.Time.Day(), 0, 0, 0, 0, env.Local)
				v.Kind = Instant
			}
		}
		format = "iso8601"
	}
	if format == "date" && (v.Kind == Instant || v.Kind == Clock) {
		v.Kind = Date
		format = ""
	}
	if format == "date" && v.Kind == Number {
		sec, _ := v.Number.Float64()
		if sec > 1e12 {
			sec = sec / 1000
		}
		v = Value{Kind: Date, Time: time.Unix(int64(sec), 0).In(env.Local)}
		format = ""
	}
	if format == "" && q.root != nil && q.root.op == "/" && len(q.target) == 0 && sameUnit(c.dimensions(v.Unit), Unit{"time": 1}) {
		if right, err := c.eval(q.root.args[1], env); err == nil && right.Kind == Number {
			format = "timespan"
		}
	}
	if (format == "timespan" || format == "laptime" || format == "parts") && !sameUnit(c.dimensions(v.Unit), Unit{"time": 1}) {
		return Evaluation{}, invalid("timespan needs a duration")
	}
	if isBase(format) && (v.Kind != Number || !v.Number.IsInt()) {
		return Evaluation{}, invalid("base conversion requires an integer")
	}
	if q.root.text == "base" && format == "" {
		return Evaluation{}, invalid("base conversion requires a target")
	}
	if (q.decimals != nil || q.nearest != nil) && v.Kind != Number && v.Kind != Percent && v.Kind != Quantity {
		return Evaluation{}, invalid("rounding requires a numeric result")
	}
	r := Evaluation{Value: v, Target: format, Expression: q.Expression, Currency: q.Money, RateUpdatedAt: env.RateUpdatedAt, Decimals: q.decimals, Nearest: q.nearest, RoundDir: q.roundDir, FormatUnits: q.formatUnits, FPS: env.FPS, Substance: q.substance}
	if q.root.op == "ratio" {
		a, _ := c.eval(q.root.args[0], env)
		b, _ := c.eval(q.root.args[1], env)
		r.Ratio = plain(a.Number) + ":" + plain(b.Number)
	}
	// Keep the existing single-duration shortcuts in presentation selection only.
	if len(q.target) == 0 && format == "" && q.decimals == nil && q.nearest == nil && q.speed == nil && len(q.formatUnits) == 0 && q.root.op == "quantity" && len(v.Unit) == 1 {
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
		if v.Number != nil {
			v.Number = new(big.Rat).Set(v.Number)
		}
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
	if v, ok, err := evalSoulverMore(n, args, env); ok || err != nil {
		return v, err
	}
	if v, ok, err := evalSoulverExtra(n, args, env); ok || err != nil {
		return v, err
	}
	if v, ok, err := c.evalSoulver(n, args, env); ok || err != nil {
		return v, err
	}
	a := args[0]
	switch n.op {
	case "quantity":
		if a.Kind == Quantity && c.dimensions(a.Unit)["money"] == 1 && c.dimensions(n.value.Unit)["money"] == 1 {
			out := a
			out.Unit = copyUnit(n.value.Unit)
			for k, exp := range a.Unit {
				if c.Units[k].Dimension != "money" {
					out.Unit[k] += exp
				}
			}
			return out, nil
		}
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
	if a.Kind == Number && isYearNumber(a) && isTemporal(b.Kind) {
		a = yearAsDate(a, env)
	}
	if b.Kind == Number && isYearNumber(b) && isTemporal(a.Kind) {
		b = yearAsDate(b, env)
	}
	if isTemporal(a.Kind) || isTemporal(b.Kind) {
		return c.temporalOperation(a, b, n.op, env)
	}
	switch n.op {
	case "+", "-":
		return c.add(a, b, n.op == "-", n.args[1].op == "quantity", env)
	case "of":
		if a.Kind == Number && b.Kind != Percent && !c.absolute(b.Unit) {
			b.Number.Mul(b.Number, a.Number)
			return checked(b)
		}
		fallthrough
	case "off", "on", "tip":
		if a.Kind != Percent || b.Kind == Percent || c.absolute(b.Unit) {
			return Value{}, invalid("expected percentage of a number or quantity")
		}
		factor := a.Number
		if n.op == "off" {
			factor = new(big.Rat).Sub(big.NewRat(1, 1), factor)
		} else if n.op == "on" {
			factor = new(big.Rat).Add(big.NewRat(1, 1), factor)
		}
		// Tip queries return the tip amount; the total is not silently substituted.
		b.Number.Mul(b.Number, factor)
		return checked(b)
	case "*", "/", "ratio":
		return c.multiply(a, b, n.op != "*", env)
	case "^", "power":
		return c.power(a, b)
	case "mod":
		if a.Kind != Number && a.Kind != Quantity || b.Kind != Number && b.Kind != Quantity {
			return Value{}, invalid("modulo requires numbers")
		}
		if b.Number.Sign() == 0 {
			return Value{}, invalid("division by zero")
		}
		ai := new(big.Int).Quo(a.Number.Num(), a.Number.Denom())
		bi := new(big.Int).Quo(b.Number.Num(), b.Number.Denom())
		if bi.Sign() == 0 {
			return Value{}, invalid("division by zero")
		}
		a.Kind = Number
		a.Unit = nil
		a.Number = new(big.Rat).SetInt(new(big.Int).Mod(ai, bi))
		return checked(a)
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
	if a.Kind == Percent && b.Kind == Number {
		// 1.0 is 100% when mixed with a percentage.
		b.Kind = Percent
	}
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
	// A bare number inherits the other operand's unit: 300 + 20 km, $20 + 30, 1btc + 1.
	// Absolute temperatures treat the number as a difference, not a second point.
	if a.Kind == Number && b.Kind == Quantity {
		a.Kind = Quantity
		if c.absolute(b.Unit) {
			a.Unit = deltaUnit(b.Unit)
		} else {
			a.Unit = copyUnit(b.Unit)
		}
	} else if b.Kind == Number && a.Kind == Quantity {
		b.Kind = Quantity
		if c.absolute(a.Unit) {
			b.Unit = deltaUnit(a.Unit)
		} else {
			b.Unit = copyUnit(a.Unit)
		}
	}
	if a.Kind != b.Kind {
		return Value{}, invalid("incompatible addition")
	}
	if a.Kind == Quantity && b.Kind == Quantity {
		if v, ok, err := c.addFramesAndTime(a, b, subtract, env); ok || err != nil {
			return v, err
		}
	}
	if a.Kind == Quantity {
		if c.dimensions(a.Unit)["time"] == -1 && c.dimensions(b.Unit)["time"] == -1 {
			if periodSeconds(b.Unit) > periodSeconds(a.Unit) {
				a, b = b, a
			}
		}
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
		if c.dimensions(a.Unit)["money"] == 1 && c.dimensions(b.Unit)["money"] == 1 && c.dimensions(a.Unit)["time"] == 0 && c.dimensions(b.Unit)["time"] == 0 {
			converted, err := c.convert(a, b.Unit, env, nil)
			if err != nil {
				return Value{}, err
			}
			a = converted
			target = b.Unit
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
	if v, ok, err := c.frameRateOp(a, b, divide, env); ok || err != nil {
		return v, err
	}
	// Soulver treats money × duration as an implicit per-duration rate.
	da, db := c.dimensions(a.Unit), c.dimensions(b.Unit)
	if !divide && ((da["mass"] != 0 && db["volume"] != 0) || (da["volume"] != 0 && db["mass"] != 0)) {
		return Value{}, invalid("unsupported compound unit")
	}
	if !divide && da["speed"] == 1 && db["time"] == 1 {
		return c.speedTimesDuration(a, b, env)
	}
	if !divide && db["speed"] == 1 && da["time"] == 1 {
		return c.speedTimesDuration(b, a, env)
	}
	if !divide && da["money"] == 1 && da["time"] == 0 && db["time"] == 1 && db["money"] == 0 {
		a.Number, _ = calc.Arithmetic("*", a.Number, b.Number)
		return checked(a)
	}
	if !divide && db["money"] == 1 && db["time"] == 0 && da["time"] == 1 && da["money"] == 0 {
		b.Number, _ = calc.Arithmetic("*", a.Number, b.Number)
		return checked(b)
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
	a = canonicalizeDerived(a)
	return checked(a)
}

// speedTimesDuration turns mph × minutes into a length in the speed's length unit.
func (c *Catalog) speedTimesDuration(speed, dur Value, env Env) (Value, error) {
	ms, err := c.factor(speed.Unit, env)
	if err != nil {
		return Value{}, err
	}
	sec, err := c.factor(dur.Unit, env)
	if err != nil {
		return Value{}, err
	}
	meters := new(big.Rat).Mul(speed.Number, ms)
	meters.Mul(meters, dur.Number)
	meters.Mul(meters, sec)
	length := speedLengthSymbol(speed.Unit)
	scale, err := c.factor(Unit{length: 1}, env)
	if err != nil {
		return Value{}, err
	}
	meters.Quo(meters, scale)
	return checked(Value{Kind: Quantity, Number: meters, Unit: Unit{length: 1}})
}

func speedLengthSymbol(u Unit) string {
	switch {
	case u["mph"] != 0:
		return "mi"
	case u["kmh"] != 0:
		return "km"
	case u["kn"] != 0:
		return "nmi"
	}
	return "m"
}

func canonicalizeDerived(v Value) Value {
	if v.Kind != Quantity {
		return v
	}
	switch {
	case len(v.Unit) == 3 && v.Unit["kg"] == 1 && v.Unit["m"] == 1 && v.Unit["s"] == -2:
		v.Unit = Unit{"N": 1}
	case len(v.Unit) == 3 && v.Unit["lb"] == 1 && v.Unit["ft"] == 1 && v.Unit["s"] == -2:
		v.Unit = Unit{"pdl": 1}
	case len(v.Unit) == 2 && v.Unit["mol"] == 1 && v.Unit["l"] == -1:
		v.Unit = Unit{"M": 1}
	case len(v.Unit) == 2 && v.Unit["N"] == 1 && v.Unit["m"] == 1:
		v.Unit = Unit{"Nm": 1}
	case len(v.Unit) == 3 && v.Unit["kg"] == 1 && v.Unit["m"] == 2 && v.Unit["s"] == -2:
		v.Unit = Unit{"Nm": 1}
	}
	return v
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
	return strings.Contains("|cot|csc|sec|coth|csch|sech|acot|acsc|asec|acoth|acsch|asech|cotd|cscd|secd|acotd|acscd|asecd|hex|bin|int|oct|sind|cosd|tand|asind|acosd|atand|fact|ln|", "|"+s+"|") && s != ""
}

// function validates arity and units before calling the shared approximate math library.
func (c *Catalog) function(name string, args []Value, env Env) (Value, error) {
	if len(args) == 0 {
		return Value{}, invalid("missing function arguments")
	}
	if len(args) == 1 && args[0].Kind == Quantity && args[0].Unit["deg"] == 1 && len(args[0].Unit) == 1 {
		switch name {
		case "sin", "cos", "tan", "asin", "acos", "atan":
			name += "d"
			args[0].Kind = Number
			args[0].Unit = nil
		}
	}
	var values []*big.Rat
	for _, a := range args {
		n := a.Number
		if a.Kind == Quantity && a.Unit["deg"] == 1 && len(a.Unit) == 1 && name != "sind" && name != "cosd" && name != "tand" && name != "asind" && name != "acosd" && name != "atand" && name != "cotd" && name != "cscd" && name != "secd" && name != "acotd" && name != "acscd" && name != "asecd" {
			n = new(big.Rat).Mul(a.Number, rational("0.017453292519943295769236907684886"))
			a.Kind = Number
			a.Unit = nil
		} else if a.Kind != Number && !((name == "sqrt" || name == "cbrt") && a.Kind == Quantity) {
			return Value{}, invalid("function requires numbers")
		}
		values = append(values, n)
	}
	result := args[0]
	if result.Unit["deg"] == 1 {
		result.Kind = Number
		result.Unit = nil
	}
	if (name == "sqrt" || name == "cbrt") && result.Kind == Quantity {
		if c.absolute(result.Unit) {
			return Value{}, invalid("absolute temperature root")
		}
		divisor := 2
		if name == "cbrt" {
			divisor = 3
		}
		result.Unit = copyUnit(result.Unit)
		for k, n := range result.Unit {
			if n%divisor != 0 {
				return Value{}, invalid("fractional unit dimension")
			}
			result.Unit[k] = n / divisor
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
		case "hex", "bin", "int", "oct":
			if !values[0].IsInt() {
				values[0] = new(big.Rat).SetInt(new(big.Int).Quo(values[0].Num(), values[0].Denom()))
			}
			result.Number = values[0]
			return checked(result)
		case "sind":
			v = math.Sin(x * math.Pi / 180)
		case "cosd":
			v = math.Cos(x * math.Pi / 180)
		case "tand":
			v = math.Tan(x * math.Pi / 180)
		case "asind":
			v = math.Asin(x) * 180 / math.Pi
		case "acosd":
			v = math.Acos(x) * 180 / math.Pi
		case "atand":
			v = math.Atan(x) * 180 / math.Pi
		case "cotd":
			v = 1 / math.Tan(x*math.Pi/180)
		case "cscd":
			v = 1 / math.Sin(x*math.Pi/180)
		case "secd":
			v = 1 / math.Cos(x*math.Pi/180)
		case "acotd":
			v = math.Atan2(1, x) * 180 / math.Pi
		case "acscd":
			v = math.Asin(1/x) * 180 / math.Pi
		case "asecd":
			v = math.Acos(1/x) * 180 / math.Pi
		case "fact":
			if x < 0 || x != math.Trunc(x) || x > 200 {
				return Value{}, invalid("invalid factorial")
			}
			n := 1.0
			for i := 2; i <= int(x); i++ {
				n *= float64(i)
			}
			v = n
		case "ln":
			v = math.Log(x)
		}
		if math.Abs(v-math.Round(v)) < 1e-10 {
			v = math.Round(v)
		} else if math.Abs(v*2-math.Round(v*2)) < 1e-10 {
			v = math.Round(v*2) / 2
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

func periodSeconds(u Unit) float64 {
	for k, n := range u {
		if n != -1 {
			continue
		}
		switch k {
		case "y":
			return 31556952
		case "mo":
			return 2629746
		case "month":
			return 2629746
		case "w":
			return 604800
		case "d":
			return 86400
		case "h":
			return 3600
		case "min", "?m":
			return 60
		case "s":
			return 1
		case "workday":
			return 28800
		}
	}
	return 0
}

func isYearNumber(v Value) bool {
	if v.Number == nil || !v.Number.IsInt() || !v.Number.Num().IsInt64() {
		return false
	}
	y := v.Number.Num().Int64()
	return y >= 1000 && y <= 9999
}

func yearAsDate(v Value, env Env) Value {
	y := int(v.Number.Num().Int64())
	return Value{Kind: Date, Time: time.Date(y, 1, 1, 0, 0, 0, 0, env.Local)}
}

// hasBareNumberAddend reports +/− of a money quantity with a dimensionless number.
func hasBareNumberAddend(n *node) bool {
	if n == nil || (n.op != "+" && n.op != "-") || len(n.args) != 2 {
		return false
	}
	return isBareNumberNode(n.args[0]) || isBareNumberNode(n.args[1])
}

func isBareNumberNode(n *node) bool {
	if n == nil {
		return false
	}
	if n.op == "value" && n.value.Kind == Number {
		return true
	}
	return n.op == "quantity" && len(n.value.Unit) == 0
}
