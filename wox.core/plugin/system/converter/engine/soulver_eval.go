package engine

import (
	"math"
	"math/big"
	"strings"
	"time"
)

// evalSoulver evaluates sentence-level operations added for Soulver syntax.
func (c *Catalog) evalSoulver(n *node, args []Value, env Env) (Value, bool, error) {
	switch n.op {
	case "is_prime":
		if args[0].Number == nil || !args[0].Number.IsInt() {
			return Value{}, true, invalid("expected an integer")
		}
		n := args[0].Number.Num().Int64()
		if n < 2 {
			return Value{Kind: Boolean, Number: new(big.Rat)}, true, nil
		}
		for i := int64(2); i*i <= n; i++ {
			if n%i == 0 {
				return Value{Kind: Boolean, Number: new(big.Rat)}, true, nil
			}
		}
		return Value{Kind: Boolean, Number: big.NewRat(1, 1)}, true, nil
	case "half":
		args[0].Number.Quo(args[0].Number, big.NewRat(2, 1))
		return args[0], true, nil
	case "midpoint":
		if args[0].Kind == Date && args[1].Kind == Date {
			a, b := args[0].Time, args[1].Time
			if b.Before(a) {
				a, b = b, a
			}
			mid := a.Add(b.Sub(a) / 2)
			return Value{Kind: Date, Time: time.Date(mid.Year(), mid.Month(), mid.Day(), 0, 0, 0, 0, a.Location())}, true, nil
		}
		args[0].Number.Add(args[0].Number, args[1].Number)
		args[0].Number.Quo(args[0].Number, big.NewRat(2, 1))
		return args[0], true, nil
	case "max2", "min2":
		if args[0].Number.Cmp(args[1].Number) >= 0 {
			if n.op == "max2" {
				return args[0], true, nil
			}
			return args[1], true, nil
		}
		if n.op == "max2" {
			return args[1], true, nil
		}
		return args[0], true, nil
	case "gcd":
		g := new(big.Int).Quo(args[0].Number.Num(), args[0].Number.Denom())
		for _, arg := range args[1:] {
			n := new(big.Int).Quo(arg.Number.Num(), arg.Number.Denom())
			g.GCD(nil, nil, g, n)
		}
		return Value{Kind: Number, Number: new(big.Rat).SetInt(g)}, true, nil
	case "lcm":
		l := new(big.Int).Quo(args[0].Number.Num(), args[0].Number.Denom())
		for _, arg := range args[1:] {
			n := new(big.Int).Quo(arg.Number.Num(), arg.Number.Denom())
			if l.Sign() == 0 || n.Sign() == 0 {
				return Value{Kind: Number, Number: new(big.Rat)}, true, nil
			}
			g := new(big.Int).GCD(nil, nil, l, n)
			l.Mul(l, n)
			l.Quo(l, g)
			l.Abs(l)
		}
		return Value{Kind: Number, Number: new(big.Rat).SetInt(l)}, true, nil
	case "perm", "comb":
		nVal, kVal := ratInt(args[0]), ratInt(args[1])
		if nVal < 0 || kVal < 0 || kVal > nVal || nVal > 200 {
			return Value{}, true, invalid("invalid permutation")
		}
		r := bigFact(nVal)
		r.Quo(r, bigFact(nVal-kVal))
		if n.op == "comb" {
			r.Quo(r, bigFact(kVal))
		}
		return Value{Kind: Number, Number: new(big.Rat).SetInt(r)}, true, nil
	case "clamp":
		v, lo, hi := args[0], args[1], args[2]
		if v.Number.Cmp(lo.Number) < 0 {
			return lo, true, nil
		}
		if v.Number.Cmp(hi.Number) > 0 {
			return hi, true, nil
		}
		return v, true, nil
	case "list_sum", "list_avg", "list_count", "list_median", "list_stddev", "list_min", "list_max":
		v, err := listStat(n.op, args)
		return v, true, err
	case "nthroot":
		degree, _ := args[0].Number.Float64()
		x, _ := args[1].Number.Float64()
		if degree == 0 {
			return Value{}, true, invalid("invalid root")
		}
		r, err := ratFromFloat(math.Pow(x, 1/degree))
		return Value{Kind: Number, Number: r}, true, err
	case "logbase":
		x, _ := args[0].Number.Float64()
		b, _ := args[1].Number.Float64()
		if x <= 0 || b <= 0 || b == 1 {
			return Value{}, true, invalid("invalid logarithm")
		}
		r, err := ratFromFloat(math.Log(x) / math.Log(b))
		if err != nil {
			return Value{}, true, err
		}
		if f, _ := r.Float64(); math.Abs(f-math.Round(f)) < 1e-10 {
			r = new(big.Rat).SetInt64(int64(math.Round(f)))
		}
		return Value{Kind: Number, Number: r}, true, nil
	case "note":
		return n.value, true, nil
	case "if":
		if args[0].Number.Sign() != 0 {
			return args[1], true, nil
		}
		return args[2], true, nil
	case "==", "!=", ">", "<", ">=", "<=":
		v, err := compareValues(args[0], args[1], n.op, env, c)
		return v, true, err
	case "not":
		if args[0].Number.Sign() == 0 {
			return Value{Kind: Boolean, Number: big.NewRat(1, 1)}, true, nil
		}
		return Value{Kind: Boolean, Number: new(big.Rat)}, true, nil
	case "and":
		if args[0].Number.Sign() != 0 && args[1].Number.Sign() != 0 {
			return Value{Kind: Boolean, Number: big.NewRat(1, 1)}, true, nil
		}
		return Value{Kind: Boolean, Number: new(big.Rat)}, true, nil
	case "or":
		if args[0].Number.Sign() != 0 || args[1].Number.Sign() != 0 {
			return Value{Kind: Boolean, Number: big.NewRat(1, 1)}, true, nil
		}
		return Value{Kind: Boolean, Number: new(big.Rat)}, true, nil
	case "pct_of_set":
		sum := new(big.Rat).Set(args[0].Number)
		for _, a := range args[1:] {
			sum.Add(sum, a.Number)
		}
		if sum.Sign() == 0 {
			return Value{}, true, invalid("division by zero")
		}
		return Value{Kind: Percent, Number: new(big.Rat).Quo(args[0].Number, sum)}, true, nil
	case "pct_after":
		orig := new(big.Rat).Sub(args[0].Number, args[1].Number)
		if orig.Sign() == 0 {
			return Value{}, true, invalid("division by zero")
		}
		return Value{Kind: Percent, Number: new(big.Rat).Quo(args[1].Number, orig)}, true, nil
	case "pct_of":
		if args[1].Number.Sign() == 0 {
			return Value{}, true, invalid("division by zero")
		}
		r := new(big.Rat).Quo(args[0].Number, args[1].Number)
		return Value{Kind: Percent, Number: r}, true, nil
	case "pct_off":
		// left is amount after discount, right is original
		if args[1].Number.Sign() == 0 {
			return Value{}, true, invalid("division by zero")
		}
		r := new(big.Rat).Sub(big.NewRat(1, 1), new(big.Rat).Quo(args[0].Number, args[1].Number))
		return Value{Kind: Percent, Number: r}, true, nil
	case "pct_on":
		if args[1].Number.Sign() == 0 {
			return Value{}, true, invalid("division by zero")
		}
		r := new(big.Rat).Sub(new(big.Rat).Quo(args[0].Number, args[1].Number), big.NewRat(1, 1))
		return Value{Kind: Percent, Number: r}, true, nil
	case "pct_base_of":
		if args[1].Kind != Percent && args[1].Kind != Number {
			return Value{}, true, invalid("expected percentage")
		}
		if args[1].Number.Sign() == 0 {
			return Value{}, true, invalid("division by zero")
		}
		r := new(big.Rat).Quo(args[0].Number, args[1].Number)
		out := args[0]
		out.Number = r
		return out, true, nil
	case "pct_base_off":
		factor := new(big.Rat).Sub(big.NewRat(1, 1), args[1].Number)
		if factor.Sign() == 0 {
			return Value{}, true, invalid("division by zero")
		}
		out := args[0]
		out.Number = new(big.Rat).Quo(args[0].Number, factor)
		return out, true, nil
	case "pct_base_on":
		factor := new(big.Rat).Add(big.NewRat(1, 1), args[1].Number)
		if factor.Sign() == 0 {
			return Value{}, true, invalid("division by zero")
		}
		out := args[0]
		out.Number = new(big.Rat).Quo(args[0].Number, factor)
		return out, true, nil
	case "pct_change":
		if args[0].Number.Sign() == 0 {
			return Value{}, true, invalid("division by zero")
		}
		r := new(big.Rat).Quo(args[1].Number, args[0].Number)
		r.Sub(r, big.NewRat(1, 1))
		return Value{Kind: Percent, Number: r}, true, nil
	case "ratio_change":
		if args[0].Number.Sign() == 0 {
			return Value{}, true, invalid("division by zero")
		}
		r := new(big.Rat).Quo(args[1].Number, args[0].Number)
		return Value{Kind: Number, Number: r}, true, nil
	case "proportion":
		a, b, left, right := args[0], args[1], args[2], args[3]
		if a.Kind == Quantity && b.Kind == Quantity {
			conv, err := c.convert(b, a.Unit, env, nil)
			if err != nil {
				return Value{}, true, err
			}
			b = conv
		}
		if left.Kind == Quantity && right.Kind == Quantity && right.Number.Sign() != 0 {
			conv, err := c.convert(right, left.Unit, env, nil)
			if err != nil {
				return Value{}, true, err
			}
			right = conv
		}
		if left.Number.Sign() == 0 && right.Number.Sign() != 0 {
			// a/b = ?/d  => ? = a*d/b, in d's unit
			if b.Number.Sign() == 0 {
				return Value{}, true, invalid("division by zero")
			}
			out := right
			out.Number = new(big.Rat).Mul(a.Number, right.Number)
			out.Number.Quo(out.Number, b.Number)
			return c.niceQuantity(out), true, nil
		}
		if right.Number.Sign() == 0 && left.Number.Sign() != 0 {
			if a.Number.Sign() == 0 {
				return Value{}, true, invalid("division by zero")
			}
			out := left
			out.Number = new(big.Rat).Mul(b.Number, left.Number)
			out.Number.Quo(out.Number, a.Number)
			return c.niceQuantity(out), true, nil
		}
		return Value{}, true, invalid("expected one unknown")
	case "compound", "compound_interest":
		v, err := compoundInterest(args[0], args[1], args[2], n.text, n.op == "compound_interest")
		return v, true, err
	case "time_to":
		period := Value{}
		if len(args) > 3 {
			period = args[3]
		}
		v, err := timeToTarget(args[0], args[1], args[2], period, n.text)
		return v, true, err
	case "annuity_required":
		v, err := requiredPrincipal(args[0], args[1])
		return v, true, err
	case "ppi_of":
		diag, err := c.convert(args[0], Unit{"in": 1}, env, nil)
		if err != nil {
			return Value{}, true, err
		}
		w, _ := args[1].Number.Float64()
		h, _ := args[2].Number.Float64()
		inch, _ := diag.Number.Float64()
		if inch == 0 {
			return Value{}, true, invalid("invalid diagonal")
		}
		return Value{Kind: Quantity, Number: ratPlaces(math.Hypot(w, h)/inch, 2), Unit: Unit{"ppi": 1}}, true, nil
	case "rate_at":
		v, err := c.rateAt(args[0], args[1], env)
		return v, true, err
	}
	return Value{}, false, nil
}

