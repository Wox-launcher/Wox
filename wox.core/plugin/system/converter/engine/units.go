package engine

import (
	"math/big"
	"sort"
	"strconv"
	"strings"
	"wox/plugin/system/converter/core"
)

// Definition is either a linear unit or an absolute temperature (with offset).
type Definition struct{ Symbol, Dimension, Scale, Offset, Singular, Plural string }
type Catalog struct {
	Units   map[string]Definition
	Aliases map[string]string
	Zones   map[string]string
	Crypto  map[string]bool
}

// NewCatalog owns static definitions; it never consults prices or the system clock.
func NewCatalog() *Catalog {
	c := &Catalog{Units: map[string]Definition{}, Aliases: map[string]string{}, Zones: ZoneAliases(), Crypto: map[string]bool{}}
	c.registerUnit("s", "time", "1", "second", "seconds", "sec", "secs")
	c.registerUnit("ms", "time", "0.001", "millisecond", "milliseconds")
	c.registerUnit("min", "time", "60", "minute", "minutes", "mins", "m")
	c.registerUnit("h", "time", "3600", "hour", "hours", "hr", "hrs")
	c.registerUnit("d", "time", "86400", "day", "days")
	c.registerUnit("w", "time", "604800", "week", "weeks")
	c.registerUnit("y", "time", "31536000", "year", "years")
	c.registerUnit("month", "calendar", "1", "month", "months")
	c.registerUnit("workday", "time", "28800", "workday", "workdays")
	c.registerUnit("m", "length", "1", "meter", "meters", "metre", "metres")
	// Bare m remains a minute unless a local length context resolves it.
	c.Aliases["m"] = "min"
	c.registerUnit("cm", "length", "0.01", "centimeter", "centimeters")
	c.registerUnit("mm", "length", "0.001", "millimeter", "millimeters")
	c.registerUnit("km", "length", "1000", "kilometer", "kilometers", "kilometre", "kilometres")
	c.registerUnit("in", "length", "0.0254", "inch", "inches")
	c.registerUnit("ft", "length", "0.3048", "foot", "feet")
	c.registerUnit("yd", "length", "0.9144", "yard", "yards")
	c.registerUnit("mi", "length", "1609.344", "mile", "miles")
	c.registerUnit("g", "mass", "1", "gram", "grams")
	c.registerUnit("mg", "mass", "0.001", "milligram", "milligrams")
	c.registerUnit("kg", "mass", "1000", "kilogram", "kilograms")
	c.registerUnit("oz", "mass", "28.349523125", "ounce", "ounces")
	c.registerUnit("lb", "mass", "453.59237", "pound", "pounds", "lbs")
	c.registerUnit("t", "mass", "1000000", "ton", "tons")
	c.registerUnit("ml", "volume", "1", "milliliter", "milliliters")
	c.registerUnit("l", "volume", "1000", "liter", "liters", "litre", "litres")
	c.registerUnit("tsp", "volume", "4.92892159375", "teaspoon", "teaspoons")
	c.registerUnit("px", "pixel", "1", "pixel", "pixels")
	glossary := core.NewStorageGlossary()
	scales := map[string]string{"B": "1", "KB": "1000", "MB": "1000000", "GB": "1000000000", "TB": "1000000000000", "KiB": "1024", "MiB": "1048576", "GiB": "1073741824", "TiB": "1099511627776"}
	for _, alias := range glossary.Aliases() {
		unit, _ := glossary.ResolveStorageUnit(alias)
		c.registerUnit(unit.Symbol, "storage", scales[unit.Symbol], unit.Symbol, unit.Symbol, alias)
	}
	for _, t := range []struct{ symbol, scale, offset, word string }{{"°C", "1", "0", "celsius"}, {"°F", "5/9", "-160/9", "fahrenheit"}, {"K", "1", "-273.15", "kelvin"}} {
		c.registerUnit(t.symbol, "temperature", t.scale, t.word, t.word)
		d := c.Units[t.symbol]
		d.Offset = t.offset
		c.Units[t.symbol] = d
		c.Aliases[strings.ToLower(strings.TrimPrefix(t.symbol, "°"))] = t.symbol
		c.registerUnit("delta"+strings.TrimPrefix(t.symbol, "°"), "temperature", t.scale, "temperature difference", "temperature difference")
	}
	c.Aliases["centigrade"] = "°C"
	return c
}

// registerUnit makes every alias refer to one canonical conversion definition.
func (c *Catalog) registerUnit(symbol, dimension, scale, singular, plural string, aliases ...string) {
	c.Units[symbol] = Definition{Symbol: symbol, Dimension: dimension, Scale: scale, Singular: singular, Plural: plural}
	for _, alias := range append(aliases, symbol, singular, plural) {
		c.Aliases[strings.ToLower(alias)] = symbol
	}
}

// AddCurrency registers syntax separately from the availability of a live price.
func (c *Catalog) AddCurrency(code string, crypto bool) {
	code = strings.ToUpper(code)
	c.registerUnit(code, "money", "1", code, code)
	c.Crypto[code] = crypto
	for alias, currency := range map[string]string{"$": "USD", "dollar": "USD", "dollars": "USD", "€": "EUR", "euro": "EUR", "euros": "EUR", "£": "GBP", "¥": "CNY"} {
		if currency == code {
			c.Aliases[alias] = code
		}
	}
}

