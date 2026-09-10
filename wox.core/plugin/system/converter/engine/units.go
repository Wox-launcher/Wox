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
	c.registerUnit("y", "time", "31556952", "year", "years", "yr", "yrs")
	c.registerUnit("month", "calendar", "1", "month", "months")
	// Rate months use the mean Gregorian month. Do not alias "month" here:
	// that word remains the civil calendar span used by date arithmetic.
	c.registerUnit("mo", "time", "2629746", "mo", "mo")
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
	c.registerUnit("tbsp", "volume", "14.78676478125", "tablespoon", "tablespoons")
	c.registerUnit("cup", "volume", "236.5882365", "cup", "cups")
	c.registerUnit("floz", "volume", "29.5735295625", "fluid ounce", "fluid ounces", "floz")
	c.registerUnit("pt", "volume", "473.176473", "pint", "pints")
	c.registerUnit("ipt", "volume", "568.26125", "imperial pint", "imperial pints")
	c.registerUnit("frame", "frame", "1", "frame", "frames")
	c.registerUnit("fps", "frequency", "1", "fps", "fps")
	c.registerUnit("px", "pixel", "1", "pixel", "pixels")
	c.registerUnit("ns", "time", "0.000000001", "nanosecond", "nanoseconds")
	c.registerUnit("us", "time", "0.000001", "microsecond", "microseconds", "µs")
	c.registerUnit("nm", "length", "0.000000001", "nanometer", "nanometers")
	c.registerUnit("ly", "length", "9460730472580800", "light year", "light years")
	c.registerUnit("nmi", "length", "1852", "nautical mile", "nautical miles", "nmile")
	c.registerUnit("ha", "area", "10000", "hectare", "hectares")
	c.registerUnit("ac", "area", "4046.8564224", "acre", "acres")
	c.registerUnit("gal", "volume", "3785.411784", "gallon", "gallons")
	c.registerUnit("st", "mass", "6350.29318", "stone", "stones")
	c.registerUnit("Pa", "pressure", "1", "pascal", "pascals")
	c.registerUnit("psi", "pressure", "6894.757293168361", "psi", "psi")
	c.registerUnit("deg", "angle", "1", "degree", "degrees", "°", "º")
	c.registerUnit("arcmin", "angle", "1/60", "arcmin", "arcmins", "′")
	c.registerUnit("arcsec", "angle", "1/3600", "arcsec", "arcsecs", "″")
	c.registerUnit("rad", "angle", "57.29577951308232", "radian", "radians")
	c.registerUnit("Hz", "freq", "1", "hertz", "hertz", "hz")
	c.registerUnit("ppi", "resolution", "1", "ppi", "ppi", "dpi")
	c.registerUnit("rpm", "freq", "1/60", "rpm", "rpm")
	c.registerUnit("mol", "amount", "1", "mole", "moles", "mols")
	c.registerUnit("mmol", "amount", "0.001", "millimole", "millimoles")
	c.registerUnit("N", "force", "1", "N", "N", "newton", "newtons")
	c.registerUnit("pdl", "force", "0.138254954376", "pdl", "pdl", "poundal", "poundals")
	c.registerUnit("W", "power", "1", "W", "W", "watt", "watts")
	c.registerUnit("M", "concentration", "1", "ᴍ", "ᴍ", "molar", "molars")
	c.registerUnit("Nm", "torque", "1", "N⋅m", "N⋅m", "n⋅m", "n-m")
	c.registerUnit("mph", "speed", "0.44704", "mph", "mph")
	c.Aliases["\""] = "in"
	c.Aliases["'"] = "ft"
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
	// Mbps is megabits/second (1e6/8 bytes per second).
	c.registerUnit("Mbps", "storage", "125000", "Mbps", "Mbps", "mbps")
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
	for alias, currency := range map[string]string{
		"$": "USD", "us$": "USD", "dollar": "USD", "dollars": "USD",
		"€": "EUR", "euro": "EUR", "euros": "EUR",
		"£": "GBP", "¥": "CNY",
		"nz$": "NZD", "c$": "CAD", "ca$": "CAD",
		"a$": "AUD", "au$": "AUD", "s$": "SGD",
		"nt$": "TWD", "hk$": "HKD", "r$": "BRL",
	} {
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
		} else if a == 3 {
			text += "³"
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
	if f, ok := specialRateFactor(u); ok {
		return f, nil
	}
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

// specialRateFactor uses Soulver's business-day and calendar-month rate ratios
// instead of raw second scales (5 workdays/week, 52 weeks/12 months).
func specialRateFactor(u Unit) (*big.Rat, bool) {
	timeU := Unit{}
	for k, n := range u {
		if n == 0 {
			continue
		}
		if d, ok := catalogDim(k); ok && d == "money" {
			continue
		}
		timeU[k] = n
	}
	if len(timeU) != 2 {
		return nil, false
	}
	if timeU["w"] == 1 && timeU["workday"] == -1 {
		return big.NewRat(5, 1), true
	}
	if timeU["w"] == -1 && timeU["workday"] == 1 {
		return big.NewRat(1, 5), true
	}
	if timeU["mo"] == 1 && timeU["w"] == -1 {
		return big.NewRat(52, 12), true
	}
	if timeU["mo"] == -1 && timeU["w"] == 1 {
		return big.NewRat(12, 52), true
	}
	return nil, false
}

func nonMoneyUnits(u Unit) Unit {
	out := Unit{}
	for k, n := range u {
		if d, ok := catalogDim(k); ok && d == "money" {
			continue
		}
		out[k] = n
	}
	return out
}

func catalogDim(symbol string) (string, bool) {
	switch symbol {
	case "USD", "EUR", "GBP", "JPY", "CNY", "INR", "HKD", "BTC", "ETH", "USDT", "BNB", "RUB", "AUD", "DKK", "NZD", "CAD", "SGD", "TWD", "BRL":
		return "money", true
	}
	return "", false
}

// convert requires complete dimensional agreement, except a single currency target
// can replace the currency factor of a rate without discarding its denominator.
func (c *Catalog) convert(v Value, target Unit, env Env, ppi *big.Rat) (Value, error) {
	if v.Number == nil {
		return Value{}, invalid("expected quantity")
	}
	if len(target) == 1 && c.dimensions(target)["time"] == -1 && c.dimensions(v.Unit)["money"] == 1 && c.dimensions(v.Unit)["time"] == -1 {
		merged := copyUnit(v.Unit)
		for k := range merged {
			if c.Units[k].Dimension == "time" || k == "mo" || k == "?m" {
				delete(merged, k)
			}
		}
		for k, n := range target {
			merged[k] = n
		}
		target = merged
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
	if v.Unit["w"] == -1 && target["mo"] == -1 && len(nonMoneyUnits(v.Unit)) == 1 && len(nonMoneyUnits(target)) == 1 {
		v.Number = new(big.Rat).Mul(v.Number, big.NewRat(52, 12))
		v.Unit = copyUnit(target)
		v.Kind = Quantity
		return checked(v)
	}
	if v.Unit["mo"] == -1 && target["w"] == -1 && len(nonMoneyUnits(v.Unit)) == 1 && len(nonMoneyUnits(target)) == 1 {
		v.Number = new(big.Rat).Mul(v.Number, big.NewRat(12, 52))
		v.Unit = copyUnit(target)
		v.Kind = Quantity
		return checked(v)
	}
	if env.Substance != "" {
		if converted, ok, err := convertCooking(v, target, env.Substance); ok || err != nil {
			return converted, err
		}
	}
	if len(target) == 1 && target["M"] == 1 {
		out, err := c.convert(v, Unit{"mol": 1, "l": -1}, env, ppi)
		if err != nil {
			return Value{}, err
		}
		out.Unit = Unit{"M": 1}
		return out, nil
	}
	fromDim, toDim := c.dimensions(v.Unit), c.dimensions(target)
	if sameUnit(fromDim, Unit{"speed": 1}) && sameUnit(toDim, Unit{"time": 1}) {
		length := speedLengthSymbol(v.Unit)
		ms, err := c.factor(v.Unit, env)
		if err != nil {
			return Value{}, err
		}
		ts, err := c.factor(target, env)
		if err != nil {
			return Value{}, err
		}
		ls, err := c.factor(Unit{length: 1}, env)
		if err != nil {
			return Value{}, err
		}
		n := new(big.Rat).Mul(v.Number, ms)
		n.Mul(n, ts)
		n.Quo(n, ls)
		out := Unit{length: 1}
		for k, e := range target {
			out[k] -= e
			if out[k] == 0 {
				delete(out, k)
			}
		}
		v.Number = n
		v.Unit = out
		v.Kind = Quantity
		return checked(v)
	}
	// Soulver treats "10 cubic centimeters to meters" as a conversion into cubic meters.
	if fromN, toN := fromDim["length"], toDim["length"]; fromN == 3 && toN == 1 && len(fromDim) == 1 && len(toDim) == 1 {
		raised := Unit{}
		for k, n := range target {
			raised[k] = n * fromN
		}
		target = raised
		toDim = c.dimensions(target)
	}
	if (sameUnit(fromDim, Unit{"freq": 1}) && sameUnit(toDim, Unit{"time": 1})) ||
		(sameUnit(fromDim, Unit{"time": 1}) && sameUnit(toDim, Unit{"freq": 1})) {
		a, err := c.factor(v.Unit, env)
		if err != nil {
			return Value{}, err
		}
		b, err := c.factor(target, env)
		if err != nil {
			return Value{}, err
		}
		hz := new(big.Rat).Mul(v.Number, a)
		if sameUnit(fromDim, Unit{"time": 1}) {
			if hz.Sign() == 0 {
				return Value{}, invalid("division by zero")
			}
			hz.Inv(hz)
		} else {
			if hz.Sign() == 0 {
				return Value{}, invalid("division by zero")
			}
			hz.Inv(hz)
		}
		hz.Quo(hz, b)
		v.Number = hz
		v.Unit = copyUnit(target)
		v.Kind = Quantity
		return checked(v)
	}
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