func ratInt(v Value) int64 {
	if v.Number == nil || !v.Number.IsInt() || !v.Number.Num().IsInt64() {
		return -1
	}
	return v.Number.Num().Int64()
}

func bigFact(n int64) *big.Int {
	r := big.NewInt(1)
	for i := int64(2); i <= n; i++ {
		r.Mul(r, big.NewInt(i))
	}
	return r
}

func ratFromFloat(f float64) (*big.Rat, error) {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return nil, invalid("non-finite result")
	}
	return new(big.Rat).SetFloat64(f), nil
}

func ratPlaces(f float64, places int) *big.Rat {
	scale := 1.0
	for i := 0; i < places; i++ {
		scale *= 10
	}
	n := int64(math.Round(f * scale))
	return new(big.Rat).SetFrac64(n, int64(scale))
}

func listStat(op string, args []Value) (Value, error) {
	if op == "list_count" {
		return Value{Kind: Number, Number: big.NewRat(int64(len(args)), 1)}, nil
	}
	if len(args) == 0 {
		return Value{}, invalid("empty list")
	}
	nums := make([]*big.Rat, len(args))
	for i, a := range args {
		nums[i] = a.Number
	}
	switch op {
	case "list_sum", "list_avg":
		sum := new(big.Rat)
		for _, n := range nums {
			sum.Add(sum, n)
		}
		if op == "list_avg" {
			sum.Quo(sum, big.NewRat(int64(len(nums)), 1))
		}
		out := args[0]
		out.Number = sum
		return out, nil
	case "list_median":
		cp := append([]*big.Rat{}, nums...)
		for i := 0; i < len(cp); i++ {
			for j := i + 1; j < len(cp); j++ {
				if cp[j].Cmp(cp[i]) < 0 {
					cp[i], cp[j] = cp[j], cp[i]
				}
			}
		}
		if len(cp)%2 == 1 {
			return Value{Kind: Number, Number: new(big.Rat).Set(cp[len(cp)/2])}, nil
		}
		m := new(big.Rat).Add(cp[len(cp)/2-1], cp[len(cp)/2])
		m.Quo(m, big.NewRat(2, 1))
		return Value{Kind: Number, Number: m}, nil
	case "list_min":
		best := nums[0]
		for _, n := range nums[1:] {
			if n.Cmp(best) < 0 {
				best = n
			}
		}
		return Value{Kind: Number, Number: new(big.Rat).Set(best)}, nil
	case "list_max":
		best := nums[0]
		for _, n := range nums[1:] {
			if n.Cmp(best) > 0 {
				best = n
			}
		}
		return Value{Kind: Number, Number: new(big.Rat).Set(best)}, nil
	case "list_stddev":
		if len(nums) < 2 {
			return Value{}, invalid("sample standard deviation needs two values")
		}
		mean := new(big.Rat)
		for _, n := range nums {
			mean.Add(mean, n)
		}
		mean.Quo(mean, big.NewRat(int64(len(nums)), 1))
		sum := 0.0
		mf, _ := mean.Float64()
		for _, n := range nums {
			x, _ := n.Float64()
			d := x - mf
			sum += d * d
		}
		r, err := ratFromFloat(math.Sqrt(sum / float64(len(nums)-1)))
		return Value{Kind: Number, Number: r}, err
	}
	return Value{}, invalid("unknown list operation")
}