// resolve preserves m as a candidate until expression context is known.
func (c *Catalog) resolve(s string) (string, bool) {
	if s == "m" {
		return "?m", true
	}
	u, ok := c.Aliases[strings.ToLower(s)]
	return u, ok
}

// unitText orders factors deterministically even when no named compound unit exists.
func unitText(u Unit) string {
	keys := make([]string, 0, len(u))
	for k := range u {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var top, bottom []string
	for _, k := range keys {
		n := u[k]
		if n == 0 {
			continue
		}
		a := n
		if a < 0 {
			a = -a
		}
		text := k
		if a == 2 {
			text += "²"
		} else if a != 1 {
			text += "^" + strconv.Itoa(a)
		}
		if n > 0 {
			top = append(top, text)
		} else {
			bottom = append(bottom, text)
		}
	}
	if len(top) == 0 {
		top = append(top, "1")
	}
	text := strings.Join(top, "·")
	if len(bottom) > 0 {
		text += "/" + strings.Join(bottom, "·")
	}
	return text
}

// dimensions returns a normalized dimension signature, ignoring display factors.
func (c *Catalog) dimensions(u Unit) Unit {
	d := Unit{}
	for k, n := range u {
		d[c.Units[k].Dimension] += n
	}
	for k, n := range d {
		if n == 0 {
			delete(d, k)
		}
	}
	return d
}

// sameUnit compares normalized factors or dimensions without depending on map order.
func sameUnit(a, b Unit) bool {
	if len(a) != len(b) {
		return false
	}
	for k, n := range a {
		if b[k] != n {
			return false
		}
	}
	return true
}

// absolute identifies affine temperatures that must not enter multiplicative algebra.
func (c *Catalog) absolute(u Unit) bool {
	for k := range u {
		if c.Units[k].Offset != "" {
			return true
		}
	}
	return false
}

// factor produces a per-query conversion factor, including the captured money prices.
func (c *Catalog) factor(u Unit, env Env) (*big.Rat, error) {
	f := big.NewRat(1, 1)
	for k, n := range u {
		d := c.Units[k]
		r := rational(d.Scale)
		if d.Dimension == "money" {
			r = env.Prices[k]
			if r == nil || r.Sign() <= 0 {
				return nil, &Error{Kind: Unavailable, Message: "missing price for " + k}
			}
		}
		if r == nil {
			return nil, invalid("unknown unit " + k)
		}
		exponent := n
		if exponent < 0 {
			exponent = -exponent
		}
		p := new(big.Rat).SetFrac(new(big.Int).Exp(r.Num(), big.NewInt(int64(exponent)), nil), new(big.Int).Exp(r.Denom(), big.NewInt(int64(exponent)), nil))
		if n < 0 {
			f.Quo(f, p)
		} else {
			f.Mul(f, p)
		}
	}
	return f, nil
}

// convert requires complete dimensional agreement, except a single currency target
// can replace the currency factor of a rate without discarding its denominator.
func (c *Catalog) convert(v Value, target Unit, env Env, ppi *big.Rat) (Value, error) {
	if v.Number == nil {
		return Value{}, invalid("expected quantity")
	}
	if len(target) == 1 && c.dimensions(target)["money"] == 1 && c.dimensions(v.Unit)["money"] == 1 {
		merged := copyUnit(v.Unit)
		for k := range merged {
			if c.Units[k].Dimension == "money" {
				delete(merged, k)
			}
		}
		for k, n := range target {
			merged[k] = n
		}
		target = merged
	}
	fromDim, toDim := c.dimensions(v.Unit), c.dimensions(target)
	if !sameUnit(fromDim, toDim) {
		if ppi == nil || ppi.Sign() <= 0 || !(sameUnit(fromDim, Unit{"length": 1}) && sameUnit(toDim, Unit{"pixel": 1}) || sameUnit(fromDim, Unit{"pixel": 1}) && sameUnit(toDim, Unit{"length": 1})) {
			return Value{}, invalid("incompatible conversion")
		}
	}
	a, err := c.factor(v.Unit, env)
	if err != nil {
		return Value{}, err
	}
	b, err := c.factor(target, env)
	if err != nil {
		return Value{}, err
	}
	n := new(big.Rat).Mul(v.Number, a)
	if c.absolute(v.Unit) {
		if len(v.Unit) != 1 {
			return Value{}, invalid("compound absolute temperature")
		}
		for k := range v.Unit {
			n.Add(n, rational(c.Units[k].Offset))
		}
	}
	if !sameUnit(fromDim, toDim) {
		pixelsPerMeter := new(big.Rat).Quo(ppi, rational("0.0254"))
		if fromDim["length"] == 1 {
			n.Mul(n, pixelsPerMeter)
		} else {
			n.Quo(n, pixelsPerMeter)
		}
	}
	if c.absolute(target) {
		if len(target) != 1 {
			return Value{}, invalid("compound absolute temperature")
		}
		for k := range target {
			n.Sub(n, rational(c.Units[k].Offset))
		}
	}
	n.Quo(n, b)
	v.Number = n
	v.Unit = copyUnit(target)
	v.Kind = Quantity
	return checked(v)
}