func compareValues(a, b Value, op string, env Env, c *Catalog) (Value, error) {
	if a.Kind == Quantity && b.Kind == Quantity {
		var err error
		b, err = c.convert(b, a.Unit, env, nil)
		if err != nil {
			return Value{}, err
		}
	}
	cmp := 0
	if isTemporal(a.Kind) && isTemporal(b.Kind) {
		cmp = a.Time.Compare(b.Time)
	} else if a.Number == nil || b.Number == nil {
		return Value{}, invalid("cannot compare these values")
	} else {
		cmp = a.Number.Cmp(b.Number)
	}
	ok := false
	switch op {
	case "==":
		ok = cmp == 0
	case "!=":
		ok = cmp != 0
	case ">":
		ok = cmp > 0
	case "<":
		ok = cmp < 0
	case ">=":
		ok = cmp >= 0
	case "<=":
		ok = cmp <= 0
	}
	n := new(big.Rat)
	if ok {
		n = big.NewRat(1, 1)
	}
	return Value{Kind: Boolean, Number: n}, nil
}

func compoundInterest(pv, years, rate Value, freq string, interestOnly bool) (Value, error) {
	if pv.Number == nil {
		return Value{}, invalid("expected amount")
	}
	payout, compound := freq, freq
	if i := strings.Index(freq, ":"); i >= 0 {
		payout, compound = freq[:i], freq[i+1:]
	}
	p, _ := pv.Number.Float64()
	n := 0.0
	if years.Kind == CalendarSpan {
		n = float64(years.Months) + float64(years.Days)/30.436875
	} else if years.Number != nil {
		n, _ = years.Number.Float64()
	}
	if years.Kind == Quantity && years.Unit["y"] == 1 && years.Number != nil {
		n, _ = years.Number.Float64()
	}
	r, _ := rate.Number.Float64()
	if rate.Kind != Percent {
		r = r / 100
	}
	periods := 1.0
	switch compound {
	case "monthly":
		periods = 12
	case "quarterly":
		periods = 4
	case "daily":
		periods = 365
	case "per_month", "per_day":
		periods = 1
	}
	fv := p * math.Pow(1+r/periods, n*periods)
	if interestOnly {
		fv -= p
	}
	places := 2
	if payout != "" && payout != compound {
		switch payout {
		case "hourly":
			fv /= n * 365.2425 * 24
			places = 4
		case "daily":
			fv /= n * 365.2425
		case "weekly":
			fv /= n * 365.2425 / 7
		case "monthly":
			fv /= n * 12
		case "yearly", "annual":
			fv /= n
		}
	}
	out := pv
	num := ratPlaces(fv, places)
	out.Number = num
	return out, nil
}

func requiredPrincipal(pmt, rate Value) (Value, error) {
	if pmt.Number == nil || rate.Number == nil {
		return Value{}, invalid("expected amount")
	}
	n, _ := pmt.Number.Float64()
	r, _ := rate.Number.Float64()
	if rate.Kind != Percent {
		r = r / 100
	}
	if r == 0 {
		return Value{}, invalid("invalid rate")
	}
	switch {
	case pmt.Unit["mo"] == -1 || pmt.Unit["month"] == -1:
		n *= 12
	case pmt.Unit["w"] == -1:
		n *= 52
	case pmt.Unit["d"] == -1:
		n *= 365.2425
	case pmt.Unit["h"] == -1:
		n *= 365.2425 * 24
	}
	out := pmt
	out.Number = ratPlaces(n/r, 2)
	out.Unit = copyUnit(pmt.Unit)
	for k, v := range out.Unit {
		if v < 0 {
			delete(out.Unit, k)
		}
	}
	return out, nil
}

func (c *Catalog) rateAt(amount, rate Value, env Env) (Value, error) {
	da, dr := c.dimensions(amount.Unit), c.dimensions(rate.Unit)
	if da["money"] == 1 && da["time"] == 0 && dr["money"] == 1 && dr["time"] == 0 && amount.Number != nil && rate.Number != nil {
		out := rate
		out.Number = new(big.Rat).Mul(amount.Number, rate.Number)
		out.Kind = Quantity
		return out, nil
	}
	if da["money"] == 1 && da["time"] == 0 && dr["money"] == 1 && dr["time"] == -1 {
		return c.multiply(amount, rate, true, env)
	}
	return c.multiply(amount, rate, false, env)
}

func timeToTarget(start, end, rate, period Value, kind string) (Value, error) {
	s, _ := start.Number.Float64()
	e, _ := end.Number.Float64()
	r, _ := rate.Number.Float64()
	if rate.Kind == Percent || kind == "growth" {
		if rate.Kind != Percent {
			r = r / 100
		}
		if r <= 0 || s <= 0 || e <= s {
			return Value{}, invalid("invalid growth")
		}
		n := math.Log(e/s) / math.Log(1+r)
		return Value{Kind: Quantity, Number: ratPlaces(n, 2), Unit: Unit{"month": 1}}, nil
	}
	if r <= 0 {
		return Value{}, invalid("invalid contribution")
	}
	n := (e - s) / r
	num, err := ratFromFloat(n)
	if err != nil {
		return Value{}, err
	}
	unit := Unit{"month": 1}
	if period.Kind == Quantity && len(period.Unit) > 0 {
		out := new(big.Rat).Mul(num, period.Number)
		return Value{Kind: Quantity, Number: out, Unit: period.Unit}, nil
	}
	return Value{Kind: Quantity, Number: num, Unit: unit}, nil
}

// niceQuantity picks a same-dimension unit whose magnitude is in [1, 1000).
func (c *Catalog) niceQuantity(v Value) Value {
	if v.Kind != Quantity || len(v.Unit) != 1 || v.Number == nil {
		return v
	}
	dim := ""
	for k := range v.Unit {
		dim = c.Units[k].Dimension
	}
	if dim != "length" {
		return v
	}
	meters, err := c.convert(v, Unit{"m": 1}, Env{}, nil)
	if err != nil {
		return v
	}
	abs, _ := new(big.Rat).Abs(meters.Number).Float64()
	symbol := "m"
	switch {
	case abs == 0:
		return v
	case abs < 0.001:
		symbol = "nm"
	case abs < 0.01:
		symbol = "mm"
	case abs < 1:
		symbol = "cm"
	case abs >= 1000:
		symbol = "km"
	}
	out, err := c.convert(meters, Unit{symbol: 1}, Env{}, nil)
	if err != nil {
		return v
	}
	return out
}
